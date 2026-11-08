package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"similar-images/pkg/cache"
	"similar-images/pkg/cluster"
	"similar-images/pkg/engine"
	"similar-images/pkg/search"
)

const Version = "0.2.0"

// Run executes the CLI application with the given arguments.
func Run(args []string, stdout, stderr io.Writer, launchDesktopFn func() error) error {
	if len(args) == 0 {
		// No args: if in a GUI desktop session (and not headless), launch desktop
		if isDesktopEnvironment() && launchDesktopFn != nil {
			return launchDesktopFn()
		}
		printUsage(stdout)
		return nil
	}

	cmd := args[0]

	// Check for help/version flags first
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		printUsage(stdout)
		return nil
	}
	if cmd == "-v" || cmd == "--version" || cmd == "version" {
		fmt.Fprintf(stdout, "similar-images version %s (Go pure-binary)\n", Version)
		return nil
	}

	// Check for desktop mode
	if cmd == "desktop" || cmd == "--desktop" || cmd == "-gui" || cmd == "--gui" {
		if launchDesktopFn != nil {
			return launchDesktopFn()
		}
		return fmt.Errorf("desktop mode is not available in this build")
	}

	// Subcommands
	switch cmd {
	case "search":
		return runSearch(args[1:], stdout, stderr)
	case "cluster":
		return runCluster(args[1:], stdout, stderr)
	case "cache":
		return runCache(args[1:], stdout, stderr)
	}

	// If the first argument is not a known subcommand but looks like an image path or flag,
	// treat as shorthand `similar-images [options] <QUERY> <SEARCH_DIR>`
	return runSearch(args, stdout, stderr)
}

func isDesktopEnvironment() bool {
	// In Linux, check DISPLAY or WAYLAND_DISPLAY
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `similar-images - Fast visual similarity search and duplicate clustering

Usage:
  similar-images [search] [OPTIONS] <QUERY_IMAGE> <SEARCH_DIR>
  similar-images cluster [OPTIONS] <SEARCH_DIR>
  similar-images cache <stats|vacuum|preindex> [OPTIONS]
  similar-images desktop [OPTIONS]

Commands:
  search     Search candidate image directory for matches to a query image (default)
  cluster    Find duplicate image clusters in a directory and evaluate canonical keeps
  cache      Inspect, vacuum, or pre-index the persistent SQLite fingerprint cache
  desktop    Launch the pure-Go GPU native desktop interface

Global & Search Options:
  -m, --metric <METRIC>        Similarity algorithm: phash (default), pdq, wavelet, dhash, palette
  -t, --threshold <NUM>        Cutoff similarity threshold percentage (0.0 - 100.0, default: 80.0)
  -k, --top <NUM>              Maximum matches to return (default: 10, 0 = unlimited)
      --no-recursive           Do not traverse subdirectories recursively
      --no-cache               Bypass SQLite fingerprint caching
      --cache-file <PATH>      Custom SQLite cache database path
  -j, --concurrency <NUM>      Worker concurrency (default: CPU cores)
      --json                   Output structured JSON results
      --desktop                Launch desktop application

Cluster Options:
      --heuristic <RULE>       Canonical keep rule: resolution (default), size, oldest, newest
      --trash-duplicates       Move duplicate candidate images to OS trash
      --dry-run                Simulate duplicate trash operations without modifying files (default)

Examples:
  similar-images ./ref.jpg ~/Pictures
  similar-images --metric pdq --threshold 85 ./ref.jpg ~/Pictures
  similar-images cluster --heuristic resolution ~/Pictures/Dumps
  similar-images cluster --trash-duplicates ~/Pictures/Dumps
  similar-images desktop
`)
}

