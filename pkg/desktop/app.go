package desktop

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

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

	// Theme mode: "dark" (default) or "light"
	themeChoice string

	// Active navigation tab (0: Visual Search, 1: Comparison View, 2: Duplicate Clusters, 3: Fingerprint Cache)
	activeTab int

	// Visual Search tab state
	queryPath       string
	queryDimensions string
	queryFormat     string
	queryFileSize   int64
	queryImgRaw     image.Image
	queryImgThumb   ui.ImageSource
	lastBrowsedDir  string

	searchDir       string
	searchMetric    string
	searchThreshold float64
	searchTopK      string
	searchRecursive bool
	searchRunning   bool
	searchStatus    string
	searchReport    *search.SearchReport

	// Search results view mode: "grid" (default) or "list"
	searchViewMode string

	// Cached result thumbnails (path -> ui.ImageSource)
	resultThumbs  map[string]ui.ImageSource
	clusterThumbs map[string]ui.ImageSource
	thumbMu       sync.RWMutex

	// Comparison tab state
	compareQueryThumb ui.ImageSource
	compareMatchThumb ui.ImageSource
	compareMatchPath  string
	compareScore      string
	compareScoreVal   float32
	compareQueryMeta  string
	compareMatchMeta  string

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

// BadgeStyle holds contrasting colors for similarity and duplicate status tags.
type BadgeStyle struct {
	Bg     ui.Color
	Text   ui.Color
	Border ui.Color
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
		searchTopK:       "12",
		searchRecursive:  true,
		searchViewMode:   "grid",
		resultThumbs:     make(map[string]ui.ImageSource),
		clusterThumbs:    make(map[string]ui.ImageSource),
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
			Title:           "similar-images - Visual Perceptual Similarity & Duplicate Finder",
			Width:           1150,
			Height:          780,
			MinWidth:        850,
			MinHeight:       620,
			BackgroundColor: "#0f1117",
			Content:         ui.View(appState.view),
		})
		appState.win.OnFileDrop(func(e *mygo.FileDropEvent) {
			if len(e.Paths) > 0 {
				appState.handleFileDrop(e.Paths[0])
			}
		})
		appState.refreshCacheStats()
	})

	return mygo.App.Run()
}

// getTheme constructs the tailored theme with guaranteed high text contrast in both light and dark appearances.
func (a *DesktopApp) getTheme() *ui.Theme {
	if a.themeChoice == "light" {
		t := ui.LightTheme()
		t.Dark = false
		t.Background = ui.Hex("#eef2f6")
		t.Surface = ui.Hex("#ffffff")
		t.SurfaceHover = ui.Hex("#f1f5f9")
		t.SurfacePressed = ui.Hex("#e2e8f0")
		t.Border = ui.Hex("#94a3b8")
		t.Text = ui.Hex("#0f172a")      // Deep slate black
		t.TextMuted = ui.Hex("#334155") // High-contrast slate
		t.Accent = ui.Hex("#2563eb")
		t.AccentHover = ui.Hex("#1d4ed8")
		t.AccentPressed = ui.Hex("#1e40af")
		t.AccentText = ui.Hex("#ffffff")
		t.Radius = 8
		t.TooltipDelay = 250 * time.Millisecond
		return t
	}

	t := ui.DarkTheme()
	t.Dark = true
	t.Background = ui.Hex("#0f1117")
	t.Surface = ui.Hex("#181b22")
	t.SurfaceHover = ui.Hex("#222733")
	t.SurfacePressed = ui.Hex("#2c3242")
	t.Border = ui.Hex("#2d3340")
	t.Text = ui.Hex("#f8fafc")      // Crisp off-white
	t.TextMuted = ui.Hex("#94a3b8") // Clear legible light gray
	t.Accent = ui.Hex("#3b82f6")
	t.AccentHover = ui.Hex("#60a5fa")
	t.AccentPressed = ui.Hex("#2563eb")
	t.AccentText = ui.Hex("#ffffff")
	t.Radius = 8
	t.TooltipDelay = 250 * time.Millisecond
	return t
}

func (a *DesktopApp) getScoreBadgeStyle(score float32, isDark bool) BadgeStyle {
	if score >= 90.0 {
		if isDark {
			return BadgeStyle{
				Bg:     ui.Hex("#052e16"),
				Text:   ui.Hex("#86efac"),
				Border: ui.Hex("#166534"),
			}
		}
		return BadgeStyle{
			Bg:     ui.Hex("#dcfce7"),
			Text:   ui.Hex("#15803d"),
			Border: ui.Hex("#86efac"),
		}
	} else if score >= 80.0 {
		if isDark {
			return BadgeStyle{
				Bg:     ui.Hex("#451a03"),
				Text:   ui.Hex("#fde68a"),
				Border: ui.Hex("#92400e"),
			}
		}
		return BadgeStyle{
			Bg:     ui.Hex("#fef3c7"),
			Text:   ui.Hex("#b45309"),
			Border: ui.Hex("#fde68a"),
		}
	}
	if isDark {
		return BadgeStyle{
			Bg:     ui.Hex("#450a0a"),
			Text:   ui.Hex("#fca5a5"),
			Border: ui.Hex("#7f1d1d"),
		}
	}
	return BadgeStyle{
		Bg:     ui.Hex("#fee2e2"),
		Text:   ui.Hex("#b91c1c"),
		Border: ui.Hex("#fca5a5"),
	}
}

