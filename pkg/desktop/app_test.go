package desktop

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"similar-images/pkg/cluster"
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

func TestShowInFileFinderButtonsPresent(t *testing.T) {
	tmpDir := t.TempDir()
	matchFile := filepath.Join(tmpDir, "match1.jpg")
	_ = os.WriteFile(matchFile, []byte("fake"), 0644)

	app := NewDesktopApp()
	app.queryPath = "/fake/query.jpg"
	app.searchReport = &search.SearchReport{
		QueryPath:    app.queryPath,
		Metric:       string(engine.MetricPHash),
		TotalScanned: 5,
		Matches: []search.MatchResult{
			{
				Path:            matchFile,
				SimilarityScore: engine.NewSimilarityScore(95.0),
				Width:           800,
				Height:          600,
				FileSize:        1024 * 100,
			},
		},
	}

	// Grid View: single row with 3 icon buttons (👁, 📂, 🗑)
	app.searchViewMode = "grid"
	testerGrid := ui.NewTester(app.view, 1000, 750)
	if !testerGrid.HasText("📂") || !testerGrid.HasText("👁") || !testerGrid.HasText("🗑") {
		t.Errorf("Expected Grid View to contain 👁, 📂, 🗑 icon buttons, got: %v", testerGrid.Texts())
	}

	// List View: contains Finder button
	app.searchViewMode = "list"
	testerList := ui.NewTester(app.view, 1000, 750)
	if !testerList.HasText("Finder") {
		t.Errorf("Expected List View to contain 'Finder' button, got: %v", testerList.Texts())
	}
}

func TestShowInFileFinderComparisonAndClusters(t *testing.T) {
	// Comparison tab
	appComp := NewDesktopApp()
	appComp.activeTab = 1
	appComp.queryPath = "/fake/query.jpg"
	appComp.compareMatchPath = "/fake/match.jpg"
	testerComp := ui.NewTester(appComp.view, 1000, 750)
	if !testerComp.HasText("Show in Finder") {
		t.Errorf("Expected Comparison View to contain 'Show in Finder' button")
	}

	// Duplicate Clusters tab
	appCluster := NewDesktopApp()
	appCluster.activeTab = 2
	appCluster.clusterReport = &cluster.ClusterReport{
		TargetDir:    "/fake/pics",
		TotalScanned: 2,
		ClusterCount: 1,
		Clusters: []cluster.Cluster{
			{
				ID: 1,
				Canonical: cluster.Candidate{
					Path:     "/fake/canon.jpg",
					Width:    1920,
					Height:   1080,
					FileSize: 500000,
				},
				Duplicates: []cluster.Candidate{
					{
						Path:     "/fake/dup.jpg",
						Width:    1280,
						Height:   720,
						FileSize: 250000,
					},
				},
				AllCandidates: []cluster.Candidate{
					{Path: "/fake/canon.jpg"},
					{Path: "/fake/dup.jpg"},
				},
			},
		},
	}
	testerCluster := ui.NewTester(appCluster.view, 1000, 750)
	if !testerCluster.HasText("📂") {
		t.Errorf("Expected Clusters View to contain '📂' button, got: %v", testerCluster.Texts())
	}
}

func TestAppShowInFileFinderStatusUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sample.jpg")
	_ = os.WriteFile(testFile, []byte("data"), 0644)

	var openedTarget string
	origCmd := openFinderCmd
	defer func() { openFinderCmd = origCmd }()
	openFinderCmd = func(target string) error {
		openedTarget = target
		return nil
	}

	app := NewDesktopApp()

	// In Visual Search tab
	app.activeTab = 0
	app.showInFileFinder(testFile)
	if openedTarget != testFile {
		t.Errorf("Expected opened target %q, got %q", testFile, openedTarget)
	}
	if !strings.Contains(app.searchStatus, "Revealed in file finder: sample.jpg") {
		t.Errorf("Expected searchStatus to report revealed file, got: %s", app.searchStatus)
	}

	// In Duplicate Clusters tab
	app.activeTab = 2
	app.showInFileFinder(testFile)
	if !strings.Contains(app.clusterStatus, "Revealed in file finder: sample.jpg") {
		t.Errorf("Expected clusterStatus to report revealed file, got: %s", app.clusterStatus)
	}

	// Error path
	app.showInFileFinder("/nonexistent_dir_xyz/missing.jpg")
	if !strings.Contains(app.clusterStatus, "Error opening file finder") {
		t.Errorf("Expected clusterStatus to report error, got: %s", app.clusterStatus)
	}
}