func runSearch(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		metricStr   string
		threshold   float64
		topK        int
		noRecursive bool
		noCache     bool
		cacheFile   string
		concurrency int
		jsonOutput  bool
		desktop     bool
	)

	fs.StringVar(&metricStr, "metric", "phash", "Similarity metric: phash, pdq, wavelet, dhash, palette")
	fs.StringVar(&metricStr, "m", "phash", "Shorthand for --metric")
	fs.Float64Var(&threshold, "threshold", 80.0, "Cutoff threshold percentage (0.0 - 100.0)")
	fs.Float64Var(&threshold, "t", 80.0, "Shorthand for --threshold")
	fs.IntVar(&topK, "top", 10, "Maximum matches to return (0 = unlimited)")
	fs.IntVar(&topK, "k", 10, "Shorthand for --top")
	fs.BoolVar(&noRecursive, "no-recursive", false, "Disable recursive directory scan")
	fs.BoolVar(&noCache, "no-cache", false, "Bypass SQLite cache")
	fs.StringVar(&cacheFile, "cache-file", "", "Custom SQLite cache database path")
	fs.IntVar(&concurrency, "concurrency", 0, "Worker concurrency")
	fs.IntVar(&concurrency, "j", 0, "Shorthand for --concurrency")
	fs.BoolVar(&jsonOutput, "json", false, "Output results in JSON format")
	fs.BoolVar(&desktop, "desktop", false, "Launch desktop interface")

	if err := fs.Parse(args); err != nil {
		return err
	}

	positional := fs.Args()
	if len(positional) < 2 {
		printUsage(stdout)
		return fmt.Errorf("search requires <QUERY_IMAGE> and <SEARCH_DIR>")
	}

	queryPath := positional[0]
	searchDir := positional[1]

	m, err := engine.ParseMetric(metricStr)
	if err != nil {
		return err
	}

	queryImg, queryMeta, err := search.OpenImage(queryPath)
	if err != nil {
		return fmt.Errorf("failed to open query image: %w", err)
	}

	var c *cache.Cache
	if !noCache {
		dbPath := cacheFile
		if dbPath == "" {
			dbPath = cache.DefaultCachePath()
		}
		openedCache, err := cache.Open(dbPath)
		if err != nil {
			fmt.Fprintf(stderr, "Warning: failed to open cache (%v), proceeding uncached\n", err)
		} else {
			c = openedCache
			defer c.Close()
		}
	}

	opts := search.SearchOptions{
		SearchDir:   searchDir,
		Metric:      m,
		Threshold:   float32(threshold),
		TopK:        topK,
		Recursive:   !noRecursive,
		UseCache:    !noCache,
		Concurrency: concurrency,
	}

	var lastPrintedTime time.Time
	progressFn := func(scanned, total int) {
		if jsonOutput {
			return
		}
		if time.Since(lastPrintedTime) > 100*time.Millisecond || scanned == total {
			lastPrintedTime = time.Now()
			percent := float64(scanned) / float64(total) * 100.0
			fmt.Fprintf(stderr, "\rScanning images: [%-30s] %d/%d (%.1f%%)",
				progressBar(percent, 30), scanned, total, percent)
			if scanned == total {
				fmt.Fprintln(stderr)
			}
		}
	}

	report, err := search.SearchDirectory(context.Background(), queryImg, queryPath, opts, c, progressFn)
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	// Terminal Formatted Report
	fmt.Fprintf(stdout, "\n=== Visual Similarity Search Report ===\n")
	fmt.Fprintf(stdout, "Query Image : %s\n", queryPath)
	fmt.Fprintf(stdout, "Dimensions  : %dx%d (%s, %s)\n",
		queryMeta.Width, queryMeta.Height, queryMeta.Format, formatBytes(queryMeta.FileSize))
	fmt.Fprintf(stdout, "Target Dir  : %s\n", searchDir)
	fmt.Fprintf(stdout, "Metric      : %s\n", report.Metric)
	fmt.Fprintf(stdout, "Threshold   : %.1f%%\n", report.Threshold)
	fmt.Fprintf(stdout, "Scanned     : %d image(s) in %s\n", report.TotalScanned, formatDuration(report.DurationMs))
	fmt.Fprintf(stdout, "Matches     : %d found\n\n", len(report.Matches))

	if len(report.Matches) == 0 {
		fmt.Fprintln(stdout, "No matching images found meeting the cutoff threshold.")
		return nil
	}

	fmt.Fprintf(stdout, "%-4s | %-10s | %-12s | %-10s | %s\n", "Rank", "Similarity", "Dimensions", "Size", "Candidate Path")
	fmt.Fprintln(stdout, strings.Repeat("-", 75))
	for i, match := range report.Matches {
		dimStr := fmt.Sprintf("%dx%d", match.Width, match.Height)
		fmt.Fprintf(stdout, "#%-3d | %-10s | %-12s | %-10s | %s\n",
			i+1,
			match.SimilarityScore.String(),
			dimStr,
			formatBytes(match.FileSize),
			match.Path,
		)
	}
	fmt.Fprintln(stdout)
	return nil
}