func (a *DesktopApp) view(c *ui.Context) {
	t := a.getTheme()
	c.SetTheme(t)
	c.Root().Background(t.Background)

	ui.Themed(c, t).Fill().Children(func() {
		ui.Column(c).Fill().Background(t.Background).Padding(16).Gap(14).Children(func() {
			// Global Header (Symmetrical with Rust Web View)
			ui.Row(c).AlignItems(ui.Center).Gap(12).Padding(2).Children(func() {
				// App Logo Icon
				ui.Row(c).Size(32, 32).Radius(8).Background(t.Accent).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(c, "❖").FontSize(17).Bold().TextColor(t.AccentText)
				})

				// Title & Subtitle
				ui.Column(c).Gap(1).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, "similar-images").FontSize(17).Bold().TextColor(t.Text)
						ui.Row(c).PaddingX(6).PaddingY(1).Radius(4).Background(t.Surface).Border(1, t.Border).Children(func() {
							ui.Text(c, "v0.1.0 Desktop").FontSize(10).Bold().TextColor(t.Accent)
						})
					})
					ui.Text(c, "Visual Perceptual Search & Duplicate Clusters Deduplication").FontSize(11).TextColor(t.TextMuted)
				})

				ui.Row(c).Grow(1) // Spacer

				// Navigation Tabs
				ui.Tabs(c, &a.activeTab, "Visual Search", "Comparison View", "Duplicate Clusters", "Fingerprint Cache")

				// Theme Switcher Button
				themeBtnLabel := "☀️ Light"
				if a.themeChoice == "light" {
					themeBtnLabel = "🌙 Dark"
				}
				if ui.Button(c, themeBtnLabel).Clicked() {
					if a.themeChoice == "light" {
						a.themeChoice = "dark"
					} else {
						a.themeChoice = "light"
					}
					if a.win != nil {
						if a.themeChoice == "light" {
							_ = a.win.SetBackgroundColor("#eef2f6")
						} else {
							_ = a.win.SetBackgroundColor("#0f1117")
						}
					}
				}
			})

			// Active Tab Content
			ui.Scroll(c).Grow(1).Padding(2).Children(func() {
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

			// Persistent Footer Status Bar
			ui.Row(c).PaddingX(12).PaddingY(6).Radius(6).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					ui.Text(c, "●").FontSize(9).TextColor(ui.Hex("#16a34a"))
					ui.Text(c, "Ready (Local GPU Engine)").FontSize(11).TextColor(t.Text)
					ui.Text(c, "|").FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, "5 Algorithms: pHash-DCT • PDQ Hash • Wavelet Haar • dHash-2D • RGB Palette").FontSize(11).TextColor(t.TextMuted)
				})
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "Safe deletion via OS Trash Bin").FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, "•").FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, "SQLite WAL Cache").FontSize(11).TextColor(t.TextMuted)
				})
			})
		})
	})
}