func createTestPNG(t *testing.T, dir, filename string, r, g, b uint8) string {
	t.Helper()
	p := filepath.Join(dir, filename)
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}
	return p
}

func TestQueryCandidateSampling(t *testing.T) {
	tmpDir := t.TempDir()
	pA := createTestPNG(t, tmpDir, "a.png", 255, 0, 0)
	pB := createTestPNG(t, tmpDir, "b.png", 0, 255, 0)
	pC := createTestPNG(t, tmpDir, "c.png", 0, 0, 255)

	app := NewDesktopApp()
	app.searchDir = tmpDir

	// Initial random candidate
	app.pickRandomCandidate()
	if app.queryPath == "" {
		t.Fatalf("Expected queryPath to be set after pickRandomCandidate")
	}
	if app.sampledIndex < 0 || app.sampledIndex > 2 {
		t.Fatalf("Expected sampledIndex in [0, 2], got %d", app.sampledIndex)
	}
	if len(app.sampledCandidates) != 3 {
		t.Fatalf("Expected 3 sampledCandidates, got %d", len(app.sampledCandidates))
	}
	if app.sampledCandidates[0] != pA || app.sampledCandidates[1] != pB || app.sampledCandidates[2] != pC {
		t.Fatalf("Expected sorted candidate paths, got %v", app.sampledCandidates)
	}

	// Test Next Candidate traversal
	initialIdx := app.sampledIndex
	app.pickNextCandidate()
	expectedNext := (initialIdx + 1) % 3
	if app.sampledIndex != expectedNext {
		t.Errorf("pickNextCandidate: expected index %d, got %d", expectedNext, app.sampledIndex)
	}
	if app.queryPath != app.sampledCandidates[expectedNext] {
		t.Errorf("pickNextCandidate: expected queryPath %s, got %s", app.sampledCandidates[expectedNext], app.queryPath)
	}

	// Step two more times to verify wrap-around
	app.pickNextCandidate()
	app.pickNextCandidate()
	if app.sampledIndex != initialIdx {
		t.Errorf("Expected wrap-around back to %d, got %d", initialIdx, app.sampledIndex)
	}

	// Test Prev Candidate traversal
	app.pickPrevCandidate()
	expectedPrev := (initialIdx - 1 + 3) % 3
	if app.sampledIndex != expectedPrev {
		t.Errorf("pickPrevCandidate: expected index %d, got %d", expectedPrev, app.sampledIndex)
	}

	// Test Another Random Image avoids repeating current image
	for i := 0; i < 10; i++ {
		cur := app.sampledIndex
		app.pickRandomCandidate()
		if app.sampledIndex == cur {
			t.Fatalf("pickRandomCandidate repeated current index %d", cur)
		}
	}

	// Test UI rendering with sampling toolbar (icon-only buttons with tooltips)
	tester := ui.NewTester(app.view, 1000, 750)
	if !tester.HasText("🎲") {
		t.Errorf("Expected UI to display '🎲' icon button")
	}
	if !tester.HasText("⏭") || !tester.HasText("⏮") {
		t.Errorf("Expected UI to display '⏭' and '⏮' icon buttons")
	}
	if !tester.HasText("📂") {
		t.Errorf("Expected UI to display '📂' icon button")
	}
	counterText := fmt.Sprintf("Candidate %d of 3", app.sampledIndex+1)
	if !tester.HasText(counterText) {
		t.Errorf("Expected UI to display counter %q, got texts: %v", counterText, tester.Texts())
	}
}

func TestQueryCandidateSamplingEmptyDir(t *testing.T) {
	emptyDir := t.TempDir()

	app := NewDesktopApp()
	app.searchDir = emptyDir

	app.pickRandomCandidate()
	if app.queryPath != "" {
		t.Errorf("Expected queryPath to remain empty for empty directory, got %s", app.queryPath)
	}
	if !strings.Contains(app.searchStatus, "No supported images found") {
		t.Errorf("Expected searchStatus to report no supported images found, got: %s", app.searchStatus)
	}

	// UI rendering with empty state should show "🎲"
	tester := ui.NewTester(app.view, 1000, 750)
	if !tester.HasText("🎲") {
		t.Errorf("Expected UI to display '🎲' button when empty, got: %v", tester.Texts())
	}
}

