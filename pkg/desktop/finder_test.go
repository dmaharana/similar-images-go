package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShowInFileFinderEmptyPath(t *testing.T) {
	err := ShowInFileFinder("")
	if err == nil {
		t.Fatalf("expected error for empty path, got nil")
	}
}

func TestShowInFileFinderNonexistent(t *testing.T) {
	err := ShowInFileFinder("/nonexistent_dir_12345/nonexistent_file_67890.jpg")
	if err == nil {
		t.Fatalf("expected error for completely nonexistent path, got nil")
	}
}

func TestShowInFileFinderMock(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_image.png")
	if err := os.WriteFile(tmpFile, []byte("fake image data"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	var calledTarget string
	origCmd := openFinderCmd
	defer func() { openFinderCmd = origCmd }()

	openFinderCmd = func(target string) error {
		calledTarget = target
		return nil
	}

	err := ShowInFileFinder(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	absTmp, _ := filepath.Abs(tmpFile)
	if calledTarget != absTmp {
		t.Errorf("expected target %q, got %q", absTmp, calledTarget)
	}
}

func TestShowInFileFinderFallbackToDir(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "deleted_image.png") // does not exist, but tmpDir does

	var calledTarget string
	origCmd := openFinderCmd
	defer func() { openFinderCmd = origCmd }()

	openFinderCmd = func(target string) error {
		calledTarget = target
		return nil
	}

	err := ShowInFileFinder(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	absDir, _ := filepath.Abs(tmpDir)
	if calledTarget != absDir {
		t.Errorf("expected target %q, got %q", absDir, calledTarget)
	}
}