func (a *DesktopApp) viewSearchTab(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Gap(14).Children(func() {
		// Top 2-Card Row: Query Image Preview Card & Target Directory Card
		ui.Row(c).Fill().Gap(14).Children(func() {
			// Query Image Card
			queryCard := ui.Column(c).Grow(1).Padding(12).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(8)
			if dropped := queryCard.DroppedFiles(); len(dropped) > 0 {
				a.setQueryImage(dropped[0])
			}
			queryDragging := queryCard.FileDragOver()
			if queryDragging {
				var dragBg ui.Color
				if t.Dark {
					dragBg = ui.Hex("#1e293b")
				} else {
					dragBg = ui.Hex("#eff6ff")
				}
				queryCard.Border(2, t.Accent).Background(dragBg)
			}
			queryCard.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Text(c, "REFERENCE QUERY IMAGE").FontSize(11).Bold().TextColor(t.Accent)
						if queryDragging {
							ui.Text(c, "• Drop Image Here!").FontSize(11).Bold().TextColor(t.Accent)
						}
					})
					browseBtn := ui.Button(c, "Browse Image...")
					if browseBtn.Clicked() {
						go a.pickQueryImage()
					}
				})

				if a.queryPath != "" {
					ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(8).Radius(6).Background(t.Background).Border(1, t.Border).Children(func() {
						if a.queryImgThumb != nil {
							ui.Image(c, a.queryImgThumb).Size(72, 72).Radius(4).Fit(ui.Cover)
						} else {
							ui.Row(c).Size(72, 72).Radius(4).Background(t.SurfaceHover).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
								ui.Text(c, "🖼").FontSize(24)
							})
						}
						ui.Column(c).Grow(1).Gap(3).Children(func() {
							ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
								ui.Text(c, filepath.Base(a.queryPath)).Bold().FontSize(13).TextColor(t.Text)
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									if ui.Button(c, "📂").Height(22).Tooltip("Show in Finder").Clicked() {
										a.showInFileFinder(a.queryPath)
									}
									changeBtn := ui.Button(c, "Change").Height(22)
									if changeBtn.Clicked() {
										go a.pickQueryImage()
									}
								})
							})
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								if a.queryDimensions != "" {
									ui.Text(c, a.queryDimensions).FontSize(11).TextColor(t.TextMuted)
								}
								if a.queryFormat != "" {
									ui.Text(c, "•").FontSize(11).TextColor(t.TextMuted)
									ui.Text(c, strings.ToUpper(a.queryFormat)).FontSize(11).TextColor(t.TextMuted)
								}
								if a.queryFileSize > 0 {
									ui.Text(c, "•").FontSize(11).TextColor(t.TextMuted)
									ui.Text(c, formatBytes(a.queryFileSize)).FontSize(11).TextColor(t.TextMuted)
								}
							})
							ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
								ui.Text(c, a.queryPath).FontSize(10).TextColor(t.TextMuted)
								ui.Text(c, "(Drop image here to replace)").FontSize(10).TextColor(t.Accent)
							})
						})
					})
				} else {
					pickDropBox := ui.Column(c).Height(72).Radius(6).Background(t.Background).Border(1, t.Border).AlignItems(ui.Center).Justify(ui.Center).Gap(4)
					if queryDragging {
						pickDropBox.Border(2, t.Accent)
					}
					pickDropBox.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Text(c, "📥").FontSize(20)
							ui.Text(c, "Drag & Drop Image Here, or click Browse Image...").Bold().FontSize(12).TextColor(t.Text)
						})
						ui.Text(c, "Supports JPG, PNG, WebP, GIF, BMP, TIFF").FontSize(11).TextColor(t.TextMuted)
					})
				}
			})

			// Search Target Directory Card
			dirCard := ui.Column(c).Grow(1).Padding(12).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(8)
			if dropped := dirCard.DroppedFiles(); len(dropped) > 0 {
				a.setSearchDir(dropped[0])
			}
			dirDragging := dirCard.FileDragOver()
			if dirDragging {
				var dragBg ui.Color
				if t.Dark {
					dragBg = ui.Hex("#052e16")
				} else {
					dragBg = ui.Hex("#f0fdf4")
				}
				dirCard.Border(2, ui.Hex("#16a34a")).Background(dragBg)
			}
			dirCard.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Text(c, "SEARCH TARGET DIRECTORY").FontSize(11).Bold().TextColor(ui.Hex("#16a34a"))
						if dirDragging {
							ui.Text(c, "• Drop Folder Here!").FontSize(11).Bold().TextColor(ui.Hex("#16a34a"))
						}
					})
					chooseBtn := ui.Button(c, "Choose Folder...")
					if chooseBtn.Clicked() {
						go a.pickSearchDir()
					}
				})

				ui.Column(c).Height(72).Justify(ui.SpaceBetween).Children(func() {
					// Key-in text input
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.TextInput(c, &a.searchDir).Placeholder("Type or paste directory path (e.g. /home/user/Pictures or ~/Pictures)...").Grow(1)
					})

					// Directory validation indicator & drop hint
					dirTrim := strings.TrimSpace(a.searchDir)
					if dirTrim == "" {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, "💡 Key in path above, click Choose Folder, or drag & drop folder here").FontSize(11).TextColor(t.TextMuted)
						})
					} else {
						resolved := expandPath(dirTrim)
						st, err := os.Stat(resolved)
						if err == nil && st.IsDir() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "✓").Bold().FontSize(12).TextColor(ui.Hex("#16a34a"))
								ui.Text(c, "Ready:").Bold().FontSize(11).TextColor(ui.Hex("#16a34a"))
								ui.Text(c, resolved).FontSize(11).TextColor(t.Text)
							})
						} else if err == nil && !st.IsDir() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "⚠️").FontSize(12)
								ui.Text(c, "Path is a file, not a directory").FontSize(11).TextColor(ui.Hex("#ea580c"))
							})
						} else {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "⚠️").FontSize(12)
								ui.Text(c, "Directory not found: "+resolved).FontSize(11).TextColor(ui.Hex("#ef4444"))
							})
						}
					}
				})
			})
		})

		// Control Strip Card
		ui.Column(c).Padding(12).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(10).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
				ui.Text(c, "Metric:").Bold().FontSize(12).TextColor(t.Text)
				ui.Select(c, &a.searchMetric, []string{"phash", "pdq", "wavelet", "dhash", "palette"})

				ui.Text(c, fmt.Sprintf("Cutoff: %.0f%%", a.searchThreshold)).Bold().FontSize(12).TextColor(t.Text)
				ui.Slider(c, &a.searchThreshold, 50.0, 100.0).Width(130)

				ui.Text(c, "Top Limit:").Bold().FontSize(12).TextColor(t.Text)
				ui.TextInput(c, &a.searchTopK).Width(55)

				ui.Checkbox(c, &a.searchRecursive, "Recursive")

				ui.Row(c).Grow(1) // Spacer

				// Search Action Button
				btn := ui.PrimaryButton(c, "Start Visual Search")
				if a.searchRunning {
					btn.Disabled(true)
				} else if btn.Clicked() {
					go a.executeSearch()
				}
			})

			if a.searchStatus != "" {
				ui.Row(c).PaddingX(8).PaddingY(4).Radius(4).Background(t.Background).Children(func() {
					ui.Text(c, a.searchStatus).FontSize(11).TextColor(t.TextMuted)
				})
			}
		})

		// Search Results Section
		if a.searchReport != nil {
			matches := a.searchReport.Matches

			// Results Header with Summary and Grid/List switcher
			ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				summaryStr := fmt.Sprintf("Scanned %d images. Found %d matching candidate(s) (Cutoff: %.0f%%) in %dms",
					a.searchReport.TotalScanned, len(matches), a.searchThreshold, a.searchReport.DurationMs)
				ui.Text(c, summaryStr).Bold().FontSize(13).TextColor(t.Text)

				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					gridBtn := ui.Button(c, "⊞ Thumbnail Grid")
					if a.searchViewMode == "grid" {
						gridBtn.Background(t.Accent).TextColor(t.AccentText)
					}
					if gridBtn.Clicked() {
						a.searchViewMode = "grid"
					}

					listBtn := ui.Button(c, "☰ List View")
					if a.searchViewMode == "list" {
						listBtn.Background(t.Accent).TextColor(t.AccentText)
					}
					if listBtn.Clicked() {
						a.searchViewMode = "list"
					}
				})
			})

			if len(matches) == 0 {
				ui.Column(c).Padding(32).Radius(8).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Justify(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "🔍 No Matching Candidate Images Found").Bold().FontSize(14).TextColor(t.Text)
					ui.Text(c, fmt.Sprintf("No candidate images reached the %.0f%% similarity cutoff threshold. Try lowering the threshold or changing the metric.", a.searchThreshold)).FontSize(12).TextColor(t.TextMuted)
				})
			} else if a.searchViewMode == "grid" {
				// Thumbnail Card Grid
				ui.Row(c).Wrap().Gap(14).Children(func() {
					for idx, match := range matches {
						m := match
						rank := idx + 1

						ui.Column(c).Width(240).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(0).Children(func() {
							// Thumbnail container
							a.thumbMu.RLock()
							thumb := a.resultThumbs[m.Path]
							a.thumbMu.RUnlock()

							ui.Row(c).Width(240).Height(140).Radius(8, 8, 0, 0).Background(t.Background).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
								if thumb != nil {
									ui.Image(c, thumb).Width(240).Height(140).Fit(ui.Cover)
								} else {
									ui.Text(c, "Loading preview...").FontSize(11).TextColor(t.TextMuted)
								}
							})

							// Card body
							ui.Column(c).Padding(10).Gap(6).Children(func() {
								// Score badge & Rank
								ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
									ui.Text(c, fmt.Sprintf("#%d", rank)).Bold().FontSize(11).TextColor(t.TextMuted)

									bStyle := a.getScoreBadgeStyle(m.SimilarityScore.AsPercent(), t.Dark)
									ui.Row(c).PaddingX(6).PaddingY(2).Radius(4).Background(bStyle.Bg).Border(1, bStyle.Border).Children(func() {
										ui.Text(c, m.SimilarityScore.String()).Bold().FontSize(11).TextColor(bStyle.Text)
									})

									ui.Row(c).Grow(1)
									dimStr := fmt.Sprintf("%dx%d", m.Width, m.Height)
									ui.Text(c, dimStr).FontSize(11).TextColor(t.TextMuted)
								})

								// File name and size
								ui.Text(c, filepath.Base(m.Path)).Bold().FontSize(12).TextColor(t.Text)
								ui.Text(c, formatBytes(m.FileSize)).FontSize(11).TextColor(t.TextMuted)
								ui.Text(c, m.Path).FontSize(10).TextColor(t.TextMuted)

								// Actions row: single row with icons and hover tooltips
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									if ui.Button(c, "👁").Grow(1).Tooltip("Compare").Clicked() {
										a.openComparison(m.Path, m.SimilarityScore.String(), m.SimilarityScore.AsPercent(), m.Width, m.Height, m.FileSize)
									}
									if ui.Button(c, "📂").Grow(1).Tooltip("Show in Finder").Clicked() {
										a.showInFileFinder(m.Path)
									}
									if ui.Button(c, "🗑").Grow(1).Tooltip("Move to Trash").Clicked() {
										_ = cluster.MoveToTrash(m.Path, a.cache)
										a.removeMatchFromReport(m.Path)
									}
								})
							})
						})
					}
				})
			} else {
				// Tabular List View
				ui.Column(c).Gap(6).Children(func() {
					for idx, match := range matches {
						m := match
						rank := idx + 1
						ui.Row(c).Padding(10).Radius(6).Background(t.Surface).Border(1, t.Border).Gap(10).AlignItems(ui.Center).Children(func() {
							ui.Text(c, fmt.Sprintf("#%d", rank)).Bold().TextColor(t.TextMuted).Width(35)

							bStyle := a.getScoreBadgeStyle(m.SimilarityScore.AsPercent(), t.Dark)
							ui.Row(c).PaddingX(6).PaddingY(2).Radius(4).Background(bStyle.Bg).Border(1, bStyle.Border).Width(70).Justify(ui.Center).Children(func() {
								ui.Text(c, m.SimilarityScore.String()).Bold().FontSize(11).TextColor(bStyle.Text)
							})

							dimStr := fmt.Sprintf("%dx%d", m.Width, m.Height)
							ui.Text(c, dimStr).TextColor(t.TextMuted).Width(95)
							ui.Text(c, formatBytes(m.FileSize)).TextColor(t.TextMuted).Width(80)
							ui.Text(c, filepath.Base(m.Path)).Bold().TextColor(t.Text).Width(160)
							ui.Text(c, m.Path).FontSize(11).TextColor(t.TextMuted).Grow(1)

							if ui.Button(c, "👁 Compare").Tooltip("Compare").Clicked() {
								a.openComparison(m.Path, m.SimilarityScore.String(), m.SimilarityScore.AsPercent(), m.Width, m.Height, m.FileSize)
							}
							if ui.Button(c, "📂 Finder").Tooltip("Show in Finder").Clicked() {
								a.showInFileFinder(m.Path)
							}
							if ui.Button(c, "🗑 Trash").Tooltip("Move to Trash").Clicked() {
								_ = cluster.MoveToTrash(m.Path, a.cache)
								a.removeMatchFromReport(m.Path)
							}
						})
					}
				})
			}
		}
	})
}

