# CLI Duplicate Clustering and Pure-Go Codec Ecosystem

We need headless deduplication workflows and portable cross-platform builds without CGO. We decided to expose Duplicate Clustering as a first-class CLI subcommand with safe dry-run semantics, and restrict image decoding to pure-Go decoders (JPEG, PNG, WebP, GIF, BMP, TIFF).

## Status

Accepted

## Context

While visual inspection is ideal for interactive deduplication in the desktop UI, automated scripting, scheduled server-side maintenance, and continuous integration require a command-line interface. Furthermore, requiring CGO or external C libraries (like libheif or libraw) breaks seamless cross-compilation across Linux, macOS, and Windows.

## Decision

1. **CLI Subcommand `cluster`**:
   `similar-images cluster [OPTIONS] <SEARCH_DIR>`:
   - Evaluates pairwise candidate similarity using Disjoint-Set union-find.
   - Automatically determines the canonical candidate to keep via `--heuristic` (`resolution`, `size`, `oldest`, `newest`).
   - Default mode is a non-destructive dry run reporting clustered duplicates and recommended actions.
   - Adding `--trash-duplicates` moves non-canonical candidate images to the OS native recycle bin / trash (with automatic cache invalidation).
   - `--json` outputs machine-readable cluster trees for pipeline automation.
2. **Pure-Go Decoders**:
   Use standard library (`image/jpeg`, `image/png`, `image/gif`) and `golang.org/x/image` (`webp`, `bmp`, `tiff`) exclusively, ensuring `CGO_ENABLED=0` builds cleanly for any target architecture.
