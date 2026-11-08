package search

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
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
