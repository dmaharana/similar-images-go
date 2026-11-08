package desktop

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"similar-images/pkg/cache"
	"similar-images/pkg/cluster"
	"similar-images/pkg/engine"
	"similar-images/pkg/search"
)

// DesktopApp manages the state and views of the Mygo GPU desktop application.
type DesktopApp struct {
	win   *mygo.Window
	cache *cache.Cache

	// Theme mode: "dark" or "light"
	themeChoice string

	// Tabs
	activeTab int // 0: Search, 1: Comparison, 2: Clusters, 3: Cache

	// Search tab state
	queryPath       string
	searchDir       string
	searchMetric    string
	searchThreshold float64
	searchTopK      string
	searchRecursive bool
	searchRunning   bool
	searchStatus    string
	searchReport    *search.SearchReport
	queryImgThumb   ui.ImageSource

	// Comparison tab state
	compareQueryThumb  ui.ImageSource
	compareMatchThumb  ui.ImageSource
	compareMatchPath   string
	compareScore       string
	compareQueryMeta   string
	compareMatchMeta   string

	// Cluster tab state
	clusterDir       string
	clusterMetric    string
	clusterThreshold float64
	clusterHeuristic string
	clusterRecursive bool
	clusterRunning   bool
	clusterStatus    string
	clusterReport    *cluster.ClusterReport

	// Cache tab state
	cacheStatsText string
	cacheDirInput  string
	cacheRunning   bool
	cacheStatus    string

	mu sync.Mutex
}

// NewDesktopApp initializes the application state.
func NewDesktopApp() *DesktopApp {
	c, _ := cache.Open(cache.DefaultCachePath())
	return &DesktopApp{
		cache:            c,
		themeChoice:      "dark",
		activeTab:        0,
		searchMetric:     "phash",
		searchThreshold:  80.0,
		searchTopK:       "10",
		searchRecursive:  true,
		clusterMetric:    "phash",
		clusterThreshold: 80.0,
		clusterHeuristic: "resolution",
		clusterRecursive: true,
	}
}

// Launch starts the desktop application window.
func Launch() error {
	appState := NewDesktopApp()

	mygo.App.SetName("similar-images")
	mygo.App.WhenReady(func() {
		appState.win = mygo.NewWindow(mygo.WindowOptions{
			Title:           "similar-images - Visual Similarity & Duplicate Finder",
			Width:           1100,
			Height:          760,
			MinWidth:        800,
			MinHeight:       600,
			BackgroundColor: "light-dark(#ffffff, #18181b)",
			Content:         ui.View(appState.view),
		})
		appState.refreshCacheStats()
	})

	return mygo.App.Run()
}

func (a *DesktopApp) view(c *ui.Context) {
	// Apply active theme
	if a.themeChoice == "dark" {
		c.SetTheme(ui.DarkTheme())
	} else if a.themeChoice == "light" {
		c.SetTheme(ui.LightTheme())
	}
	t := c.Theme()

	// Fill the root window background with the theme background color
	c.Root().Background(t.Background)

	ui.Column(c).Fill().Background(t.Background).Padding(16).Gap(14).Children(func() {
		// Header Title & Theme Switcher
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, "similar-images").FontSize(22).Bold().TextColor(t.Text)
			ui.Text(c, "Visual Perceptual Search & Clustering").FontSize(14).TextColor(t.TextMuted).Grow(1)

			themeBtnLabel := "Switch to Light Theme"
			if a.themeChoice == "light" {
				themeBtnLabel = "Switch to Dark Theme"
			}
			if ui.Button(c, themeBtnLabel).Clicked() {
				if a.themeChoice == "light" {
					a.themeChoice = "dark"
				} else {
					a.themeChoice = "light"
				}
			}
		})

		// Tab Bar
		ui.Tabs(c, &a.activeTab, "1-to-N Search", "Comparison View", "Duplicate Clusters", "Cache Management")

		// Active Tab Content
		ui.Scroll(c).Grow(1).Padding(4).Children(func() {
			switch a.activeTab {
			case 0:
				a.viewSearchTab(c)
			case 1:
				a.viewComparisonTab(c)
			case 2:
				a.viewClustersTab(c)
			case 3:
				a.viewCacheTab(c)
			}
		})
	})
}