func (a *DesktopApp) viewComparisonTab(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Gap(14).Children(func() {
		if a.compareMatchPath == "" {
			ui.Column(c).Padding(32).Radius(8).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Justify(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "No candidate match selected for comparison.").Bold().FontSize(14).TextColor(t.Text)
				ui.Text(c, "Run a Visual Search and click 'Compare' next to any candidate match card.").FontSize(12).TextColor(t.TextMuted)
			})
			return
		}

		// Comparison Header
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Text(c, "Side-by-Side Comparison").FontSize(16).Bold().TextColor(t.Text)
				bStyle := a.getScoreBadgeStyle(a.compareScoreVal, t.Dark)
				ui.Row(c).PaddingX(8).PaddingY(2).Radius(4).Background(bStyle.Bg).Border(1, bStyle.Border).Children(func() {
					ui.Text(c, fmt.Sprintf("%s Match", a.compareScore)).Bold().FontSize(12).TextColor(bStyle.Text)
				})
			})

			if ui.Button(c, "← Back to Search Results").Clicked() {
				a.activeTab = 0
			}
		})

		// Dual Image Display
		ui.Row(c).Fill().Gap(16).Children(func() {
			// Query Image Card
			ui.Column(c).Grow(1).Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(8).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Text(c, "REFERENCE QUERY IMAGE").Bold().FontSize(11).TextColor(t.Accent)
					if ui.Button(c, "Show in Finder").Clicked() {
						a.showInFileFinder(a.queryPath)
					}
				})
				ui.Row(c).Height(380).Radius(6).Background(t.Background).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					if a.compareQueryThumb != nil {
						ui.Image(c, a.compareQueryThumb).Fit(ui.Contain).MaxHeight(370)
					}
				})
				ui.Text(c, filepath.Base(a.queryPath)).Bold().FontSize(13).TextColor(t.Text)
				ui.Text(c, a.compareQueryMeta).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, a.queryPath).FontSize(10).TextColor(t.TextMuted)
			})

			// Match Candidate Card
			ui.Column(c).Grow(1).Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(8).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Text(c, "CANDIDATE MATCH").Bold().FontSize(11).TextColor(ui.Hex("#16a34a"))
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						if ui.Button(c, "Show in Finder").Clicked() {
							a.showInFileFinder(a.compareMatchPath)
						}
						if ui.Button(c, "Move Candidate to Trash").Clicked() {
							_ = cluster.MoveToTrash(a.compareMatchPath, a.cache)
							a.removeMatchFromReport(a.compareMatchPath)
							a.compareMatchPath = ""
							a.compareMatchThumb = nil
							a.activeTab = 0
						}
					})
				})
				ui.Row(c).Height(380).Radius(6).Background(t.Background).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					if a.compareMatchThumb != nil {
						ui.Image(c, a.compareMatchThumb).Fit(ui.Contain).MaxHeight(370)
					}
				})
				ui.Text(c, filepath.Base(a.compareMatchPath)).Bold().FontSize(13).TextColor(t.Text)
				ui.Text(c, a.compareMatchMeta).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, a.compareMatchPath).FontSize(10).TextColor(t.TextMuted)
			})
		})
	})
}

