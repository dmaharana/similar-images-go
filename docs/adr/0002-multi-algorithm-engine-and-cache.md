# Multi-Algorithm Similarity Suite and Wide-Table Persistent Caching

We need a flexible image similarity suite that supports multiple perceptual and color metrics with persistent caching across repeated scans. We decided to implement five distinct similarity engines (`phash` as default, `pdq`, `wavelet`, `dhash`, `palette`) backed by a wide-column SQLite schema with atomic UPSERT and `COALESCE`.

## Status

Accepted

## Context

Different visual matching use cases demand different trade-offs:
- `phash` (Discrete Cosine Transform): Highly robust to JPEG compression, scaling, and contrast changes.
- `pdq` (256-bit Meta/ThreatExchange): Industry-standard perceptual hashing for high-accuracy near-duplicate tracking.
- `wavelet` (2D Haar DWT): Spatial-frequency multi-resolution fingerprinting.
- `dhash` (128-bit dual gradient): Rapid horizontal and vertical structural difference hashing.
- `palette` (64-bin 3D RGB histogram): Color palette distribution matching independent of geometry.

Repeated scans across thousands of images must avoid re-decoding images on disk.

## Decision

1. **Similarity Engine Interface**: A unified Go interface `SimilarityEngine[F any]` parameterized over fingerprint type `F`, providing `ComputeFingerprint`, `Similarity(a, b) SimilarityScore`, and serialization helpers.
2. **Default Metric**: Default to `phash` (pHash-DCT 64-bit), with `dhash` (aliased to `perceptual` for Rust compatibility), `pdq`, `wavelet`, and `palette` selectable via `--metric`.
3. **SQLite Schema**:
   A single `fingerprints` table storing metadata and individual BLOB fingerprints per algorithm:
   ```sql
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
   ```
   Updates use `ON CONFLICT(canonical_path) DO UPDATE SET ... COALESCE(...)` ensuring computing one metric never erases previously cached fingerprints from other metrics.