func (a *DesktopApp) viewSearchTab(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Gap(14).Children(func() {
		// Controls Card Panel
		ui.Column(c).Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(12).Children(func() {
			// Query Image Selector
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				if ui.Button(c, "Select Query Image...").Clicked() {
					go a.pickQueryImage()
				}
				pathText := a.queryPath
				if pathText == "" {
					pathText = "No query image selected"
				}
				ui.Text(c, pathText).TextColor(t.TextMuted).Grow(1)
			})

			// Search Directory Selector
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				if ui.Button(c, "Select Search Directory...").Clicked() {
					go a.pickSearchDir()
				}
				dirText := a.searchDir
				if dirText == "" {
					dirText = "No directory selected"
				}
				ui.Text(c, dirText).TextColor(t.TextMuted).Grow(1)
			})

			// Search Parameters
			ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
				ui.Text(c, "Metric:").TextColor(t.Text)
				ui.Select(c, &a.searchMetric, []string{"phash", "pdq", "wavelet", "dhash", "palette"})

				ui.Text(c, fmt.Sprintf("Threshold: %.0f%%", a.searchThreshold)).TextColor(t.Text)
				ui.Slider(c, &a.searchThreshold, 50.0, 100.0).Width(140)

				ui.Text(c, "Top-K:").TextColor(t.Text)
				ui.TextInput(c, &a.searchTopK).Width(60)

				ui.Checkbox(c, &a.searchRecursive, "Recursive")
			})

			// Action Bar
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				btn := ui.PrimaryButton(c, "Run Similarity Search")
				if a.searchRunning {
					btn.Disabled(true)
				} else if btn.Clicked() {
					go a.executeSearch()
				}

				if a.searchStatus != "" {
					ui.Text(c, a.searchStatus).TextColor(t.TextMuted)
				}
			})

			// Query Thumbnail Preview (if loaded)
			if a.queryImgThumb != nil {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Query Preview:").FontSize(12).TextColor(t.TextMuted)
					ui.Image(c, a.queryImgThumb).Size(64, 64)
				})
			}
		})

		// Results Header and List
		if a.searchReport != nil {
			matches := a.searchReport.Matches
			ui.Text(c, fmt.Sprintf("Found %d match(es) in %dms:", len(matches), a.searchReport.DurationMs)).Bold().TextColor(t.Text)

			if len(matches) == 0 {
				ui.Text(c, "No images matched the cutoff threshold.").TextColor(t.TextMuted)
			} else {
				for idx, match := range matches {
					m := match
					rank := idx + 1
					ui.Row(c).Padding(10).Radius(6).Background(t.Surface).Border(1, t.Border).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("#%d", rank)).Bold().TextColor(t.Text).Width(30)
						ui.Text(c, m.SimilarityScore.String()).Bold().TextColor(ui.Hex("#16a34a")).Width(65)
						dimStr := fmt.Sprintf("%dx%d", m.Width, m.Height)
						ui.Text(c, dimStr).TextColor(t.TextMuted).Width(90)
						ui.Text(c, formatBytes(m.FileSize)).TextColor(t.TextMuted).Width(75)
						ui.Text(c, filepath.Base(m.Path)).TextColor(t.Text).Grow(1)

						if ui.Button(c, "Compare").Clicked() {
							a.openComparison(m.Path, m.SimilarityScore.String(), m.Width, m.Height, m.FileSize)
						}
						if ui.Button(c, "Trash").Clicked() {
							_ = cluster.MoveToTrash(m.Path, a.cache)
							a.removeMatchFromReport(m.Path)
						}
					})
				}
			}
		}
	})
}

func (a *DesktopApp) viewComparisonTab(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Gap(14).Children(func() {
		if a.compareMatchPath == "" {
			ui.Column(c).Padding(24).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(8).Children(func() {
				ui.Text(c, "No candidate match selected for comparison.").TextColor(t.TextMuted)
				ui.Text(c, "Run a search and click 'Compare' next to any match card.").TextColor(t.Text)
			})
			return
		}

		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, "Similarity Score:").Bold().TextColor(t.Text)
			ui.Text(c, a.compareScore).FontSize(20).Bold().TextColor(ui.Hex("#16a34a"))
		})

		ui.Row(c).Fill().Gap(24).Children(func() {
			// Query Image Card
			ui.Column(c).Grow(1).Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(8).Children(func() {
				ui.Text(c, "Query Image").Bold().TextColor(t.Text)
				if a.compareQueryThumb != nil {
					ui.Image(c, a.compareQueryThumb).Fit(ui.Contain).Grow(1).MaxHeight(360)
				}
				ui.Text(c, a.queryPath).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, a.compareQueryMeta).FontSize(13).TextColor(t.Text)
			})

			// Match Candidate Card
			ui.Column(c).Grow(1).Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(8).Children(func() {
				ui.Text(c, "Candidate Match").Bold().TextColor(t.Text)
				if a.compareMatchThumb != nil {
					ui.Image(c, a.compareMatchThumb).Fit(ui.Contain).Grow(1).MaxHeight(360)
				}
				ui.Text(c, a.compareMatchPath).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, a.compareMatchMeta).FontSize(13).TextColor(t.Text)

				if ui.Button(c, "Move Candidate to Trash").Clicked() {
					_ = cluster.MoveToTrash(a.compareMatchPath, a.cache)
					a.compareMatchPath = ""
					a.compareMatchThumb = nil
				}
			})
		})
	})
}

