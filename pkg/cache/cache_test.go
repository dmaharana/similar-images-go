package cache

import (
	"bytes"
	"testing"
)

func TestCacheCRUDAndInvalidation(t *testing.T) {
	c, err := OpenInMemory()
	if err != nil {
		t.Fatalf("failed to open in-memory cache: %v", err)
	}
	defer c.Close()

	pHashBytes := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	entry := &CacheEntry{
		CanonicalPath: "/path/to/img1.jpg",
		FileSize:      1024,
		Mtime:         1700000000,
		Width:         800,
		Height:        600,
		Format:        "jpeg",
		PHashHash:     pHashBytes,
	}

	// Insert
	if err := c.PutBatch([]*CacheEntry{entry}); err != nil {
		t.Fatalf("PutBatch failed: %v", err)
	}

	// Hit
	rec, err := c.Get("/path/to/img1.jpg", 1024, 1700000000)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if rec == nil {
		t.Fatalf("expected cache hit, got nil")
	}
	if !bytes.Equal(rec.PHashHash, pHashBytes) {
		t.Errorf("expected phash %v, got %v", pHashBytes, rec.PHashHash)
	}
	if rec.Width != 800 || rec.Height != 600 || rec.Format != "jpeg" {
		t.Errorf("metadata mismatch: %+v", rec)
	}

	// Miss on file size change
	recStaleSize, err := c.Get("/path/to/img1.jpg", 2048, 1700000000)
	if err != nil || recStaleSize != nil {
		t.Errorf("expected cache miss on file size mismatch, got %+v, err: %v", recStaleSize, err)
	}

	// Miss on mtime change
	recStaleMtime, err := c.Get("/path/to/img1.jpg", 1024, 1700000001)
	if err != nil || recStaleMtime != nil {
		t.Errorf("expected cache miss on mtime mismatch, got %+v, err: %v", recStaleMtime, err)
	}
}

func TestCacheCoalescePreservesOtherMetrics(t *testing.T) {
	c, err := OpenInMemory()
	if err != nil {
		t.Fatalf("failed to open in-memory cache: %v", err)
	}
	defer c.Close()

	// 1. First run computes pHash
	pHashBytes := []byte{1, 1, 1, 1, 1, 1, 1, 1}
	entry1 := &CacheEntry{
		CanonicalPath: "/path/to/img1.jpg",
		FileSize:      1024,
		Mtime:         1700000000,
		PHashHash:     pHashBytes,
	}
	if err := c.PutBatch([]*CacheEntry{entry1}); err != nil {
		t.Fatalf("PutBatch 1 failed: %v", err)
	}

	// 2. Second run computes palette histogram on same image
	paletteBytes := make([]byte, 256)
	paletteBytes[0] = 42
	entry2 := &CacheEntry{
		CanonicalPath:    "/path/to/img1.jpg",
		FileSize:         1024,
		Mtime:            1700000000,
		PaletteHistogram: paletteBytes,
	}
	if err := c.PutBatch([]*CacheEntry{entry2}); err != nil {
		t.Fatalf("PutBatch 2 failed: %v", err)
	}

	// Both metrics should be present!
	rec, err := c.Get("/path/to/img1.jpg", 1024, 1700000000)
	if err != nil || rec == nil {
		t.Fatalf("expected record, got nil or err: %v", err)
	}
	if !bytes.Equal(rec.PHashHash, pHashBytes) {
		t.Errorf("pHash was overwritten or lost: got %v", rec.PHashHash)
	}
	if !bytes.Equal(rec.PaletteHistogram, paletteBytes) {
		t.Errorf("palette histogram was lost: got %v", rec.PaletteHistogram)
	}
}

func TestCacheBatchAndInvalidate(t *testing.T) {
	c, err := OpenInMemory()
	if err != nil {
		t.Fatalf("failed to open in-memory cache: %v", err)
	}
	defer c.Close()

	entries := []*CacheEntry{
		{CanonicalPath: "/img1.jpg", FileSize: 100, Mtime: 1000, PHashHash: []byte{1}},
		{CanonicalPath: "/img2.jpg", FileSize: 200, Mtime: 2000, PHashHash: []byte{2}},
		{CanonicalPath: "/img3.jpg", FileSize: 300, Mtime: 3000, PHashHash: []byte{3}},
	}
	if err := c.PutBatch(entries); err != nil {
		t.Fatalf("PutBatch failed: %v", err)
	}

	paths := []string{"/img1.jpg", "/img2.jpg", "/img3.jpg", "/missing.jpg"}
	sizes := []int64{100, 999, 300, 500} // img2 has size mismatch (stale)
	mtimes := []int64{1000, 2000, 3000, 500}

	batch, err := c.GetBatch(paths, sizes, mtimes)
	if err != nil {
		t.Fatalf("GetBatch failed: %v", err)
	}

	if len(batch) != 2 {
		t.Fatalf("expected 2 valid hits (img1, img3), got %d", len(batch))
	}
	if batch["/img1.jpg"] == nil || batch["/img3.jpg"] == nil {
		t.Errorf("missing expected img1 or img3: %+v", batch)
	}
	if batch["/img2.jpg"] != nil {
		t.Errorf("img2 should have been invalidated due to size mismatch")
	}

	// Test Invalidate
	if err := c.Invalidate("/img1.jpg"); err != nil {
		t.Fatalf("Invalidate failed: %v", err)
	}
	rec, err := c.Get("/img1.jpg", 100, 1000)
	if err != nil || rec != nil {
		t.Errorf("expected nil after invalidate, got %+v", rec)
	}

	// Test Stats
	stats, err := c.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.TotalEntries != 2 {
		t.Errorf("expected 2 remaining entries, got %d", stats.TotalEntries)
	}
}
