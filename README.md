# similar-images

A fast, multithreaded command-line and GPU-native desktop application written in pure Go to search directories for visually similar images and detect duplicate clusters.

Built with a unified multi-algorithm similarity suite supporting structural perceptual hashing, frequency transforms, and color palette distribution matching.

---

## Features

- **Single Embedded Binary (CLI + Desktop)**:
  - Unified executable: runs standard headless CLI commands or launches the GPU-accelerated desktop interface via `--desktop` or `similar-images desktop`.
  - Zero web runtimes or node_modules: the Desktop UI is written 100% in pure Go with [Mygo](https://mygo.egoist.dev/) (`mygo/ui`), directly rendering on the GPU (OpenGL / Metal / Direct3D).
  - Four desktop tabs:
    1. **1-to-N Search**: Select Query Image, scan candidate directory, inspect matches in a gallery with similarity badges.
    2. **Comparison View**: Zoomable side-by-side inspection layout displaying Query Image alongside candidate matches with score metrics.
    3. **Duplicate Clusters**: Automatically partition target directory images into clusters using Disjoint-Set union-find, with automated canonical keep rules and single-click or batch trashing.
    4. **Cache Management**: Inspect cache entry counts, database disk size, vacuum SQLite database, or proactively pre-index directories.
- **5 Similarity Engines**:
  - **`phash` (pHash-DCT, default)**: 64-bit 2D Discrete Cosine Transform on low-frequency luminance components. Highly robust against compression artifacts, scaling, and contrast shifts.
  - **`pdq` (PDQ Hash)**: 256-bit perceptual hash (Meta / ThreatExchange standard) optimized for high accuracy near-duplicate tracking across complex transforms.
  - **`wavelet` (Wavelet Hash)**: 64-bit 2D Discrete Haar Wavelet Transform (DWT), capturing multi-resolution spatial-frequency structure.
  - **`dhash` (dHash-2D / perceptual)**: 128-bit dual-gradient difference hashing capturing horizontal and vertical intensity gradients.
  - **`palette` (Color Palette)**: 64-bin 3D RGB quantized color histogram intersection for color distribution matching independent of content geometry.
- **Persistent SQLite Cache (`modernc.org/sqlite`)**:
  - Embedded SQLite database at `~/.cache/similar-images/cache.db` operating in WAL mode.
  - Validates cached fingerprints against `(file_size, mtime)` for instantaneous invalidation.
  - Atomic UPSERT with `COALESCE` preserves multiple metric fingerprints for the same image across separate runs.
  - 100% pure Go with `CGO_ENABLED=0` for effortless cross-compilation across Linux, macOS, and Windows.
- **Duplicate Clustering (`similar-images cluster`)**:
  - First-class CLI and Desktop clustering powered by Disjoint-Set union-find.
  - Canonical selection heuristics: `resolution` (default), `size`, `oldest`, `newest`.
  - Native OS Recycle Bin / Trash integration with automatic cache invalidation and dry-run safety by default.
- **Pure-Go Format Support**: JPEG, PNG, WebP, GIF, BMP, and TIFF.

---

## Installation & Build

Requires Go 1.27+:

```bash
# Build using Makefile (pure Go, CGO_ENABLED=0)
make build

# Or directly with go
CGO_ENABLED=0 go build -ldflags="-s -w" -o similar-images .

# Run test suite
make test

# Launch desktop GUI
make desktop
```

---

## Usage

### 1. Visual Similarity Search (1-to-N)

Find candidate images matching a reference query image:

```bash
# Default search (pHash-DCT, threshold >= 80%, top 10)
similar-images ./ref.jpg ~/Pictures

# Select a different metric (pdq, wavelet, dhash, or palette)
similar-images --metric pdq --threshold 85 ./ref.jpg ~/Pictures

# Output machine-readable JSON
similar-images --json ./ref.jpg ~/Pictures
```

### 2. Duplicate Clustering

Detect groups of near-duplicate images in a directory:

```bash
# Dry run report (does not modify files)
similar-images cluster ~/Pictures/Photos

# Cluster and move non-canonical duplicates to OS Trash
similar-images cluster --trash-duplicates ~/Pictures/Photos

# Specify canonical retention rule (resolution, size, oldest, newest)
similar-images cluster --heuristic size ~/Pictures/Photos
```

### 3. Desktop Application

Launch the Mygo native desktop graphical interface:

```bash
similar-images desktop
# or
similar-images --desktop
```

### 4. Cache Utilities

```bash
# Display cache statistics and disk usage
similar-images cache stats

# Proactively pre-index candidate image fingerprints
similar-images cache preindex ~/Pictures

# Vacuum and reclaim SQLite database space
similar-images cache vacuum
```

---

## Architecture & Domain Model

- [`GLOSSARY.md`](./GLOSSARY.md): Ubiquitous domain terminology.
- [`docs/adr/0001-mygo-native-ui-single-binary.md`](./docs/adr/0001-mygo-native-ui-single-binary.md): Pure Go Mygo desktop architecture.
- [`docs/adr/0002-multi-algorithm-engine-and-cache.md`](./docs/adr/0002-multi-algorithm-engine-and-cache.md): Multi-algorithm engine suite & SQLite schema.
- [`docs/adr/0003-cli-clustering-and-pure-go-codecs.md`](./docs/adr/0003-cli-clustering-and-pure-go-codecs.md): CLI duplicate clustering & pure-Go codecs.