func (a *DesktopApp) viewClustersTab(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Gap(14).Children(func() {
		// Target Directory Selector Card
		clusterCard := ui.Column(c).Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(12)
		if dropped := clusterCard.DroppedFiles(); len(dropped) > 0 {
			a.setClusterDir(dropped[0])
		}
		clusterDragging := clusterCard.FileDragOver()
		if clusterDragging {
			var dragBg ui.Color
			if t.Dark {
				dragBg = ui.Hex("#052e16")
			} else {
				dragBg = ui.Hex("#f0fdf4")
			}
			clusterCard.Border(2, ui.Hex("#16a34a")).Background(dragBg)
		}
		clusterCard.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Text(c, "Duplicate Clusters Deduplication").FontSize(15).Bold().TextColor(t.Text)
						if clusterDragging {
							ui.Text(c, "• Drop Folder Here!").FontSize(12).Bold().TextColor(ui.Hex("#16a34a"))
						}
					})
					ui.Text(c, "Identify all groups of visually identical and near-duplicate images within a directory.").FontSize(11).TextColor(t.TextMuted)
				})
				if ui.Button(c, "Choose Directory...").Clicked() {
					go a.pickClusterDir()
				}
			})

			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.TextInput(c, &a.clusterDir).Placeholder("Type or paste target directory path (e.g. /home/user/Pictures or ~/Pictures)...").Grow(1)
			})

			dirTrim := strings.TrimSpace(a.clusterDir)
			if dirTrim == "" {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "💡 Key in directory path above, click Choose Directory, or drag & drop folder here").FontSize(11).TextColor(t.TextMuted)
				})
			} else {
				resolved := expandPath(dirTrim)
				st, err := os.Stat(resolved)
				if err == nil && st.IsDir() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "✓").Bold().FontSize(12).TextColor(ui.Hex("#16a34a"))
						ui.Text(c, "Ready:").Bold().FontSize(11).TextColor(ui.Hex("#16a34a"))
						ui.Text(c, resolved).FontSize(11).TextColor(t.Text)
					})
				} else {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "⚠️").FontSize(12)
						ui.Text(c, "Directory not found: "+resolved).FontSize(11).TextColor(ui.Hex("#ef4444"))
					})
				}
			}

			// Cluster Parameters
			ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
				ui.Text(c, "Metric:").Bold().FontSize(12).TextColor(t.Text)
				ui.Select(c, &a.clusterMetric, []string{"phash", "pdq", "wavelet", "dhash", "palette"})

				ui.Text(c, fmt.Sprintf("Cutoff: %.0f%%", a.clusterThreshold)).Bold().FontSize(12).TextColor(t.Text)
				ui.Slider(c, &a.clusterThreshold, 50.0, 100.0).Width(130)

				ui.Text(c, "Keep Rule:").Bold().FontSize(12).TextColor(t.Text)
				ui.Select(c, &a.clusterHeuristic, []string{"resolution", "size", "oldest", "newest"})

				ui.Checkbox(c, &a.clusterRecursive, "Recursive")

				ui.Row(c).Grow(1) // Spacer

				btn := ui.PrimaryButton(c, "Find Duplicate Clusters")
				if a.clusterRunning {
					btn.Disabled(true)
				} else if btn.Clicked() {
					go a.executeClusters()
				}
			})

			if a.clusterStatus != "" {
				ui.Row(c).PaddingX(8).PaddingY(4).Radius(4).Background(t.Background).Children(func() {
					ui.Text(c, a.clusterStatus).FontSize(11).TextColor(t.TextMuted)
				})
			}
		})

		// Clusters Output
		if a.clusterReport != nil {
			rep := a.clusterReport

			ui.Row(c).Padding(10).Radius(6).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				summaryStr := fmt.Sprintf("Scanned %d images. Found %d duplicate cluster(s) with %d redundant copy(ies) in %dms",
					rep.TotalScanned, rep.ClusterCount, rep.DuplicateCount, rep.DurationMs)
				ui.Text(c, summaryStr).Bold().FontSize(13).TextColor(t.Text)
			})

			if rep.ClusterCount == 0 {
				ui.Column(c).Padding(32).Radius(8).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Justify(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "✓ No Duplicate Clusters Detected").Bold().FontSize(14).TextColor(ui.Hex("#16a34a"))
					ui.Text(c, "All scanned images appear visually distinct at the chosen threshold.").FontSize(12).TextColor(t.TextMuted)
				})
			} else {
				for _, cl := range rep.Clusters {
					clusterItem := cl
					ui.Column(c).Padding(12).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(10).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							ui.Text(c, fmt.Sprintf("Cluster #%d (%d variants)", clusterItem.ID, len(clusterItem.AllCandidates))).Bold().FontSize(13).TextColor(t.Text)
							if ui.Button(c, fmt.Sprintf("Trash %d Duplicate(s)", len(clusterItem.Duplicates))).Clicked() {
								for _, dup := range clusterItem.Duplicates {
									_ = cluster.MoveToTrash(dup.Path, a.cache)
								}
								a.refreshClusters()
							}
						})

						// Candidate Images Grid
						ui.Row(c).Wrap().Gap(12).Children(func() {
							// Canonical Image Card (Keep)
							ui.Column(c).Width(230).Radius(8).Background(t.Background).Border(1, ui.Hex("#16a34a")).Children(func() {
								a.thumbMu.RLock()
								cThumb := a.clusterThumbs[clusterItem.Canonical.Path]
								a.thumbMu.RUnlock()

								ui.Row(c).Width(230).Height(130).Radius(8, 8, 0, 0).Background(t.Surface).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
									if cThumb != nil {
										ui.Image(c, cThumb).Width(230).Height(130).Fit(ui.Cover)
									} else {
										ui.Text(c, "Loading...").FontSize(10).TextColor(t.TextMuted)
									}
								})
								ui.Column(c).Padding(8).Gap(4).Children(func() {
									ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
										ui.Row(c).PaddingX(4).PaddingY(1).Radius(3).Background(ui.Hex("#dcfce7")).Children(func() {
											ui.Text(c, "KEEP (ORIGINAL)").Bold().FontSize(10).TextColor(ui.Hex("#166534"))
										})
										if ui.Button(c, "📂").Tooltip("Show in Finder").Clicked() {
											a.showInFileFinder(clusterItem.Canonical.Path)
										}
									})
									ui.Text(c, filepath.Base(clusterItem.Canonical.Path)).Bold().FontSize(11).TextColor(t.Text)
									dimStr := fmt.Sprintf("%dx%d • %s", clusterItem.Canonical.Width, clusterItem.Canonical.Height, formatBytes(clusterItem.Canonical.FileSize))
									ui.Text(c, dimStr).FontSize(10).TextColor(t.TextMuted)
									ui.Text(c, clusterItem.Canonical.Path).FontSize(9).TextColor(t.TextMuted)
								})
							})

							// Duplicate Image Cards
							for _, dup := range clusterItem.Duplicates {
								dupItem := dup
								ui.Column(c).Width(230).Radius(8).Background(t.Background).Border(1, ui.Hex("#dc2626")).Children(func() {
									a.thumbMu.RLock()
									dThumb := a.clusterThumbs[dupItem.Path]
									a.thumbMu.RUnlock()

									ui.Row(c).Width(230).Height(130).Radius(8, 8, 0, 0).Background(t.Surface).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
										if dThumb != nil {
											ui.Image(c, dThumb).Width(230).Height(130).Fit(ui.Cover)
										} else {
											ui.Text(c, "Loading...").FontSize(10).TextColor(t.TextMuted)
										}
									})
									ui.Column(c).Padding(8).Gap(4).Children(func() {
										ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
											ui.Row(c).PaddingX(4).PaddingY(1).Radius(3).Background(ui.Hex("#fee2e2")).Children(func() {
												ui.Text(c, "DUPLICATE").Bold().FontSize(10).TextColor(ui.Hex("#991b1b"))
											})
											ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
												if ui.Button(c, "📂").Tooltip("Show in Finder").Clicked() {
													a.showInFileFinder(dupItem.Path)
												}
												if ui.Button(c, "🗑").Tooltip("Move to Trash").Clicked() {
													_ = cluster.MoveToTrash(dupItem.Path, a.cache)
													a.refreshClusters()
												}
											})
										})
										ui.Text(c, filepath.Base(dupItem.Path)).Bold().FontSize(11).TextColor(t.Text)
										dimStr := fmt.Sprintf("%dx%d • %s", dupItem.Width, dupItem.Height, formatBytes(dupItem.FileSize))
										ui.Text(c, dimStr).FontSize(10).TextColor(t.TextMuted)
										ui.Text(c, dupItem.Path).FontSize(9).TextColor(t.TextMuted)
									})
								})
							}
						})
					})
				}
			}
		}
	})
}