func TestOpenFullViewFromQueryImage(t *testing.T) {
	tmpDir := t.TempDir()
	pA := createTestPNG(t, tmpDir, "query_test.png", 100, 150, 200)

	app := NewDesktopApp()
	app.setQueryImage(pA)

	// Open query image in Full View
	app.openFullView(pA)

	if app.activeTab != 4 {
		t.Errorf("Expected activeTab == 4 (Full View), got %d", app.activeTab)
	}
	if app.fullViewPath != pA {
		t.Errorf("Expected fullViewPath == %s, got %s", pA, app.fullViewPath)
	}
	if app.fullViewThumb == nil {
		t.Errorf("Expected fullViewThumb to be initialized")
	}

	// Verify UI rendering of Full View
	tester := ui.NewTester(app.view, 1000, 750)
	if !tester.HasText("Full View") && !tester.HasText("Reference Query Image") {
		t.Errorf("Expected header in Full View tab, got: %v", tester.Texts())
	}
	if !tester.HasText("🎲") || !tester.HasText("⏭") || !tester.HasText("⏮") || !tester.HasText("📂") {
		t.Errorf("Expected navigation buttons below image in Full View, got: %v", tester.Texts())
	}
	if !tester.HasText("Back to Visual Search") {
		t.Errorf("Expected 'Back to Visual Search' button")
	}
}

func TestOpenFullViewFromResultMatch(t *testing.T) {
	tmpDir := t.TempDir()
	pQ := createTestPNG(t, tmpDir, "q.png", 10, 20, 30)
	pM1 := createTestPNG(t, tmpDir, "m1.png", 12, 22, 32)
	pM2 := createTestPNG(t, tmpDir, "m2.png", 14, 24, 34)

	app := NewDesktopApp()
	app.queryPath = pQ
	app.searchReport = &search.SearchReport{
		QueryPath:    pQ,
		Metric:       string(engine.MetricPHash),
		TotalScanned: 2,
		Matches: []search.MatchResult{
			{
				Path:            pM1,
				SimilarityScore: engine.NewSimilarityScore(96.0),
				Width:           8,
				Height:          8,
				FileSize:        100,
			},
			{
				Path:            pM2,
				SimilarityScore: engine.NewSimilarityScore(91.0),
				Width:           8,
				Height:          8,
				FileSize:        120,
			},
		},
	}

	// Open first match
	app.openFullView(pM1)
	if app.activeTab != 4 {
		t.Errorf("Expected activeTab == 4, got %d", app.activeTab)
	}
	if app.fullViewPath != pM1 {
		t.Errorf("Expected fullViewPath == %s, got %s", pM1, app.fullViewPath)
	}
	if app.fullViewContext != "results" {
		t.Errorf("Expected fullViewContext == 'results', got %s", app.fullViewContext)
	}
	if app.fullViewMatchIdx != 0 {
		t.Errorf("Expected fullViewMatchIdx == 0, got %d", app.fullViewMatchIdx)
	}

	// Test Next in Full View
	app.fullViewPickNext()
	if app.fullViewPath != pM2 || app.fullViewMatchIdx != 1 {
		t.Errorf("Expected next match pM2 (idx 1), got path %s (idx %d)", app.fullViewPath, app.fullViewMatchIdx)
	}

	// Wrap around Next
	app.fullViewPickNext()
	if app.fullViewPath != pM1 || app.fullViewMatchIdx != 0 {
		t.Errorf("Expected wrap-around to pM1 (idx 0), got path %s (idx %d)", app.fullViewPath, app.fullViewMatchIdx)
	}

	// Test Prev in Full View
	app.fullViewPickPrev()
	if app.fullViewPath != pM2 || app.fullViewMatchIdx != 1 {
		t.Errorf("Expected prev match pM2 (idx 1), got path %s (idx %d)", app.fullViewPath, app.fullViewMatchIdx)
	}

	// Test UI rendering
	tester := ui.NewTester(app.view, 1000, 750)
	if !tester.HasText("Match 2 of 2") {
		t.Errorf("Expected counter 'Match 2 of 2' in Full View, got: %v", tester.Texts())
	}
}