func (a *DesktopApp) viewClustersTab(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Gap(14).Children(func() {
		// Target Directory Selector Card
		ui.Column(c).Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(12).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				if ui.Button(c, "Select Target Directory...").Clicked() {
					go a.pickClusterDir()
				}
				dirText := a.clusterDir
				if dirText == "" {
					dirText = "No directory selected"
				}
				ui.Text(c, dirText).TextColor(t.TextMuted).Grow(1)
			})

			// Cluster Parameters
			ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
				ui.Text(c, "Metric:").TextColor(t.Text)
				ui.Select(c, &a.clusterMetric, []string{"phash", "pdq", "wavelet", "dhash", "palette"})

				ui.Text(c, fmt.Sprintf("Threshold: %.0f%%", a.clusterThreshold)).TextColor(t.Text)
				ui.Slider(c, &a.clusterThreshold, 50.0, 100.0).Width(140)

				ui.Text(c, "Keep Rule:").TextColor(t.Text)
				ui.Select(c, &a.clusterHeuristic, []string{"resolution", "size", "oldest", "newest"})

				ui.Checkbox(c, &a.clusterRecursive, "Recursive")
			})

			// Actions
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				btn := ui.PrimaryButton(c, "Find Duplicate Clusters")
				if a.clusterRunning {
					btn.Disabled(true)
				} else if btn.Clicked() {
					go a.executeClusters()
				}

				if a.clusterStatus != "" {
					ui.Text(c, a.clusterStatus).TextColor(t.TextMuted)
				}
			})
		})

		// Clusters Output
		if a.clusterReport != nil {
			rep := a.clusterReport
			ui.Text(c, fmt.Sprintf("Found %d cluster(s) (%d redundant copies) in %dms:",
				rep.ClusterCount, rep.DuplicateCount, rep.DurationMs)).Bold().TextColor(t.Text)

			if rep.ClusterCount == 0 {
				ui.Text(c, "No duplicate clusters detected in target directory.").TextColor(t.TextMuted)
			} else {
				for _, cl := range rep.Clusters {
					clusterItem := cl
					ui.Column(c).Padding(12).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(8).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
							ui.Text(c, fmt.Sprintf("Cluster #%d (%d items)", clusterItem.ID, len(clusterItem.AllCandidates))).Bold().TextColor(t.Text)
							if ui.Button(c, "Trash All Duplicates in Cluster").Clicked() {
								for _, dup := range clusterItem.Duplicates {
									_ = cluster.MoveToTrash(dup.Path, a.cache)
								}
								a.refreshClusters()
							}
						})

						// Canonical Candidate (Keep)
						ui.Row(c).Padding(6).Radius(4).Background(t.Background).Gap(10).AlignItems(ui.Center).Children(func() {
							ui.Text(c, "[KEEP]").Bold().TextColor(ui.Hex("#16a34a")).Width(50)
							ui.Text(c, fmt.Sprintf("%dx%d", clusterItem.Canonical.Width, clusterItem.Canonical.Height)).TextColor(t.TextMuted).Width(80)
							ui.Text(c, formatBytes(clusterItem.Canonical.FileSize)).TextColor(t.TextMuted).Width(70)
							ui.Text(c, clusterItem.Canonical.Path).TextColor(t.Text).Grow(1)
						})

						// Duplicate Candidates
						for _, dup := range clusterItem.Duplicates {
							dupItem := dup
							ui.Row(c).Padding(6).Radius(4).Background(t.Background).Gap(10).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "[TRASH]").Bold().TextColor(ui.Hex("#dc2626")).Width(50)
								ui.Text(c, fmt.Sprintf("%dx%d", dupItem.Width, dupItem.Height)).TextColor(t.TextMuted).Width(80)
								ui.Text(c, formatBytes(dupItem.FileSize)).TextColor(t.TextMuted).Width(70)
								ui.Text(c, dupItem.Path).TextColor(t.Text).Grow(1)

								if ui.Button(c, "Trash").Clicked() {
									_ = cluster.MoveToTrash(dupItem.Path, a.cache)
									a.refreshClusters()
								}
							})
						}
					})
				}
			}
		}
	})
}

