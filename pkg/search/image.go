package search

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"similar-images/pkg/engine"
)

// ImageMetadata captures file metadata and decoded dimensions.
type ImageMetadata struct {
	CanonicalPath string
	Width         int
	Height        int
	Format        string
	FileSize      int64
	Mtime         int64
}

// OpenImage reads and decodes an image file from disk along with its metadata.
func OpenImage(path string) (image.Image, ImageMetadata, error) {
	var meta ImageMetadata

	fi, err := os.Stat(path)
	if err != nil {
		return nil, meta, fmt.Errorf("failed to stat image file %s: %w", path, err)
	}

	meta.CanonicalPath = path
	meta.FileSize = fi.Size()
	meta.Mtime = fi.ModTime().Unix()

	f, err := os.Open(path)
	if err != nil {
		return nil, meta, fmt.Errorf("failed to open image file %s: %w", path, err)
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, meta, fmt.Errorf("failed to decode image %s: %w", path, err)
	}

	bounds := img.Bounds()
	meta.Width = bounds.Dx()
	meta.Height = bounds.Dy()
	meta.Format = format

	return img, meta, nil
}

// CreateThumbnail produces a downsampled RGBA thumbnail bounded by maxDim.
func CreateThumbnail(img image.Image, maxDim int) *image.RGBA {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	targetW := w
	targetH := h
	if w > maxDim || h > maxDim {
		if w >= h {
			targetW = maxDim
			targetH = h * maxDim / w
			if targetH < 1 {
				targetH = 1
			}
		} else {
			targetH = maxDim
			targetW = w * maxDim / h
			if targetW < 1 {
				targetW = 1
			}
		}
	}
	rgb := engine.ResizeRGB(img, targetW, targetH)
	thumb := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			idx := (y*targetW + x) * 3
			thumb.SetRGBA(x, y, color.RGBA{R: rgb[idx], G: rgb[idx+1], B: rgb[idx+2], A: 255})
		}
	}
	return thumb
}

