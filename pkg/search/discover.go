package search

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var supportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
	".bmp":  true,
	".tif":  true,
	".tiff": true,
}

// IsSupportedImageExtension checks if the file path has a recognized image extension.
func IsSupportedImageExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return supportedExtensions[ext]
}

// DiscoverCandidatePaths finds all candidate image paths in the target directory.
func DiscoverCandidatePaths(dir string, recursive bool) ([]string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}

	var candidates []string

	if !recursive {
		entries, err := os.ReadDir(absDir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(absDir, entry.Name())
			if IsSupportedImageExtension(path) {
				candidates = append(candidates, path)
			}
		}
		return candidates, nil
	}

	err = filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip unreadable files or dirs with graceful continuation
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if IsSupportedImageExtension(path) {
			candidates = append(candidates, path)
		}
		return nil
	})

	return candidates, err
}