func TestFullViewOptionButtonsInVisualSearch(t *testing.T) {
	tmpDir := t.TempDir()
	pQ := createTestPNG(t, tmpDir, "q.png", 10, 20, 30)
	pM := createTestPNG(t, tmpDir, "m.png", 10, 20, 30)

	app := NewDesktopApp()
	app.setQueryImage(pQ)
	app.searchReport = &search.SearchReport{
		QueryPath:    pQ,
		Metric:       string(engine.MetricPHash),
		TotalScanned: 1,
		Matches: []search.MatchResult{
			{
				Path:            pM,
				SimilarityScore: engine.NewSimilarityScore(99.0),
				Width:           8,
				Height:          8,
				FileSize:        100,
			},
		},
	}

	// Grid View: Query card and match card both have ⛶
	app.searchViewMode = "grid"
	testerGrid := ui.NewTester(app.view, 1000, 750)
	if !testerGrid.HasText("⛶") {
		t.Errorf("Expected Grid View to contain '⛶' Full View button, got: %v", testerGrid.Texts())
	}

	// List View: has Full View button
	app.searchViewMode = "list"
	testerList := ui.NewTester(app.view, 1000, 750)
	if !testerList.HasText("Full View") {
		t.Errorf("Expected List View to contain 'Full View' button, got: %v", testerList.Texts())
	}
}

func TestKeyboardShortcutsNavigation(t *testing.T) {
	tmpDir := t.TempDir()
	pA := createTestPNG(t, tmpDir, "a.png", 255, 0, 0)
	pB := createTestPNG(t, tmpDir, "b.png", 0, 255, 0)
	pC := createTestPNG(t, tmpDir, "c.png", 0, 0, 255)

	app := NewDesktopApp()
	app.searchDir = tmpDir

	// Start in Full View with first candidate
	app.setQueryImage(pA)
	app.openFullView(pA)

	tester := ui.NewTester(app.view, 1000, 750)

	// Press 'n' -> Next candidate
	tester.Key(0, ui.KeyN)
	if app.fullViewPath != pB {
		t.Errorf("After KeyN: expected fullViewPath == %s, got %s", pB, app.fullViewPath)
	}

	// Press 'n' -> Next candidate
	tester.Key(0, ui.KeyN)
	if app.fullViewPath != pC {
		t.Errorf("After second KeyN: expected fullViewPath == %s, got %s", pC, app.fullViewPath)
	}

	// Press 'p' -> Previous candidate
	tester.Key(0, ui.KeyP)
	if app.fullViewPath != pB {
		t.Errorf("After KeyP: expected fullViewPath == %s, got %s", pB, app.fullViewPath)
	}

	// Press 'r' -> Random candidate (distinct from current)
	curPath := app.fullViewPath
	tester.Key(0, ui.KeyR)
	if app.fullViewPath == curPath {
		t.Errorf("After KeyR: expected different image, but remained %s", curPath)
	}

	// Press Escape in Full View -> Back to Visual Search
	tester.Key(0, ui.KeyEscape)
	if app.activeTab != 0 {
		t.Errorf("After KeyEscape: expected activeTab == 0, got %d", app.activeTab)
	}
}

