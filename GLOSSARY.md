# Similar Image Search (Go)

A unified CLI and native desktop application for visual similarity search, duplicate detection, and image clustering across image collections.

## Language

**Query Image**:
The reference image provided by the user against which candidate images are evaluated.
_Avoid_: Source image, input image, probe

**Candidate Image**:
An image discovered within the search target directory evaluated for similarity.
_Avoid_: Target image, library image, gallery image

**Query Candidate Sampling**:
The interactive or automated selection of a candidate image discovered within the search target directory to serve as the active query image.
_Avoid_: Random picking, image cycling, sample query

**Fingerprint**:
A compact numerical representation extracted from an image used for rapid similarity comparison.
_Avoid_: Hash (when referring generally to fingerprints), signature, vector

**Metric**:
The algorithmic strategy used to extract fingerprints and calculate similarity scores.
_Avoid_: Method, algorithm, comparator

**pHash-DCT**:
A 64-bit perceptual fingerprint extracted via 2D Discrete Cosine Transform on low-frequency luminance components, highly robust to compression artifacts and minor geometric shifts.
_Avoid_: DCT hash, standard hash

**PDQ Hash**:
A 256-bit perceptual fingerprint optimized for industry-standard near-duplicate detection and transformations.
_Avoid_: Facebook hash, ThreatExchange vector

**Wavelet Hash**:
A multi-resolution frequency fingerprint extracted via 2D Discrete Haar Wavelet Transform (DWT), capturing localized structural details.
_Avoid_: Haar hash, DWT fingerprint

**dHash-2D**:
A 128-bit dual-gradient difference fingerprint capturing horizontal (64-bit) and vertical (64-bit) intensity gradients.
_Avoid_: Difference hash, gradient hash

**Palette Histogram**:
A 64-bin 3D quantized RGB color distribution vector used to match color palettes independent of spatial structure.
_Avoid_: Color vector, color histogram

**Similarity Score**:
A normalized representation (0.0% to 100.0%) of visual closeness between two images under a specific metric.
_Avoid_: Raw distance, difference score, hamming score

**Cutoff Threshold**:
The minimum similarity score a candidate image must reach to qualify as a match.
_Avoid_: Minimum score, floor, limit

**Match**:
A candidate image whose similarity to the query image satisfies the configured cutoff threshold.
_Avoid_: Duplicate, hit

**Fingerprint Cache**:
A persistent local SQLite database storing pre-calculated fingerprints associated with canonical file paths, file sizes, and modification timestamps.
_Avoid_: Hash database, index file, cache file

**Cache Invalidation**:
The process of recomputing or discarding a cached fingerprint when a candidate image's file size or modification time differs from the stored record.
_Avoid_: Cache purge, cache bust

**Duplicate Cluster**:
A collection of candidate images within a target directory that mutually satisfy the cutoff threshold, identified without requiring an external query image.
_Avoid_: Duplicate group, duplicate set, clone cluster

**Canonical Candidate**:
The designated candidate image in a duplicate cluster recommended for retention based on a configurable heuristic (e.g. highest resolution, largest file size, oldest/newest timestamp).
_Avoid_: Master image, primary image, original

**Canonical Selection Heuristic**:
The automated rule used to designate the canonical candidate within a duplicate cluster (`HighestResolution`, `LargestFileSize`, `OldestTimestamp`, `NewestTimestamp`).
_Avoid_: Selection rule, winner criteria, tiebreaker

**Dry Run**:
An execution mode that reports matches or planned duplicate trash operations without modifying files on disk or in the fingerprint cache.
_Avoid_: Simulation, trial run, test run

**Desktop Mode**:
Interactive graphical user interface mode rendered natively using Mygo (`mygo/ui`), activated via `--desktop` or when launched without CLI arguments in a desktop session.
_Avoid_: GUI mode, web mode, window mode

**CLI Mode**:
Headless command-line execution mode providing terminal progress reporting and structured JSON output for scripts and pipelines.
_Avoid_: Terminal mode, headless mode, console mode