func (a *DesktopApp) viewCacheTab(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Gap(14).Children(func() {
		// Cache Stats Card
		ui.Column(c).Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(12).Children(func() {
			ui.Text(c, "SQLite Fingerprint Cache").FontSize(16).Bold().TextColor(t.Text)
			ui.Text(c, "Stores computed perceptual hashes across runs to eliminate duplicate image I/O.").FontSize(11).TextColor(t.TextMuted)

			ui.Row(c).Padding(10).Radius(6).Background(t.Background).Border(1, t.Border).Children(func() {
				ui.Text(c, a.cacheStatsText).FontSize(12).TextColor(t.Text)
			})

			ui.Row(c).Gap(10).Children(func() {
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

		// Directory Pre-Indexing Card
		ui.Column(c).Padding(14).Radius(8).Background(t.Surface).Border(1, t.Border).Gap(12).Children(func() {
			ui.Text(c, "Proactive Directory Pre-Indexing").FontSize(14).Bold().TextColor(t.Text)
			ui.Text(c, "Index an entire photo library ahead of time to make future similarity searches instantaneous.").FontSize(11).TextColor(t.TextMuted)

			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				if ui.Button(c, "Select Directory to Pre-index...").Clicked() {
					go a.pickPreindexDir()
				}
				txt := a.cacheDirInput
				if txt == "" {
					txt = "No directory chosen"
				}
				ui.Text(c, txt).FontSize(11).TextColor(t.TextMuted).Grow(1)
			})

			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				btn := ui.PrimaryButton(c, "Start Pre-indexing")
				if a.cacheRunning || a.cacheDirInput == "" {
					btn.Disabled(true)
				} else if btn.Clicked() {
					go a.executePreindex()
				}

				if a.cacheStatus != "" {
					ui.Text(c, a.cacheStatus).FontSize(11).TextColor(t.TextMuted)
				}
			})
		})
	})
}

// Background operations & dialog triggers