func runCluster(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("cluster", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		metricStr       string
		threshold       float64
		heuristicStr    string
		trashDuplicates bool
		dryRun          bool
		noRecursive     bool
		noCache         bool
		cacheFile       string
		jsonOutput      bool
	)

	fs.StringVar(&metricStr, "metric", "phash", "Similarity metric: phash, pdq, wavelet, dhash, palette")
	fs.StringVar(&metricStr, "m", "phash", "Shorthand for --metric")
	fs.Float64Var(&threshold, "threshold", 80.0, "Cutoff threshold percentage")
	fs.Float64Var(&threshold, "t", 80.0, "Shorthand for --threshold")
	fs.StringVar(&heuristicStr, "heuristic", "resolution", "Canonical keep rule: resolution, size, oldest, newest")
	fs.BoolVar(&trashDuplicates, "trash-duplicates", false, "Move duplicates to OS recycle bin / trash")
	fs.BoolVar(&dryRun, "dry-run", true, "Dry run without moving files (default: true)")
	fs.BoolVar(&noRecursive, "no-recursive", false, "Disable recursive directory traversal")
	fs.BoolVar(&noCache, "no-cache", false, "Bypass SQLite cache")
	fs.StringVar(&cacheFile, "cache-file", "", "Custom SQLite cache database path")
	fs.BoolVar(&jsonOutput, "json", false, "Output results in JSON format")

	if err := fs.Parse(args); err != nil {
		return err
	}

	positional := fs.Args()
	if len(positional) < 1 {
		return fmt.Errorf("cluster requires <SEARCH_DIR>")
	}
	targetDir := positional[0]

	m, err := engine.ParseMetric(metricStr)
	if err != nil {
		return err
	}

	h, err := cluster.ParseHeuristic(heuristicStr)
	if err != nil {
		return err
	}

	var c *cache.Cache
	if !noCache {
		dbPath := cacheFile
		if dbPath == "" {
			dbPath = cache.DefaultCachePath()
		}
		if opened, err := cache.Open(dbPath); err == nil {
			c = opened
			defer c.Close()
		}
	}

	progressFn := func(scanned, total int, phase string) {
		if jsonOutput {
			return
		}
		percent := float64(scanned) / float64(total) * 100.0
		fmt.Fprintf(stderr, "\r%s: [%-30s] %d/%d (%.1f%%)",
			strings.Title(phase), progressBar(percent, 30), scanned, total, percent)
		if scanned == total {
			fmt.Fprintln(stderr)
		}
	}

	report, err := cluster.FindDuplicateClusters(
		context.Background(),
		targetDir,
		m,
		float32(threshold),
		h,
		!noRecursive,
		c,
		progressFn,
	)
	if err != nil {
		return err
	}

	// Handle trash actions if requested
	if trashDuplicates {
		for _, cl := range report.Clusters {
			for _, dup := range cl.Duplicates {
				if err := cluster.MoveToTrash(dup.Path, c); err != nil {
					fmt.Fprintf(stderr, "Warning: failed to trash %s: %v\n", dup.Path, err)
				}
			}
		}
	}

	if jsonOutput {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	// Terminal Formatted Report
	fmt.Fprintf(stdout, "\n=== Duplicate Cluster Report ===\n")
	fmt.Fprintf(stdout, "Target Directory : %s\n", targetDir)
	fmt.Fprintf(stdout, "Metric           : %s\n", report.Metric)
	fmt.Fprintf(stdout, "Threshold        : %.1f%%\n", report.Threshold)
	fmt.Fprintf(stdout, "Keep Heuristic   : %s\n", report.Heuristic)
	fmt.Fprintf(stdout, "Scanned          : %d image(s) in %s\n", report.TotalScanned, formatDuration(report.DurationMs))
	fmt.Fprintf(stdout, "Clusters Found   : %d (%d duplicate copies)\n\n", report.ClusterCount, report.DuplicateCount)

	if len(report.Clusters) == 0 {
		fmt.Fprintln(stdout, "No duplicate clusters found meeting the cutoff threshold.")
		return nil
	}

	for _, cl := range report.Clusters {
		fmt.Fprintf(stdout, "Cluster #%d (%d images):\n", cl.ID, len(cl.AllCandidates))
		fmt.Fprintf(stdout, "  [KEEP]   %s (%dx%d, %s)\n",
			cl.Canonical.Path, cl.Canonical.Width, cl.Canonical.Height, formatBytes(cl.Canonical.FileSize))
		for _, dup := range cl.Duplicates {
			action := "[TRASH]"
			if trashDuplicates {
				action = "[TRASHED]"
			}
			fmt.Fprintf(stdout, "  %-8s %s (%dx%d, %s)\n",
				action, dup.Path, dup.Width, dup.Height, formatBytes(dup.FileSize))
		}
		fmt.Fprintln(stdout)
	}

	if !trashDuplicates && report.DuplicateCount > 0 {
		fmt.Fprintln(stdout, "Dry-run mode: no files were modified. Re-run with --trash-duplicates to move duplicates to trash.")
	}

	return nil
}

func runCache(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("cache subcommand requires an action: stats, vacuum, or preindex")
	}

	action := args[0]
	dbPath := cache.DefaultCachePath()

	c, err := cache.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open cache: %w", err)
	}
	defer c.Close()

	switch action {
	case "stats":
		stats, err := c.Stats()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "=== Fingerprint Cache Statistics ===\n")
		fmt.Fprintf(stdout, "Database Path : %s\n", stats.DBPath)
		fmt.Fprintf(stdout, "Total Entries : %d\n", stats.TotalEntries)
		fmt.Fprintf(stdout, "Disk Size     : %s\n", formatBytes(stats.DBSizeBytes))
		return nil

	case "vacuum":
		fmt.Fprintln(stdout, "Vacuuming cache database...")
		if err := c.Vacuum(); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Cache vacuum complete.")
		return nil

	case "preindex":
		if len(args) < 2 {
			return fmt.Errorf("preindex requires a target directory: similar-images cache preindex <DIR>")
		}
		targetDir := args[1]
		candidates, err := search.DiscoverCandidatePaths(targetDir, true)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Pre-indexing %d candidate images from %s...\n", len(candidates), targetDir)
		m := engine.MetricPHash
		eng, _ := engine.GetEngine(m)

		var count int
		for i, p := range candidates {
			img, meta, err := search.OpenImage(p)
			if err == nil {
				fp := eng.Compute(img)
				_ = c.PutBatch([]*cache.CacheEntry{
					{
						CanonicalPath: p,
						FileSize:      meta.FileSize,
						Mtime:         meta.Mtime,
						Width:         meta.Width,
						Height:        meta.Height,
						Format:        meta.Format,
						PHashHash:     fp.Bytes(),
					},
				})
				count++
			}
			if (i+1)%50 == 0 || i+1 == len(candidates) {
				fmt.Fprintf(stdout, "\rIndexed %d/%d images...", i+1, len(candidates))
			}
		}
		fmt.Fprintf(stdout, "\nPre-indexing complete: %d fingerprints stored in cache.\n", count)
		return nil

	default:
		return fmt.Errorf("unknown cache action: %q (expected: stats, vacuum, preindex)", action)
	}
}

func progressBar(percent float64, width int) string {
	filled := int(percent / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("=", filled) + strings.Repeat("-", width-filled)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.2fs", float64(ms)/1000.0)
}
