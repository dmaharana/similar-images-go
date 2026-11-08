package cluster

import (
	"context"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"similar-images/pkg/cache"
	"similar-images/pkg/engine"
	"similar-images/pkg/search"
)

// Candidate represents an image evaluated in clustering.
type Candidate struct {
	Path        string `json:"path"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FileSize    int64  `json:"file_size"`
	Mtime       int64  `json:"mtime"`
	Format      string `json:"format"`
	Fingerprint engine.Fingerprint
}

// Cluster represents a group of mutually similar candidate images.
type Cluster struct {
	ID             int         `json:"id"`
	CanonicalIndex int         `json:"canonical_index"`
	Canonical      Candidate   `json:"canonical"`
	Duplicates     []Candidate `json:"duplicates"`
	AllCandidates  []Candidate `json:"all_candidates"`
}

// ClusterReport summarizes the results of duplicate clustering.
type ClusterReport struct {
	TargetDir      string    `json:"target_dir"`
	Metric         string    `json:"metric"`
	Threshold      float32   `json:"threshold"`
	Heuristic      string    `json:"heuristic"`
	TotalScanned   int       `json:"total_scanned"`
	ClusterCount   int       `json:"cluster_count"`
	DuplicateCount int       `json:"duplicate_count"`
	DurationMs     int64     `json:"duration_ms"`
	Clusters       []Cluster `json:"clusters"`
}

// FindDuplicateClusters scans targetDir and partitions candidate images into duplicate clusters using Disjoint-Set.
func FindDuplicateClusters(
	ctx context.Context,
	targetDir string,
	metric engine.Metric,
	threshold float32,
	heuristic Heuristic,
	recursive bool,
	c *cache.Cache,
	progressFn func(scanned, total int, phase string),
) (*ClusterReport, error) {
	startTime := time.Now()

	eng, err := engine.GetEngine(metric)
	if err != nil {
		return nil, err
	}

	report := &ClusterReport{
		TargetDir: targetDir,
		Metric:    string(metric.Canonical()),
		Threshold: threshold,
		Heuristic: string(heuristic),
	}

	candidatePaths, err := search.DiscoverCandidatePaths(targetDir, recursive)
	if err != nil {
		return nil, err
	}
	if len(candidatePaths) < 2 {
		report.TotalScanned = len(candidatePaths)
		report.DurationMs = time.Since(startTime).Milliseconds()
		return report, nil
	}

	total := len(candidatePaths)
	report.TotalScanned = total

	// Step 1: Pre-index and extract fingerprints
	concurrency := runtime.GOMAXPROCS(0)
	candidates := make([]Candidate, total)
	validCandidates := make([]Candidate, 0, total)

	// Fetch cached records
	type meta struct {
		path string
		size int64
		time int64
	}
	metaList := make([]meta, total)
	paths := make([]string, total)
	sizes := make([]int64, total)
	mtimes := make([]int64, total)

	for i, p := range candidatePaths {
		paths[i] = p
		metaList[i].path = p
		if fi, err := os.Stat(p); err == nil {
			metaList[i].size = fi.Size()
			metaList[i].time = fi.ModTime().Unix()
			sizes[i] = fi.Size()
			mtimes[i] = fi.ModTime().Unix()
		}
	}

	cachedRecords := make(map[string]*cache.CachedRecord)
	if c != nil {
		if batch, err := c.GetBatch(paths, sizes, mtimes); err == nil {
			cachedRecords = batch
		}
	}

	var (
		scannedCount atomic.Int64
		newCacheMu   sync.Mutex
		newEntries   []*cache.CacheEntry
	)

	taskCh := make(chan int, total)
	for i := 0; i < total; i++ {
		taskCh <- i
	}
	close(taskCh)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for idx := range taskCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				m := metaList[idx]
				var (
					candFP   engine.Fingerprint
					candW    int
					candH    int
					candSize int64 = m.size
					candTime int64 = m.time
					candFmt  string
					hit      bool
				)

				if rec, ok := cachedRecords[m.path]; ok {
					var rawFP []byte
					switch metric.Canonical() {
					case engine.MetricPHash:
						rawFP = rec.PHashHash
					case engine.MetricPDQ:
						rawFP = rec.PDQHash
					case engine.MetricWavelet:
						rawFP = rec.WaveletHash
					case engine.MetricDHash:
						rawFP = rec.DHashHash
					case engine.MetricPalette:
						rawFP = rec.PaletteHistogram
					}
					if len(rawFP) > 0 {
						if decoded, err := eng.DecodeFingerprint(rawFP); err == nil {
							candFP = decoded
							candW = rec.Width
							candH = rec.Height
							candFmt = rec.Format
							hit = true
						}
					}
				}

				if !hit {
					img, meta, err := search.OpenImage(m.path)
					if err == nil {
						candFP = eng.Compute(img)
						candW = meta.Width
						candH = meta.Height
						candSize = meta.FileSize
						candTime = meta.Mtime
						candFmt = meta.Format

						if c != nil {
							entry := &cache.CacheEntry{
								CanonicalPath: m.path,
								FileSize:      meta.FileSize,
								Mtime:         meta.Mtime,
								Width:         candW,
								Height:        candH,
								Format:        candFmt,
							}
							switch metric.Canonical() {
							case engine.MetricPHash:
								entry.PHashHash = candFP.Bytes()
							case engine.MetricPDQ:
								entry.PDQHash = candFP.Bytes()
							case engine.MetricWavelet:
								entry.WaveletHash = candFP.Bytes()
							case engine.MetricDHash:
								entry.DHashHash = candFP.Bytes()
							case engine.MetricPalette:
								entry.PaletteHistogram = candFP.Bytes()
							}
							newCacheMu.Lock()
							newEntries = append(newEntries, entry)
							newCacheMu.Unlock()
						}
					}
				}

				if candFP != nil {
					candidates[idx] = Candidate{
						Path:        m.path,
						Width:       candW,
						Height:      candH,
						FileSize:    candSize,
						Mtime:       candTime,
						Format:      candFmt,
						Fingerprint: candFP,
					}
				}

				scanned := int(scannedCount.Add(1))
				if progressFn != nil {
					progressFn(scanned, total, "fingerprinting")
				}
			}
		}()
	}

	wg.Wait()

	// Flush new cache entries
	if c != nil && len(newEntries) > 0 {
		_ = c.PutBatch(newEntries)
	}

	// Filter out failed candidates
	for _, cand := range candidates {
		if cand.Fingerprint != nil {
			validCandidates = append(validCandidates, cand)
		}
	}

	N := len(validCandidates)
	if N < 2 {
		report.DurationMs = time.Since(startTime).Milliseconds()
		return report, nil
	}

	// Step 2: Pairwise comparison with DisjointSet
	ds := NewDisjointSet(N)
	pairTotal := (N * (N - 1)) / 2
	var comparedPairs atomic.Int64

	// Parallel pairwise evaluation
	type pair struct {
		i int
		j int
	}
	pairCh := make(chan pair, 1000)

	go func() {
		for i := 0; i < N; i++ {
			for j := i + 1; j < N; j++ {
				pairCh <- pair{i: i, j: j}
			}
		}
		close(pairCh)
	}()

	var pairWg sync.WaitGroup
	var dsMu sync.Mutex

	for w := 0; w < concurrency; w++ {
		pairWg.Add(1)
		go func() {
			defer pairWg.Done()
			for p := range pairCh {
				score := eng.Similarity(validCandidates[p.i].Fingerprint, validCandidates[p.j].Fingerprint)
				if score.MeetsThreshold(threshold) {
					dsMu.Lock()
					ds.Union(p.i, p.j)
					dsMu.Unlock()
				}
				done := int(comparedPairs.Add(1))
				if progressFn != nil && done%100 == 0 {
					progressFn(done, pairTotal, "clustering")
				}
			}
		}()
	}

	pairWg.Wait()

	// Step 3: Group candidates by root
	groups := make(map[int][]Candidate)
	for i := 0; i < N; i++ {
		root := ds.Find(i)
		groups[root] = append(groups[root], validCandidates[i])
	}

	// Step 4: Build cluster report (only groups with >= 2 candidates)
	clusterID := 1
	var totalDuplicates int
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}

		canonicalIdx := PickCanonical(group, heuristic)
		canonical := group[canonicalIdx]

		var duplicates []Candidate
		for idx, cand := range group {
			if idx != canonicalIdx {
				duplicates = append(duplicates, cand)
			}
		}

		totalDuplicates += len(duplicates)
		report.Clusters = append(report.Clusters, Cluster{
			ID:             clusterID,
			CanonicalIndex: canonicalIdx,
			Canonical:      canonical,
			Duplicates:     duplicates,
			AllCandidates:  group,
		})
		clusterID++
	}

	report.ClusterCount = len(report.Clusters)
	report.DuplicateCount = totalDuplicates
	report.DurationMs = time.Since(startTime).Milliseconds()

	return report, nil
}
