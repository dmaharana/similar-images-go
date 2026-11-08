# Unified CLI and Pure Go Native Desktop Application via Mygo

We need a single binary that provides both a fast headless CLI and an interactive desktop graphical interface for visual similarity search and duplicate clustering. We decided to use Mygo (`github.com/egoist/mygo/ui`) to build a 100% pure Go native GPU-rendered desktop interface directly within the executable, replacing web toolchains (React, Node, Vite) and external browser engines.

## Status

Accepted

## Context

The previous Rust implementation used two executables (`similar-image` and `similar-image-desktop`) where the desktop interface embedded a React Vite web build and served it via an internal Axum HTTP server to an external browser. For the Go implementation, we require a single binary, zero npm/web dependencies, minimal memory footprint, and seamless headless CLI usage on headless servers without X11/Wayland display errors.

## Decision

1. **Pure Go Native UI**: Implement the desktop application using Mygo's native immediate UI engine (`github.com/egoist/mygo/ui`). The UI renders directly to platform GPU APIs (OpenGL on Linux, Direct3D on Windows, Metal on macOS) without webviews or HTML/JS.
2. **Lazy Initialization**: Mygo and GPU subsystems are initialized lazily only when entering Desktop Mode (`--desktop` or `similar-images desktop`). In standard CLI mode, no display or GPU contexts are initialized, allowing headless execution in Docker, SSH, and CI.
3. **Execution Modes**:
   - `similar-images [flags] <QUERY_IMAGE> <SEARCH_DIR>`: Runs standard CLI Mode.
   - `similar-images --desktop` or `similar-images desktop`: Runs Desktop Mode.
   - Running with no arguments in an interactive desktop session opens Desktop Mode; in a headless/terminal session it displays CLI usage help.