func (a *DesktopApp) viewCacheTab(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Gap(14).Children(func() {
		ui.Column(c).Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(12).Children(func() {
			ui.Text(c, "SQLite Fingerprint Cache").FontSize(18).Bold().TextColor(t.Text)

			ui.Text(c, a.cacheStatsText).TextColor(t.TextMuted)

			ui.Row(c).Gap(12).Children(func() {
				if ui.Button(c, "Vacuum Cache Database").Clicked() {
					if a.cache != nil {
						_ = a.cache.Vacuum()
						a.refreshCacheStats()
					}
				}
				if ui.Button(c, "Refresh Statistics").Clicked() {
					a.refreshCacheStats()
				}
			})
		})

		ui.Column(c).Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(12).Children(func() {
			ui.Text(c, "Proactive Directory Pre-Indexing").Bold().TextColor(t.Text)
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				if ui.Button(c, "Select Directory to Pre-index...").Clicked() {
					go a.pickPreindexDir()
				}
				txt := a.cacheDirInput
				if txt == "" {
					txt = "No directory chosen"
				}
				ui.Text(c, txt).TextColor(t.TextMuted).Grow(1)
			})

			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				btn := ui.PrimaryButton(c, "Start Pre-indexing")
				if a.cacheRunning || a.cacheDirInput == "" {
					btn.Disabled(true)
				} else if btn.Clicked() {
					go a.executePreindex()
				}

				if a.cacheStatus != "" {
					ui.Text(c, a.cacheStatus).TextColor(t.TextMuted)
				}
			})
		})
	})
}

// Background operations & dialog triggers

func (a *DesktopApp) pickQueryImage() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title: "Choose Query Image",
		Filters: []mygo.FileFilter{
			{Name: "Images", Extensions: []string{"jpg", "jpeg", "png", "webp", "gif", "bmp", "tif", "tiff"}},
		},
	})
	if err == nil && len(paths) > 0 {
		p := paths[0]
		img, _, err := search.OpenImage(p)
		var thumb ui.ImageSource
		if err == nil {
			thumb = ui.NewBitmap(img)
		}
		a.win.Update(func() {
			a.queryPath = p
			a.queryImgThumb = thumb
		})
	}
}

func (a *DesktopApp) pickSearchDir() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:     "Choose Candidate Images Directory",
		Directory: true,
	})
	if err == nil && len(paths) > 0 {
		a.win.Update(func() {
			a.searchDir = paths[0]
		})
	}
}

func (a *DesktopApp) pickClusterDir() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:     "Choose Target Directory to Deduplicate",
		Directory: true,
	})
	if err == nil && len(paths) > 0 {
		a.win.Update(func() {
			a.clusterDir = paths[0]
		})
	}
}

func (a *DesktopApp) pickPreindexDir() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:     "Choose Directory for Cache Pre-indexing",
		Directory: true,
	})
	if err == nil && len(paths) > 0 {
		a.win.Update(func() {
			a.cacheDirInput = paths[0]
		})
	}
}

func (a *DesktopApp) executeSearch() {
	if a.queryPath == "" || a.searchDir == "" {
		a.win.Update(func() { a.searchStatus = "Please select both query image and search directory." })
		return
	}

	a.win.Update(func() {
		a.searchRunning = true
		a.searchStatus = "Initializing search..."
		a.searchReport = nil
	})

	m, _ := engine.ParseMetric(a.searchMetric)
	topK, _ := strconv.Atoi(a.searchTopK)
	opts := search.SearchOptions{
		SearchDir:   a.searchDir,
		Metric:      m,
		Threshold:   float32(a.searchThreshold),
		TopK:        topK,
		Recursive:   a.searchRecursive,
		UseCache:    true,
	}

	queryImg, _, err := search.OpenImage(a.queryPath)
	if err != nil {
		a.win.Update(func() {
			a.searchRunning = false
			a.searchStatus = fmt.Sprintf("Error opening query image: %v", err)
		})
		return
	}

	rep, err := search.SearchDirectory(context.Background(), queryImg, a.queryPath, opts, a.cache, func(scanned, total int) {
		a.win.Update(func() {
			a.searchStatus = fmt.Sprintf("Scanning candidates: %d / %d", scanned, total)
		})
	})

	a.win.Update(func() {
		a.searchRunning = false
		if err != nil {
			a.searchStatus = fmt.Sprintf("Search error: %v", err)
		} else {
			a.searchReport = rep
			a.searchStatus = fmt.Sprintf("Completed: scanned %d images.", rep.TotalScanned)
		}
	})
}

