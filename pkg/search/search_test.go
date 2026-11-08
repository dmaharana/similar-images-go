package search

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"similar-images/pkg/cache"
	"similar-images/pkg/engine"
)

func createTestPNG(path string, w, h int, clr color.Color) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, clr)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func TestSearchDirectory(t *testing.T) {
	tempDir := t.TempDir()

	queryPath := filepath.Join(tempDir, "query.png")
	matchPath := filepath.Join(tempDir, "match.png")
	diffPath := filepath.Join(tempDir, "different.png")

	// Create identical match (red) and different image (blue)
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}

	if err := createTestPNG(queryPath, 64, 64, red); err != nil {
		t.Fatalf("failed to create query: %v", err)
	}
	if err := createTestPNG(matchPath, 64, 64, red); err != nil {
		t.Fatalf("failed to create match: %v", err)
	}
	if err := createTestPNG(diffPath, 64, 64, blue); err != nil {
		t.Fatalf("failed to create diff: %v", err)
	}

	queryImg, _, err := OpenImage(queryPath)
	if err != nil {
		t.Fatalf("failed to open query: %v", err)
	}

	inMemCache, err := cache.OpenInMemory()
	if err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	defer inMemCache.Close()

	opts := SearchOptions{
		SearchDir:   tempDir,
		Metric:      engine.MetricPHash,
		Threshold:   80.0,
		TopK:        10,
		Recursive:   false,
		UseCache:    true,
		Concurrency: 2,
	}

	var progressCalls int
	report, err := SearchDirectory(context.Background(), queryImg, queryPath, opts, inMemCache, func(scanned, total int) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("SearchDirectory failed: %v", err)
	}

	if report.TotalScanned < 3 {
		t.Errorf("expected at least 3 scanned, got %d", report.TotalScanned)
	}
	if len(report.Matches) == 0 {
		t.Fatalf("expected at least 1 match, got 0")
	}

	// Verify top match is red image
	topMatch := report.Matches[0]
	if topMatch.ScorePercent < 99.0 {
		t.Errorf("expected 100%% similarity on identical image, got %f", topMatch.ScorePercent)
	}

	// Verify caching occurred
	stats, err := inMemCache.Stats()
	if err != nil || stats.TotalEntries == 0 {
		t.Errorf("expected fingerprints to be cached, stats: %+v, err: %v", stats, err)
	}
}