func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func (a *DesktopApp) setQueryImage(p string) {
	p = expandPath(p)
	if p == "" {
		return
	}
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return
	}
	img, meta, err := search.OpenImage(p)
	if err != nil {
		return
	}
	thumbImg := search.CreateThumbnail(img, 240)
	thumb := ui.NewBitmap(thumbImg)
	a.lastBrowsedDir = filepath.Dir(p)

	updateFn := func() {
		a.queryPath = p
		a.queryDimensions = fmt.Sprintf("%d × %d", meta.Width, meta.Height)
		a.queryFormat = meta.Format
		a.queryFileSize = meta.FileSize
		a.queryImgRaw = thumbImg
		a.queryImgThumb = thumb
	}
	if a.win != nil {
		a.win.Update(updateFn)
	} else {
		updateFn()
	}
}

func (a *DesktopApp) setSearchDir(p string) {
	p = expandPath(p)
	if p == "" {
		return
	}
	st, err := os.Stat(p)
	if err == nil && !st.IsDir() {
		p = filepath.Dir(p)
	}
	a.lastBrowsedDir = p
	updateFn := func() {
		a.searchDir = p
	}
	if a.win != nil {
		a.win.Update(updateFn)
	} else {
		updateFn()
	}
}

func (a *DesktopApp) setClusterDir(p string) {
	p = expandPath(p)
	if p == "" {
		return
	}
	st, err := os.Stat(p)
	if err == nil && !st.IsDir() {
		p = filepath.Dir(p)
	}
	a.lastBrowsedDir = p
	updateFn := func() {
		a.clusterDir = p
	}
	if a.win != nil {
		a.win.Update(updateFn)
	} else {
		updateFn()
	}
}

func (a *DesktopApp) handleFileDrop(p string) {
	p = expandPath(p)
	st, err := os.Stat(p)
	if err != nil {
		return
	}
	if st.IsDir() {
		if a.activeTab == 2 {
			a.setClusterDir(p)
		} else {
			a.setSearchDir(p)
		}
	} else {
		a.setQueryImage(p)
	}
}

func (a *DesktopApp) pickQueryImage() {
	defPath := a.queryPath
	if defPath == "" {
		defPath = a.searchDir
	}
	if defPath == "" {
		defPath = a.lastBrowsedDir
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       "Select Reference Query Image",
		DefaultPath: defPath,
		Filters: []mygo.FileFilter{
			{Name: "Image files", Extensions: []string{"jpg", "jpeg", "png", "webp", "gif", "bmp", "tif", "tiff"}},
		},
	})
	if err == nil && len(paths) > 0 {
		a.setQueryImage(paths[0])
	}
}

func (a *DesktopApp) pickSearchDir() {
	defPath := a.searchDir
	if defPath == "" {
		if a.queryPath != "" {
			defPath = filepath.Dir(a.queryPath)
		} else {
			defPath = a.lastBrowsedDir
		}
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       "Choose Candidate Images Directory",
		Directory:   true,
		DefaultPath: defPath,
	})
	if err == nil && len(paths) > 0 {
		a.setSearchDir(paths[0])
	}
}

func (a *DesktopApp) pickClusterDir() {
	defPath := a.clusterDir
	if defPath == "" {
		defPath = a.searchDir
	}
	if defPath == "" {
		defPath = a.lastBrowsedDir
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       "Choose Target Directory to Deduplicate",
		Directory:   true,
		DefaultPath: defPath,
	})
	if err == nil && len(paths) > 0 {
		a.setClusterDir(paths[0])
	}
}

func (a *DesktopApp) pickPreindexDir() {
	defPath := a.cacheDirInput
	if defPath == "" {
		defPath = a.searchDir
	}
	if defPath == "" {
		defPath = a.lastBrowsedDir
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       "Choose Directory for Cache Pre-indexing",
		Directory:   true,
		DefaultPath: defPath,
	})
	if err == nil && len(paths) > 0 {
		a.lastBrowsedDir = paths[0]
		if a.win != nil {
			a.win.Update(func() {
				a.cacheDirInput = paths[0]
			})
		}
	}
}

