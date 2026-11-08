package search

import (
	"context"
	"image"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"similar-images/pkg/cache"
	"similar-images/pkg/engine"
)

// SearchOptions configures 1-to-N similarity search.
type SearchOptions struct {
	SearchDir   string
	Metric      engine.Metric
	Threshold   float32
	TopK        int
	Recursive   bool
	UseCache    bool
	Concurrency int
}

// MatchResult represents a single candidate image matching the query image.
type MatchResult struct {
	Path            string                 `json:"path"`
	SimilarityScore engine.SimilarityScore `json:"similarity_score"`
	ScorePercent    float32                `json:"score_percent"`
	Width           int                    `json:"width"`
	Height          int                    `json:"height"`
	FileSize        int64                  `json:"file_size"`
	Format          string                 `json:"format"`
}

// SearchReport contains full search execution details and ranked results.
type SearchReport struct {
	QueryPath     string         `json:"query_path"`
	QueryWidth    int            `json:"query_width"`
	QueryHeight   int            `json:"query_height"`
	QueryFileSize int64          `json:"query_file_size"`
	Metric        string         `json:"metric"`
	Threshold     float32        `json:"threshold"`
	TotalScanned  int            `json:"total_scanned"`
	DurationMs    int64          `json:"duration_ms"`
	Matches       []MatchResult  `json:"matches"`
}

// SearchDirectory executes visual similarity search against candidate images in a directory.
func SearchDirectory(
	ctx context.Context,
	queryImg image.Image,
	queryPath string,
	opts SearchOptions,
	c *cache.Cache,
	progressFn func(scanned, total int),
) (*SearchReport, error) {
	startTime := time.Now()

	eng, err := engine.GetEngine(opts.Metric)
	if err != nil {
		return nil, err
	}

	queryFP := eng.Compute(queryImg)
	qBounds := queryImg.Bounds()
	var qSize int64
	if fi, err := os.Stat(queryPath); err == nil {
		qSize = fi.Size()
	}

	report := &SearchReport{
		QueryPath:     queryPath,
		QueryWidth:    qBounds.Dx(),
		QueryHeight:   qBounds.Dy(),
		QueryFileSize: qSize,
		Metric:        string(opts.Metric.Canonical()),
		Threshold:     opts.Threshold,
	}

	candidates, err := DiscoverCandidatePaths(opts.SearchDir, opts.Recursive)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		report.DurationMs = time.Since(startTime).Milliseconds()
		return report, nil
	}

	total := len(candidates)
	report.TotalScanned = total

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = runtime.GOMAXPROCS(0)
	}

	// Cache lookup pre-pass
	type candidateMeta struct {
		path     string
		fileSize int64
		mtime    int64
	}

	metaList := make([]candidateMeta, total)
	for i, path := range candidates {
		metaList[i].path = path
		if fi, err := os.Stat(path); err == nil {
			metaList[i].fileSize = fi.Size()
			metaList[i].mtime = fi.ModTime().Unix()
		}
	}

	cachedRecords := make(map[string]*cache.CachedRecord)
	if opts.UseCache && c != nil {
		paths := make([]string, total)
		sizes := make([]int64, total)
		mtimes := make([]int64, total)
		for i, m := range metaList {
			paths[i] = m.path
			sizes[i] = m.fileSize
			mtimes[i] = m.mtime
		}
		if batch, err := c.GetBatch(paths, sizes, mtimes); err == nil {
			cachedRecords = batch
		}
	}

	var (
		matchesMu   sync.Mutex
		matches     []MatchResult
		newCacheMu  sync.Mutex
		newEntries  []*cache.CacheEntry
		scannedCount atomic.Int64
	)

	// Work distribution channel
	taskCh := make(chan candidateMeta, total)
	for _, m := range metaList {
		taskCh <- m
	}
	close(taskCh)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for m := range taskCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				var (
					candFP   engine.Fingerprint
					candW    int
					candH    int
					candSize int64 = m.fileSize
					candFmt  string
					cacheHit bool
				)

				// Check if fingerprint is in cache
				if rec, ok := cachedRecords[m.path]; ok {
					var rawFP []byte
					switch opts.Metric.Canonical() {
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
							cacheHit = true
						}
					}
				}

				if !cacheHit {
					img, meta, err := OpenImage(m.path)
					if err != nil {
						// Skip unreadable files gracefully
						scanned := int(scannedCount.Add(1))
						if progressFn != nil {
							progressFn(scanned, total)
						}
						continue
					}

					candFP = eng.Compute(img)
					candW = meta.Width
					candH = meta.Height
					candSize = meta.FileSize
					candFmt = meta.Format

					if opts.UseCache && c != nil {
						entry := &cache.CacheEntry{
							CanonicalPath: m.path,
							FileSize:      meta.FileSize,
							Mtime:         meta.Mtime,
							Width:         candW,
							Height:        candH,
							Format:        candFmt,
						}
						switch opts.Metric.Canonical() {
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

				// Calculate similarity score
				score := eng.Similarity(queryFP, candFP)
				if score.MeetsThreshold(opts.Threshold) {
					res := MatchResult{
						Path:            m.path,
						SimilarityScore: score,
						ScorePercent:    score.AsPercent(),
						Width:           candW,
						Height:          candH,
						FileSize:        candSize,
						Format:          candFmt,
					}
					matchesMu.Lock()
					matches = append(matches, res)
					matchesMu.Unlock()
				}

				scanned := int(scannedCount.Add(1))
				if progressFn != nil {
					progressFn(scanned, total)
				}
			}
		}()
	}

	wg.Wait()

	// Persist newly computed fingerprints to SQLite cache
	if opts.UseCache && c != nil && len(newEntries) > 0 {
		_ = c.PutBatch(newEntries)
	}

	// Sort matches descending by similarity score
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ScorePercent > matches[j].ScorePercent
	})

	if opts.TopK > 0 && len(matches) > opts.TopK {
		matches = matches[:opts.TopK]
	}

	report.Matches = matches
	report.DurationMs = time.Since(startTime).Milliseconds()

	return report, nil
}