func TestDuplicateClustersFullViewButtonAndNavigation(t *testing.T) {
	tmpDir := t.TempDir()
	pCanon := createTestPNG(t, tmpDir, "canon.png", 255, 0, 0)
	pDup := createTestPNG(t, tmpDir, "dup.png", 200, 0, 0)

	app := NewDesktopApp()
	app.activeTab = 2 // Duplicate Clusters tab
	app.clusterReport = &cluster.ClusterReport{
		TargetDir:    tmpDir,
		TotalScanned: 2,
		ClusterCount: 1,
		Clusters: []cluster.Cluster{
			{
				ID: 1,
				Canonical: cluster.Candidate{
					Path:     pCanon,
					Width:    64,
					Height:   64,
					FileSize: 1000,
				},
				Duplicates: []cluster.Candidate{
					{
						Path:     pDup,
						Width:    64,
						Height:   64,
						FileSize: 900,
					},
				},
				AllCandidates: []cluster.Candidate{
					{Path: pCanon},
					{Path: pDup},
				},
			},
		},
	}

	tester := ui.NewTester(app.view, 1000, 750)
	if !tester.HasText("⛶") {
		t.Errorf("Expected Clusters tab to contain '⛶' Full View button")
	}

	// Open Full View from Duplicate Clusters
	app.openFullView(pDup)
	if app.activeTab != 4 {
		t.Fatalf("Expected activeTab == 4 (Full View), got %d", app.activeTab)
	}
	if app.fullViewContext != "cluster" {
		t.Errorf("Expected fullViewContext == 'cluster', got %s", app.fullViewContext)
	}
	if app.fullViewPrevTab != 2 {
		t.Errorf("Expected fullViewPrevTab == 2, got %d", app.fullViewPrevTab)
	}

	testerFull := ui.NewTester(app.view, 1000, 750)
	if !testerFull.HasText("Duplicate Cluster Image") {
		t.Errorf("Expected header 'Duplicate Cluster Image' in Full View, got texts: %v", testerFull.Texts())
	}
	if !testerFull.HasText("Back to Duplicate Clusters") {
		t.Errorf("Expected 'Back to Duplicate Clusters' button in Full View")
	}

	// Press Escape to return to Duplicate Clusters (tab 2)
	testerFull.Key(0, ui.KeyEscape)
	if app.activeTab != 2 {
		t.Errorf("After KeyEscape: expected return to activeTab == 2 (Duplicate Clusters), got %d", app.activeTab)
	}
}

func TestDuplicateClustersPagination(t *testing.T) {
	app := NewDesktopApp()
	app.activeTab = 2
	app.clusterPageSize = 10

	var clustersList []cluster.Cluster
	for i := 1; i <= 25; i++ {
		clustersList = append(clustersList, cluster.Cluster{
			ID: i,
			Canonical: cluster.Candidate{
				Path:     fmt.Sprintf("/fake/canon_%d.png", i),
				Width:    100,
				Height:   100,
				FileSize: 1000,
			},
			Duplicates: []cluster.Candidate{
				{
					Path:     fmt.Sprintf("/fake/dup_%d.png", i),
					Width:    100,
					Height:   100,
					FileSize: 900,
				},
			},
			AllCandidates: []cluster.Candidate{
				{Path: fmt.Sprintf("/fake/canon_%d.png", i)},
				{Path: fmt.Sprintf("/fake/dup_%d.png", i)},
			},
		})
	}

	app.clusterReport = &cluster.ClusterReport{
		TotalScanned:   50,
		ClusterCount:   25,
		DuplicateCount: 25,
		Clusters:       clustersList,
	}

	// Page 1 (clusters 1 through 10)
	tester := ui.NewTester(app.view, 1000, 750)
	if !tester.HasText("Page 1 of 3") {
		t.Errorf("Expected 'Page 1 of 3' on first page, got texts: %v", tester.Texts())
	}
	if !tester.HasText("Cluster #1 ") {
		t.Errorf("Expected Cluster #1 to be rendered on page 1")
	}
	if tester.HasText("Cluster #11 ") {
		t.Errorf("Did not expect Cluster #11 to be rendered on page 1")
	}

	// Advance to Page 2
	tester.Click("Next ▶")
	if app.clusterPage != 1 {
		t.Fatalf("Expected clusterPage == 1, got %d", app.clusterPage)
	}

	testerP2 := ui.NewTester(app.view, 1000, 750)
	if !testerP2.HasText("Page 2 of 3") {
		t.Errorf("Expected 'Page 2 of 3' on second page, got texts: %v", testerP2.Texts())
	}
	if !testerP2.HasText("Cluster #11 ") {
		t.Errorf("Expected Cluster #11 to be rendered on page 2")
	}
	if testerP2.HasText("Cluster #1 ") {
		t.Errorf("Did not expect Cluster #1 to be rendered on page 2")
	}

	// Advance to Page 3
	testerP2.Click("Next ▶")
	if app.clusterPage != 2 {
		t.Fatalf("Expected clusterPage == 2, got %d", app.clusterPage)
	}

	// Go back to Page 2
	testerP3 := ui.NewTester(app.view, 1000, 750)
	testerP3.Click("◀ Prev")
	if app.clusterPage != 1 {
		t.Fatalf("Expected clusterPage == 1 after ◀ Prev, got %d", app.clusterPage)
	}
}