func (a *DesktopApp) executeClusters() {
	if a.clusterDir == "" {
		a.win.Update(func() { a.clusterStatus = "Please select a target directory." })
		return
	}

	a.win.Update(func() {
		a.clusterRunning = true
		a.clusterStatus = "Scanning images..."
		a.clusterReport = nil
	})

	m, _ := engine.ParseMetric(a.clusterMetric)
	h, _ := cluster.ParseHeuristic(a.clusterHeuristic)

	rep, err := cluster.FindDuplicateClusters(
		context.Background(),
		a.clusterDir,
		m,
		float32(a.clusterThreshold),
		h,
		a.clusterRecursive,
		a.cache,
		func(scanned, total int, phase string) {
			a.win.Update(func() {
				a.clusterStatus = fmt.Sprintf("%s: %d / %d", phase, scanned, total)
			})
		},
	)

	a.win.Update(func() {
		a.clusterRunning = false
		if err != nil {
			a.clusterStatus = fmt.Sprintf("Clustering error: %v", err)
		} else {
			a.clusterReport = rep
			a.clusterStatus = fmt.Sprintf("Found %d clusters (%d duplicates).", rep.ClusterCount, rep.DuplicateCount)
		}
	})
}

func (a *DesktopApp) executePreindex() {
	dir := a.cacheDirInput
	if dir == "" || a.cache == nil {
		return
	}

	a.win.Update(func() {
		a.cacheRunning = true
		a.cacheStatus = "Discovering candidate files..."
	})

	candidates, err := search.DiscoverCandidatePaths(dir, true)
	if err != nil {
		a.win.Update(func() {
			a.cacheRunning = false
			a.cacheStatus = fmt.Sprintf("Error discovering files: %v", err)
		})
		return
	}

	eng, _ := engine.GetEngine(engine.MetricPHash)
	total := len(candidates)
	var count int

	for i, p := range candidates {
		img, meta, err := search.OpenImage(p)
		if err == nil {
			fp := eng.Compute(img)
			_ = a.cache.PutBatch([]*cache.CacheEntry{
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
		if (i+1)%25 == 0 || i+1 == total {
			done := i + 1
			a.win.Update(func() {
				a.cacheStatus = fmt.Sprintf("Indexed %d / %d images", done, total)
			})
		}
	}

	a.win.Update(func() {
		a.cacheRunning = false
		a.cacheStatus = fmt.Sprintf("Pre-indexing finished: %d images cached.", count)
		a.refreshCacheStats()
	})
}

func (a *DesktopApp) refreshClusters() {
	if a.clusterDir != "" {
		go a.executeClusters()
	}
}

func (a *DesktopApp) refreshCacheStats() {
	if a.cache == nil {
		a.cacheStatsText = "Cache disabled or unavailable"
		return
	}
	stats, err := a.cache.Stats()
	if err != nil {
		a.cacheStatsText = fmt.Sprintf("Error reading cache stats: %v", err)
		return
	}
	a.win.Update(func() {
		a.cacheStatsText = fmt.Sprintf("Entries: %d | Disk Size: %s | Location: %s",
			stats.TotalEntries, formatBytes(stats.DBSizeBytes), stats.DBPath)
	})
}

func (a *DesktopApp) openComparison(matchPath, score string, w, h int, size int64) {
	qImg, qMeta, errQ := search.OpenImage(a.queryPath)
	mImg, _, errM := search.OpenImage(matchPath)

	var qThumb, mThumb ui.ImageSource
	if errQ == nil {
		qThumb = ui.NewBitmap(qImg)
	}
	if errM == nil {
		mThumb = ui.NewBitmap(mImg)
	}

	a.win.Update(func() {
		a.compareQueryThumb = qThumb
		a.compareMatchThumb = mThumb
		a.compareMatchPath = matchPath
		a.compareScore = score
		a.compareQueryMeta = fmt.Sprintf("%dx%d | %s | %s", qMeta.Width, qMeta.Height, qMeta.Format, formatBytes(qMeta.FileSize))
		a.compareMatchMeta = fmt.Sprintf("%dx%d | %s", w, h, formatBytes(size))
		a.activeTab = 1 // Switch to Comparison tab
	})
}

func (a *DesktopApp) removeMatchFromReport(path string) {
	if a.searchReport == nil {
		return
	}
	var filtered []search.MatchResult
	for _, m := range a.searchReport.Matches {
		if m.Path != path {
			filtered = append(filtered, m)
		}
	}
	a.searchReport.Matches = filtered
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
