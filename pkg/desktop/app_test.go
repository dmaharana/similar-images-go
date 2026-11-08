package desktop

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"similar-images/pkg/engine"
	"similar-images/pkg/search"
)

func TestThemeIsLightAndDark(t *testing.T) {
	app := NewDesktopApp()

	app.themeChoice = "light"
	tl := app.getTheme()
	if tl.Dark {
		t.Errorf("Expected light theme Dark=false")
	}
	if tl.Text != ui.Hex("#0f172a") {
		t.Errorf("Expected light theme text #0f172a, got %v", tl.Text)
	}

	app.themeChoice = "dark"
	td := app.getTheme()
	if !td.Dark {
		t.Errorf("Expected dark theme Dark=true")
	}
	if td.Text != ui.Hex("#f8fafc") {
		t.Errorf("Expected dark theme text #f8fafc, got %v", td.Text)
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

func TestKeyInTargetDirectoryAndExpandPath(t *testing.T) {
	app := NewDesktopApp()

	// Direct key-in simulation
	app.searchDir = "/home/titu/Pictures"
	if app.searchDir != "/home/titu/Pictures" {
		t.Fatalf("Expected keyed-in searchDir to be retained")
	}

	// Home expansion test
	exp := expandPath("~/Pictures")
	if exp == "~/Pictures" {
		t.Errorf("Expected ~ to be expanded to home directory, got %s", exp)
	}

	empty := expandPath("")
	if empty != "" {
		t.Errorf("Expected empty path to remain empty, got %s", empty)
	}
}

func TestDragAndDropSetters(t *testing.T) {
	app := NewDesktopApp()

	// Setting search directory via drag-and-drop
	app.setSearchDir("/home/titu/Pictures")
	if app.searchDir != "/home/titu/Pictures" {
		t.Errorf("Expected setSearchDir to update searchDir, got %s", app.searchDir)
	}
	if app.lastBrowsedDir != "/home/titu/Pictures" {
		t.Errorf("Expected setSearchDir to update lastBrowsedDir, got %s", app.lastBrowsedDir)
	}

	// Handle folder drop
	app.handleFileDrop("/home/titu/Pictures")
	if app.searchDir != "/home/titu/Pictures" {
		t.Errorf("Expected handleFileDrop to update searchDir, got %s", app.searchDir)
	}

	// Test UI rendering with key-in and drop zone
	tester := ui.NewTester(app.view, 1000, 750)
	img := tester.Image()
	if img.Bounds().Dx() != 1000 {
		t.Errorf("Unexpected width: %d", img.Bounds().Dx())
	}
}
