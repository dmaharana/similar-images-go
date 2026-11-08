package cluster

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"similar-images/pkg/cache"
)

// MoveToTrash safely moves a file to the OS Trash/Recycle bin and invalidates its cache entry.
func MoveToTrash(filePath string, c *cache.Cache) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", absPath)
	}

	trashDir := getTrashDir()
	filesDir := filepath.Join(trashDir, "files")
	infoDir := filepath.Join(trashDir, "info")

	if err := os.MkdirAll(filesDir, 0700); err != nil {
		return fmt.Errorf("failed to create trash files directory: %w", err)
	}
	if err := os.MkdirAll(infoDir, 0700); err != nil {
		return fmt.Errorf("failed to create trash info directory: %w", err)
	}

	baseName := filepath.Base(absPath)
	destName := baseName
	destFile := filepath.Join(filesDir, destName)
	infoFile := filepath.Join(infoDir, destName+".trashinfo")

	// Ensure unique destination name if already in trash
	counter := 1
	for {
		if _, err := os.Stat(destFile); os.IsNotExist(err) {
			break
		}
		ext := filepath.Ext(baseName)
		rawName := strings.TrimSuffix(baseName, ext)
		destName = fmt.Sprintf("%s.%d%s", rawName, counter, ext)
		destFile = filepath.Join(filesDir, destName)
		infoFile = filepath.Join(infoDir, destName+".trashinfo")
		counter++
	}

	// Write .trashinfo metadata
	delTime := time.Now().Format("2006-01-02T15:04:05")
	trashInfoContent := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n", absPath, delTime)
	if err := os.WriteFile(infoFile, []byte(trashInfoContent), 0600); err != nil {
		return fmt.Errorf("failed to write trashinfo metadata: %w", err)
	}

	// Move file
	if err := moveFile(absPath, destFile); err != nil {
		_ = os.Remove(infoFile)
		return fmt.Errorf("failed to move file to trash: %w", err)
	}

	// Invalidate cache
	if c != nil {
		_ = c.Invalidate(absPath)
	}

	return nil
}

func getTrashDir() string {
	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData != "" {
		return filepath.Join(xdgData, "Trash")
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".local", "share", "Trash")
	}
	return filepath.Join(os.TempDir(), "Trash")
}

func moveFile(src, dst string) error {
	// Try atomic rename first (works within same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Cross-device link fallback (copy then delete)
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return os.Remove(src)
}
