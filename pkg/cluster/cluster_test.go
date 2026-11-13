package cluster

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

func createPatternPNG(path string, w, h int, pattern string) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch pattern {
			case "horizontal":
				if (y/8)%2 == 0 {
					img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
				} else {
					img.Set(x, y, color.RGBA{R: 10, G: 10, B: 10, A: 255})
				}
			case "vertical":
				if (x/8)%2 == 0 {
					img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
				} else {
					img.Set(x, y, color.RGBA{R: 10, G: 10, B: 10, A: 255})
				}
			case "diagonal":
				if ((x+y)/8)%2 == 0 {
					img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
				} else {
					img.Set(x, y, color.RGBA{R: 10, G: 10, B: 10, A: 255})
				}
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func TestDisjointSet(t *testing.T) {
	ds := NewDisjointSet(5)
	if ds.Find(0) == ds.Find(1) {
		t.Errorf("elements should initially be separate")
	}

	ds.Union(0, 1)
	ds.Union(1, 2)
	if ds.Find(0) != ds.Find(2) {
		t.Errorf("0 and 2 should be in same set after union")
	}

	ds.Union(3, 4)
	if ds.Find(0) == ds.Find(3) {
		t.Errorf("0 and 3 should be in different sets")
	}

	ds.Union(2, 3)
	if ds.Find(0) != ds.Find(4) {
		t.Errorf("all elements should now be connected")
	}
}

func TestPickCanonicalHeuristics(t *testing.T) {
	candidates := []Candidate{
		{Path: "low_res_large_file.png", Width: 100, Height: 100, FileSize: 5000, Mtime: 1000},
		{Path: "high_res_small_file.png", Width: 800, Height: 600, FileSize: 2000, Mtime: 2000},
		{Path: "oldest.png", Width: 200, Height: 200, FileSize: 1000, Mtime: 500},
	}

	// Highest resolution
	idxRes := PickCanonical(candidates, HeuristicHighestResolution)
	if idxRes != 1 {
		t.Errorf("expected candidate 1 (high res), got %d", idxRes)
	}

	// Largest file size
	idxSize := PickCanonical(candidates, HeuristicLargestFileSize)
	if idxSize != 0 {
		t.Errorf("expected candidate 0 (large file), got %d", idxSize)
	}

	// Oldest timestamp
	idxOldest := PickCanonical(candidates, HeuristicOldestTimestamp)
	if idxOldest != 2 {
		t.Errorf("expected candidate 2 (oldest), got %d", idxOldest)
	}

	// Newest timestamp
	idxNewest := PickCanonical(candidates, HeuristicNewestTimestamp)
	if idxNewest != 1 {
		t.Errorf("expected candidate 1 (newest), got %d", idxNewest)
	}
}

func TestFindDuplicateClusters(t *testing.T) {
	tempDir := t.TempDir()

	h1 := filepath.Join(tempDir, "h1.png")
	h2 := filepath.Join(tempDir, "h2.png")
	v1 := filepath.Join(tempDir, "v1.png")
	v2 := filepath.Join(tempDir, "v2.png")
	diag := filepath.Join(tempDir, "diag_unique.png")

	// Horizontal stripes cluster (h2 has higher resolution)
	createPatternPNG(h1, 64, 64, "horizontal")
	createPatternPNG(h2, 128, 128, "horizontal")

	// Vertical stripes cluster (v1 and v2)
	createPatternPNG(v1, 64, 64, "vertical")
	createPatternPNG(v2, 64, 64, "vertical")

	// Unique diagonal
	createPatternPNG(diag, 64, 64, "diagonal")

	inMemCache, _ := cache.OpenInMemory()
	defer inMemCache.Close()

	report, err := FindDuplicateClusters(
		context.Background(),
		tempDir,
		engine.MetricPHash,
		80.0,
		HeuristicHighestResolution,
		false,
		inMemCache,
		nil,
	)
	if err != nil {
		t.Fatalf("FindDuplicateClusters failed: %v", err)
	}

	if report.ClusterCount != 2 {
		t.Fatalf("expected 2 duplicate clusters, got %d", report.ClusterCount)
	}
	if report.DuplicateCount != 2 {
		t.Fatalf("expected 2 duplicate images across clusters, got %d", report.DuplicateCount)
	}
}

func TestMoveToTrash(t *testing.T) {
	tempDir := t.TempDir()
	sampleFile := filepath.Join(tempDir, "sample_to_trash.png")
	if err := os.WriteFile(sampleFile, []byte("fake png content"), 0644); err != nil {
		t.Fatalf("failed to create sample file: %v", err)
	}

	inMemCache, _ := cache.OpenInMemory()
	defer inMemCache.Close()

	if err := MoveToTrash(sampleFile, inMemCache); err != nil {
		t.Fatalf("MoveToTrash failed: %v", err)
	}

	// Verify file is removed from original location
	if _, err := os.Stat(sampleFile); !os.IsNotExist(err) {
		t.Errorf("file should not exist at original location after trash")
	}
}

func TestFindDuplicateClusters_NoSameImageDuplicates(t *testing.T) {
	tempDir := t.TempDir()

	img1 := filepath.Join(tempDir, "img1.png")
	createPatternPNG(img1, 64, 64, "horizontal")

	// Create a symlink to img1
	symlink := filepath.Join(tempDir, "img1_symlink.png")
	if err := os.Symlink(img1, symlink); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	inMemCache, _ := cache.OpenInMemory()
	defer inMemCache.Close()

	report, err := FindDuplicateClusters(
		context.Background(),
		tempDir,
		engine.MetricPHash,
		80.0,
		HeuristicHighestResolution,
		false,
		inMemCache,
		nil,
	)
	if err != nil {
		t.Fatalf("FindDuplicateClusters failed: %v", err)
	}

	// Because img1 and img1_symlink point to the exact same file,
	// there should be 0 duplicate clusters (no fake duplicate of the same image).
	if report.ClusterCount != 0 {
		t.Errorf("expected 0 duplicate clusters when only 1 physical image exists with symlink, got %d", report.ClusterCount)
		for _, cl := range report.Clusters {
			t.Logf("Cluster: Canonical=%s, Duplicates=%+v", cl.Canonical.Path, cl.Duplicates)
			for _, dup := range cl.Duplicates {
				if dup.Path == cl.Canonical.Path {
					t.Errorf("found duplicate identical to canonical: %s", dup.Path)
				}
			}
		}
	}
}
