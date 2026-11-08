package cache

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// CachedRecord contains stored fingerprints and metadata for a candidate image.
type CachedRecord struct {
	CanonicalPath    string
	FileSize         int64
	Mtime            int64
	Width            int
	Height           int
	Format           string
	PHashHash        []byte
	PDQHash          []byte
	WaveletHash      []byte
	DHashHash        []byte
	PaletteHistogram []byte
}

// CacheEntry represents an entry to store or update in the cache.
type CacheEntry struct {
	CanonicalPath    string
	FileSize         int64
	Mtime            int64
	Width            int
	Height           int
	Format           string
	PHashHash        []byte
	PDQHash          []byte
	WaveletHash      []byte
	DHashHash        []byte
	PaletteHistogram []byte
}

// CacheStats provides information about the persistent SQLite database.
type CacheStats struct {
	TotalEntries int    `json:"total_entries"`
	DBSizeBytes  int64  `json:"db_size_bytes"`
	DBPath       string `json:"db_path"`
}

// Cache manages persistent fingerprint caching with SQLite WAL mode.
type Cache struct {
	db     *sql.DB
	dbPath string
}

// DefaultCachePath returns the user cache location ~/.cache/similar-images/cache.db.
func DefaultCachePath() string {
	userCache, err := os.UserCacheDir()
	if err != nil || userCache == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			userCache = filepath.Join(home, ".cache")
		} else {
			userCache = "."
		}
	}
	return filepath.Join(userCache, "similar-images", "cache.db")
}

// Open opens or initializes a SQLite cache at the given path.
func Open(path string) (*Cache, error) {
	if path == ":memory:" {
		return OpenInMemory()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory %s: %w", dir, err)
	}

	// modernc.org/sqlite accepts DSN with pragma options
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	c := &Cache{
		db:     db,
		dbPath: path,
	}

	if err := c.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return c, nil
}

// OpenInMemory opens an in-memory SQLite cache, ideal for testing.
func OpenInMemory() (*Cache, error) {
	db, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open in-memory sqlite database: %w", err)
	}

	c := &Cache{
		db:     db,
		dbPath: ":memory:",
	}

	if err := c.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return c, nil
}

