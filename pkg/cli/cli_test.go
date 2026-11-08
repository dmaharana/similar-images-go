package cli

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestCliHelpAndVersion(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	// Help
	if err := Run([]string{"--help"}, &out, &errOut, nil); err != nil {
		t.Fatalf("help failed: %v", err)
	}
	if !strings.Contains(out.String(), "similar-images - Fast visual similarity search") {
		t.Errorf("expected usage output, got: %s", out.String())
	}

	// Version
	out.Reset()
	if err := Run([]string{"--version"}, &out, &errOut, nil); err != nil {
		t.Fatalf("version failed: %v", err)
	}
	if !strings.Contains(out.String(), "similar-images version") {
		t.Errorf("expected version output, got: %s", out.String())
	}
}

func TestCliSearchAndClusterCommands(t *testing.T) {
	tempDir := t.TempDir()

	img1 := filepath.Join(tempDir, "img1.png")
	img2 := filepath.Join(tempDir, "img2.png")
	red := color.RGBA{R: 250, G: 0, B: 0, A: 255}

	createTestPNG(img1, 64, 64, red)
	createTestPNG(img2, 64, 64, red)

	var out bytes.Buffer
	var errOut bytes.Buffer

	// Test Search
	argsSearch := []string{"--no-cache", img1, tempDir}
	if err := Run(argsSearch, &out, &errOut, nil); err != nil {
		t.Fatalf("search run failed: %v", err)
	}
	if !strings.Contains(out.String(), "Visual Similarity Search Report") {
		t.Errorf("expected search report, got: %s", out.String())
	}

	// Test Cluster
	out.Reset()
	errOut.Reset()
	argsCluster := []string{"cluster", "--no-cache", "--json", tempDir}
	if err := Run(argsCluster, &out, &errOut, nil); err != nil {
		t.Fatalf("cluster run failed: %v", err)
	}
	if !strings.Contains(out.String(), `"cluster_count"`) {
		t.Errorf("expected JSON cluster output, got: %s", out.String())
	}
}