func (a *DesktopApp) executeSearch() {
	resolvedQuery := expandPath(strings.TrimSpace(a.queryPath))
	resolvedSearch := expandPath(strings.TrimSpace(a.searchDir))

	if resolvedQuery == "" || resolvedSearch == "" {
		if a.win != nil {
			a.win.Update(func() { a.searchStatus = "Please select both query image and search directory." })
		}
		return
	}

	st, err := os.Stat(resolvedSearch)
	if err != nil || !st.IsDir() {
		if a.win != nil {
			a.win.Update(func() { a.searchStatus = "Target search directory does not exist: " + resolvedSearch })
		}
		return
	}

	if a.win != nil {
		a.win.Update(func() {
			a.searchRunning = true
			a.searchStatus = "Initializing search..."
			a.searchReport = nil
		})
	}

	m, _ := engine.ParseMetric(a.searchMetric)
	topK, _ := strconv.Atoi(a.searchTopK)
	opts := search.SearchOptions{
		SearchDir:   resolvedSearch,
		Metric:      m,
		Threshold:   float32(a.searchThreshold),
		TopK:        topK,
		Recursive:   a.searchRecursive,
		UseCache:    true,
	}

	queryImg, _, err := search.OpenImage(resolvedQuery)
	if err != nil {
		if a.win != nil {
			a.win.Update(func() {
				a.searchRunning = false
				a.searchStatus = fmt.Sprintf("Error opening query image: %v", err)
			})
		}
		return
	}

	rep, err := search.SearchDirectory(context.Background(), queryImg, resolvedQuery, opts, a.cache, func(scanned, total int) {
		if a.win != nil {
			a.win.Update(func() {
				a.searchStatus = fmt.Sprintf("Scanning candidates: %d / %d", scanned, total)
			})
		}
	})

	a.thumbMu.Lock()
	a.resultThumbs = make(map[string]ui.ImageSource)
	a.thumbMu.Unlock()

	if a.win != nil {
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

	// Preload thumbnails asynchronously in background
	if rep != nil && len(rep.Matches) > 0 {
		go func(matches []search.MatchResult) {
			for _, match := range matches {
				img, _, err := search.OpenImage(match.Path)
				if err == nil {
					thumbImg := search.CreateThumbnail(img, 240)
					bm := ui.NewBitmap(thumbImg)
					a.thumbMu.Lock()
					a.resultThumbs[match.Path] = bm
					a.thumbMu.Unlock()
					if a.win != nil {
						a.win.Update(func() {})
					}
				}
			}
		}(rep.Matches)
	}
}

func (a *DesktopApp) executeClusters() {
	resolvedCluster := expandPath(strings.TrimSpace(a.clusterDir))
	if resolvedCluster == "" {
		if a.win != nil {
			a.win.Update(func() { a.clusterStatus = "Please select a target directory." })
		}
		return
	}

	st, err := os.Stat(resolvedCluster)
	if err != nil || !st.IsDir() {
		if a.win != nil {
			a.win.Update(func() { a.clusterStatus = "Target directory does not exist: " + resolvedCluster })
		}
		return
	}

	if a.win != nil {
		a.win.Update(func() {
			a.clusterRunning = true
			a.clusterStatus = "Scanning images..."
			a.clusterReport = nil
		})
	}

	m, _ := engine.ParseMetric(a.clusterMetric)
	h, _ := cluster.ParseHeuristic(a.clusterHeuristic)

	rep, err := cluster.FindDuplicateClusters(
		context.Background(),
		resolvedCluster,
		m,
		float32(a.clusterThreshold),
		h,
		a.clusterRecursive,
		a.cache,
		func(scanned, total int, phase string) {
			if a.win != nil {
				a.win.Update(func() {
					a.clusterStatus = fmt.Sprintf("%s: %d / %d", phase, scanned, total)
				})
			}
		},
	)

	a.thumbMu.Lock()
	a.clusterThumbs = make(map[string]ui.ImageSource)
	a.thumbMu.Unlock()

	if a.win != nil {
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

	// Preload cluster thumbnails in background
	if rep != nil && len(rep.Clusters) > 0 {
		go func(clusters []cluster.Cluster) {
			for _, cl := range clusters {
				for _, cand := range cl.AllCandidates {
					img, _, err := search.OpenImage(cand.Path)
					if err == nil {
						thumbImg := search.CreateThumbnail(img, 240)
						bm := ui.NewBitmap(thumbImg)
						a.thumbMu.Lock()
						a.clusterThumbs[cand.Path] = bm
						a.thumbMu.Unlock()
						if a.win != nil {
							a.win.Update(func() {})
						}
					}
				}
			}
		}(rep.Clusters)
	}
}

func (a *DesktopApp) executePreindex() {
	dir := expandPath(strings.TrimSpace(a.cacheDirInput))
	if dir == "" || a.cache == nil {
		return
	}

	if a.win != nil {
		a.win.Update(func() {
			a.cacheRunning = true
			a.cacheStatus = "Discovering candidate files..."
		})
	}

	candidates, err := search.DiscoverCandidatePaths(dir, true)
	if err != nil {
		if a.win != nil {
			a.win.Update(func() {
				a.cacheRunning = false
				a.cacheStatus = fmt.Sprintf("Error discovering files: %v", err)
			})
		}
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
			if a.win != nil {
				a.win.Update(func() {
					a.cacheStatus = fmt.Sprintf("Indexed %d / %d images", done, total)
				})
			}
		}
	}

	if a.win != nil {
		a.win.Update(func() {
			a.cacheRunning = false
			a.cacheStatus = fmt.Sprintf("Pre-indexing finished: %d images cached.", count)
			a.refreshCacheStats()
		})
	}
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
	if a.win != nil {
		a.win.Update(func() {
			a.cacheStatsText = fmt.Sprintf("Entries: %d | Disk Size: %s | Location: %s",
				stats.TotalEntries, formatBytes(stats.DBSizeBytes), stats.DBPath)
		})
	} else {
		a.cacheStatsText = fmt.Sprintf("Entries: %d | Disk Size: %s | Location: %s",
			stats.TotalEntries, formatBytes(stats.DBSizeBytes), stats.DBPath)
	}
}

func (a *DesktopApp) openComparison(matchPath, score string, scoreVal float32, w, h int, size int64) {
	qImg, qMeta, errQ := search.OpenImage(a.queryPath)
	mImg, _, errM := search.OpenImage(matchPath)

	var qThumb, mThumb ui.ImageSource
	if errQ == nil {
		qThumb = ui.NewBitmap(qImg)
	}
	if errM == nil {
		mThumb = ui.NewBitmap(mImg)
	}

	if a.win != nil {
		a.win.Update(func() {
			a.compareQueryThumb = qThumb
			a.compareMatchThumb = mThumb
			a.compareMatchPath = matchPath
			a.compareScore = score
			a.compareScoreVal = scoreVal
			a.compareQueryMeta = fmt.Sprintf("%dx%d | %s | %s", qMeta.Width, qMeta.Height, qMeta.Format, formatBytes(qMeta.FileSize))
			a.compareMatchMeta = fmt.Sprintf("%dx%d | %s", w, h, formatBytes(size))
			a.activeTab = 1 // Switch to Comparison tab
		})
	}
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

func (a *DesktopApp) showInFileFinder(path string) {
	if path == "" {
		return
	}
	err := ShowInFileFinder(path)
	var statusMsg string
	if err != nil {
		statusMsg = fmt.Sprintf("Error opening file finder: %v", err)
	} else {
		statusMsg = fmt.Sprintf("Revealed in file finder: %s", filepath.Base(path))
	}

	updateState := func() {
		if a.activeTab == 2 {
			a.clusterStatus = statusMsg
		} else {
			a.searchStatus = statusMsg
		}
	}

	if a.win != nil {
		a.win.Update(updateState)
	} else {
		updateState()
	}
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