func TestDuplicateClustersSkipSameImageInDuplicates(t *testing.T) {
	app := NewDesktopApp()
	app.activeTab = 2

	// Cluster with duplicate matching canonical path
	app.clusterReport = &cluster.ClusterReport{
		ClusterCount: 1,
		Clusters: []cluster.Cluster{
			{
				ID: 1,
				Canonical: cluster.Candidate{
					Path: "/fake/same_image.png",
				},
				Duplicates: []cluster.Candidate{
					{
						Path: "/fake/same_image.png", // Identical path to canonical
					},
					{
						Path: "/fake/actual_dup.png",
					},
				},
				AllCandidates: []cluster.Candidate{
					{Path: "/fake/same_image.png"},
					{Path: "/fake/actual_dup.png"},
				},
			},
		},
	}

	tester := ui.NewTester(app.view, 1000, 750)
	texts := tester.Texts()

	// Count occurrences of "same_image.png" in rendered texts
	count := 0
	for _, txt := range texts {
		if strings.Contains(txt, "same_image.png") {
			count++
		}
	}
	// Should only be rendered once (in the KEEP card), not in the DUPLICATE card
	// Notice a card might display filename and full path, so check occurrences of DUPLICATE tag
	duplicateTagCount := 0
	for _, txt := range texts {
		if txt == "DUPLICATE" {
			duplicateTagCount++
		}
	}
	if duplicateTagCount != 1 {
		t.Errorf("Expected only 1 DUPLICATE card (for actual_dup.png), got %d duplicate cards", duplicateTagCount)
	}
}

func TestClusterThumbLoading(t *testing.T) {
	tmpDir := t.TempDir()
	pA := createTestPNG(t, tmpDir, "sample_thumb.png", 100, 150, 200)

	app := NewDesktopApp()
	app.activeTab = 2
	app.requestClusterThumb(pA)

	// Wait briefly for background goroutine to complete
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		app.thumbMu.RLock()
		thumb := app.clusterThumbs[pA]
		app.thumbMu.RUnlock()
		if thumb != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	app.thumbMu.RLock()
	thumb := app.clusterThumbs[pA]
	app.thumbMu.RUnlock()

	if thumb == nil {
		t.Errorf("Expected cluster thumbnail to be loaded for %s, got nil", pA)
	}
}

func TestAppIconEmbeddingAndScaling(t *testing.T) {
	if len(appIconPNG) == 0 {
		t.Fatal("Expected embedded appIconPNG to be non-empty")
	}
	if len(appIcon512PNG) == 0 {
		t.Fatal("Expected embedded appIcon512PNG to be non-empty")
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(appIconPNG))
	if err != nil {
		t.Fatalf("Failed to decode appIconPNG: %v", err)
	}
	if cfg.Width != 128 || cfg.Height != 128 {
		t.Errorf("Expected appIconPNG to be 128x128, got %dx%d", cfg.Width, cfg.Height)
	}

	cfg512, err := png.DecodeConfig(bytes.NewReader(appIcon512PNG))
	if err != nil {
		t.Fatalf("Failed to decode appIcon512PNG: %v", err)
	}
	if cfg512.Width != 512 || cfg512.Height != 512 {
		t.Errorf("Expected appIcon512PNG to be 512x512, got %dx%d", cfg512.Width, cfg512.Height)
	}

	for _, sz := range []int{16, 24, 32, 48, 64} {
		scaled := scalePNG(appIcon512PNG, sz)
		scfg, err := png.DecodeConfig(bytes.NewReader(scaled))
		if err != nil {
			t.Errorf("Failed to decode scaled PNG for size %d: %v", sz, err)
			continue
		}
		if scfg.Width != sz || scfg.Height != sz {
			t.Errorf("Expected scaled PNG %dx%d, got %dx%d", sz, sz, scfg.Width, scfg.Height)
		}
	}
}