func (c *Cache) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS fingerprints (
		canonical_path TEXT PRIMARY KEY,
		file_size INTEGER NOT NULL,
		mtime INTEGER NOT NULL,
		width INTEGER,
		height INTEGER,
		format TEXT,
		phash_hash BLOB,
		pdq_hash BLOB,
		wavelet_hash BLOB,
		dhash_hash BLOB,
		palette_histogram BLOB
	);
	CREATE INDEX IF NOT EXISTS idx_fingerprints_mtime ON fingerprints(mtime);
	`
	_, err := c.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (c *Cache) Close() error {
	return c.db.Close()
}

// Get looks up a cached record, returning nil if missing or if file size/mtime differ (cache invalidation).
func (c *Cache) Get(canonicalPath string, fileSize int64, mtime int64) (*CachedRecord, error) {
	query := `
	SELECT canonical_path, file_size, mtime, width, height, format,
	       phash_hash, pdq_hash, wavelet_hash, dhash_hash, palette_histogram
	FROM fingerprints
	WHERE canonical_path = ?;
	`
	row := c.db.QueryRow(query, canonicalPath)
	rec, err := scanRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Cache invalidation check
	if rec.FileSize != fileSize || rec.Mtime != mtime {
		return nil, nil // Stale cache entry
	}

	return rec, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(s rowScanner) (*CachedRecord, error) {
	var (
		rec             CachedRecord
		width           sql.NullInt32
		height          sql.NullInt32
		format          sql.NullString
		phashHash       []byte
		pdqHash         []byte
		waveletHash     []byte
		dhashHash       []byte
		paletteHist     []byte
	)

	err := s.Scan(
		&rec.CanonicalPath,
		&rec.FileSize,
		&rec.Mtime,
		&width,
		&height,
		&format,
		&phashHash,
		&pdqHash,
		&waveletHash,
		&dhashHash,
		&paletteHist,
	)
	if err != nil {
		return nil, err
	}

	if width.Valid {
		rec.Width = int(width.Int32)
	}
	if height.Valid {
		rec.Height = int(height.Int32)
	}
	if format.Valid {
		rec.Format = format.String
	}
	rec.PHashHash = phashHash
	rec.PDQHash = pdqHash
	rec.WaveletHash = waveletHash
	rec.DHashHash = dhashHash
	rec.PaletteHistogram = paletteHist

	return &rec, nil
}

// GetBatch retrieves cached records for multiple files. Invalidated entries are excluded.
func (c *Cache) GetBatch(paths []string, fileSizes []int64, mtimes []int64) (map[string]*CachedRecord, error) {
	result := make(map[string]*CachedRecord, len(paths))
	if len(paths) == 0 {
		return result, nil
	}

	// SQLite parameter limit workaround: chunk by 200
	const chunkSize = 200
	for i := 0; i < len(paths); i += chunkSize {
		end := i + chunkSize
		if end > len(paths) {
			end = len(paths)
		}

		chunkPaths := paths[i:end]
		placeholders := strings.Repeat("?,", len(chunkPaths))
		placeholders = placeholders[:len(placeholders)-1]

		args := make([]any, len(chunkPaths))
		for idx, p := range chunkPaths {
			args[idx] = p
		}

		query := fmt.Sprintf(`
		SELECT canonical_path, file_size, mtime, width, height, format,
		       phash_hash, pdq_hash, wavelet_hash, dhash_hash, palette_histogram
		FROM fingerprints
		WHERE canonical_path IN (%s);
		`, placeholders)

		rows, err := c.db.Query(query, args...)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			rec, err := scanRecord(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			result[rec.CanonicalPath] = rec
		}
		rows.Close()
	}

	// Validate against fileSize and mtime
	valid := make(map[string]*CachedRecord, len(result))
	for idx, p := range paths {
		if rec, exists := result[p]; exists {
			if rec.FileSize == fileSizes[idx] && rec.Mtime == mtimes[idx] {
				valid[p] = rec
			}
		}
	}

	return valid, nil
}

// PutBatch writes or updates cache entries in a single atomic SQLite transaction using UPSERT with COALESCE.
func (c *Cache) PutBatch(entries []*CacheEntry) error {
	if len(entries) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
	INSERT INTO fingerprints (
		canonical_path, file_size, mtime, width, height, format,
		phash_hash, pdq_hash, wavelet_hash, dhash_hash, palette_histogram
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(canonical_path) DO UPDATE SET
		file_size = excluded.file_size,
		mtime = excluded.mtime,
		width = COALESCE(excluded.width, fingerprints.width),
		height = COALESCE(excluded.height, fingerprints.height),
		format = COALESCE(excluded.format, fingerprints.format),
		phash_hash = COALESCE(excluded.phash_hash, fingerprints.phash_hash),
		pdq_hash = COALESCE(excluded.pdq_hash, fingerprints.pdq_hash),
		wavelet_hash = COALESCE(excluded.wavelet_hash, fingerprints.wavelet_hash),
		dhash_hash = COALESCE(excluded.dhash_hash, fingerprints.dhash_hash),
		palette_histogram = COALESCE(excluded.palette_histogram, fingerprints.palette_histogram);
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare upsert: %w", err)
	}
	defer stmt.Close()

	for _, e := range entries {
		var width, height any
		if e.Width > 0 {
			width = e.Width
		}
		if e.Height > 0 {
			height = e.Height
		}
		var format any
		if e.Format != "" {
			format = e.Format
		}

		_, err := stmt.ExecContext(ctx,
			e.CanonicalPath,
			e.FileSize,
			e.Mtime,
			width,
			height,
			format,
			e.PHashHash,
			e.PDQHash,
			e.WaveletHash,
			e.DHashHash,
			e.PaletteHistogram,
		)
		if err != nil {
			return fmt.Errorf("failed to insert entry %s: %w", e.CanonicalPath, err)
		}
	}

	return tx.Commit()
}

// Invalidate removes a file's fingerprint from the cache.
func (c *Cache) Invalidate(canonicalPath string) error {
	_, err := c.db.Exec("DELETE FROM fingerprints WHERE canonical_path = ?;", canonicalPath)
	return err
}

// Stats returns statistics regarding cache size and row count.
func (c *Cache) Stats() (CacheStats, error) {
	stats := CacheStats{DBPath: c.dbPath}
	var count int
	err := c.db.QueryRow("SELECT COUNT(*) FROM fingerprints;").Scan(&count)
	if err != nil {
		return stats, err
	}
	stats.TotalEntries = count

	if c.dbPath != ":memory:" {
		fi, err := os.Stat(c.dbPath)
		if err == nil {
			stats.DBSizeBytes = fi.Size()
		}
	}

	return stats, nil
}

// Vacuum reclaims disk space in the SQLite database.
func (c *Cache) Vacuum() error {
	_, err := c.db.Exec("VACUUM;")
	return err
}
