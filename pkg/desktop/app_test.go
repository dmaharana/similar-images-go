package desktop

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"

	"similar-images/pkg/engine"
	"similar-images/pkg/search"
)

func TestThemedLightAndDarkVisibility(t *testing.T) {
	app := NewDesktopApp()

	for _, mode := range []string{"light", "dark"} {
		app.themeChoice = mode
		tester := ui.NewTester(app.view, 1000, 750)
		img := tester.Image()

		colorCounts := make(map[color.Color]int)
		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := img.At(x, y)
				colorCounts[c]++
			}
		}

		theme := app.getTheme()
		t.Logf("=== %s MODE ===", mode)
		t.Logf("Theme Text: %v, Theme Background: %v, Surface: %v", theme.Text, theme.Background, theme.Surface)

		var textPixelCount, bgPixelCount, surfacePixelCount int
		for c, count := range colorCounts {
			rgba := c.(color.RGBA)
			if rgba.R == theme.Text.R && rgba.G == theme.Text.G && rgba.B == theme.Text.B {
				textPixelCount += count
			}
			if rgba.R == theme.Background.R && rgba.G == theme.Background.G && rgba.B == theme.Background.B {
				bgPixelCount += count
			}
			if rgba.R == theme.Surface.R && rgba.G == theme.Surface.G && rgba.B == theme.Surface.B {
				surfacePixelCount += count
			}
		}
		t.Logf("Background pixels: %d, Surface pixels: %d, Text pixels: %d", bgPixelCount, surfacePixelCount, textPixelCount)

		// Assert text is rendered and not 0 pixels
		if textPixelCount == 0 {
			t.Errorf("%s mode: text pixels count is 0, text is not visible", mode)
		}
		if bgPixelCount == 0 {
			t.Errorf("%s mode: background pixels count is 0", mode)
		}
	}
}

func TestSearchResultsGridViewAndListView(t *testing.T) {
	app := NewDesktopApp()
	app.themeChoice = "light"
	app.queryPath = "/fake/query.jpg"
	app.queryDimensions = "1920 × 1080"
	app.queryFormat = "jpeg"
	app.queryFileSize = 1024 * 500

	app.searchReport = &search.SearchReport{
		QueryPath:    app.queryPath,
		Metric:       string(engine.MetricPHash),
		TotalScanned: 10,
		Matches: []search.MatchResult{
			{
				Path:            "/fake/match1.jpg",
				SimilarityScore: engine.NewSimilarityScore(95.5),
				Width:           1920,
				Height:          1080,
				FileSize:        1024 * 450,
			},
			{
				Path:            "/fake/match2.jpg",
				SimilarityScore: engine.NewSimilarityScore(82.0),
				Width:           1280,
				Height:          720,
				FileSize:        1024 * 250,
			},
		},
		DurationMs: 15,
	}

	// Test Grid View
	app.searchViewMode = "grid"
	testerGrid := ui.NewTester(app.view, 1000, 750)
	imgGrid := testerGrid.Image()
	if imgGrid.Bounds().Dx() != 1000 {
		t.Errorf("Unexpected width: %d", imgGrid.Bounds().Dx())
	}

	// Test List View
	app.searchViewMode = "list"
	testerList := ui.NewTester(app.view, 1000, 750)
	imgList := testerList.Image()
	if imgList.Bounds().Dx() != 1000 {
		t.Errorf("Unexpected width: %d", imgList.Bounds().Dx())
	}
}

func TestComparisonViewRendering(t *testing.T) {
	app := NewDesktopApp()
	app.activeTab = 1
	app.queryPath = "/path/to/query.png"
	app.compareMatchPath = "/path/to/match.png"
	app.compareScore = "96.4%"
	app.compareScoreVal = 96.4
	app.compareQueryMeta = "1920x1080 | PNG | 2.1 MB"
	app.compareMatchMeta = "1920x1080 | 2.0 MB"

	tester := ui.NewTester(app.view, 1000, 750)
	img := tester.Image()
	if img.Bounds().Dx() != 1000 {
		t.Errorf("Unexpected width: %d", img.Bounds().Dx())
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1024 * 1024 * 2, "2.0 MB"},
	}
	for _, tc := range cases {
		got := formatBytes(tc.in)
		if got != tc.want {
			t.Errorf("formatBytes(%d) = %s, want %s", tc.in, got, tc.want)
		}
	}
}
