# godisplacementx

A Go port of [Displacement X](https://displacementx.pages.dev/) — a procedural
generator of grayscale **displacement / height maps** (a sci-fi "JSplacement"
aesthetic) for 3D software such as Blender, Cinema4D and Octane.

It ships as a **single executable** that is both:

- a **CLI** for headless / scripted generation, and
- a **cross-platform GUI** (Windows / macOS / Linux) built with
  [Wails](https://wails.io/), whose UI is a static HTML + htmx port of the
  original.

The image-generation engine is pure Go (`internal/gen`) and is shared by both
front-ends, so the CLI and GUI produce identical results.

## Features

- All six drawing primitives: rectangles, grid, columns, rows, lines, sprites.
- All 16 canvas composition (blend) modes, implemented as a software compositor.
- Seamless / tileable texture mode.
- Sprite packs (`classic`, `bigdata`, `aggromaxx`, `crappack`), pre-rasterized
  from the original SVGs and embedded in the binary.
- Post-processing: **normal map**, **color map** (via a custom gradient), and
  **invert**.
- Reproducible output via `--seed` (the original used an unseeded RNG).
- Resolutions 1024 / 2048 / 4096 / 8192 (the CLI also accepts arbitrary sizes).

## Building

The build system is a `Makefile` (recipes are POSIX sh — on Windows run them
from Git Bash or WSL with `make` installed, e.g. `choco install make`):

```sh
make cli        # pure-Go CLI            -> build/bin/godisplacementx-cli
make cli-simd   # experimental SIMD CLI  -> build/bin/godisplacementx-cli-simd
make gui        # Wails desktop app (host-only)
make test       # go test ./...
make bench      # run benchmarks
make help       # list all targets
```

`make cli` is the default build and is identical to before: pure Go, no
`GOEXPERIMENT`, and it cross-compiles to every GOARCH. `make cli-simd` builds the
experimental `simd/archsimd` variant — see [docs/SIMD.md](docs/SIMD.md).

## Running

### CLI

```sh
# Build the CLI (pure Go, cross-compiles without a C toolchain):
make cli        # or: go build -o godisplacementx .

# Generate a 2048x2048 map with a fixed seed:
./godisplacementx generate --seed 42 --resolution 2048 -o out.png

# Normal map / color map / invert:
./godisplacementx generate --seed 42 --mode normal -o normal.png
./godisplacementx generate --seed 42 --mode color --gradient "#00ffff,#9500ff,#ffe500" -o color.png
./godisplacementx generate --seed 42 --invert -o inverted.png

# Sprites and seamless tiling:
./godisplacementx generate --sprites --sprite-packs classic,crappack -o sprites.png
./godisplacementx generate --seamless -o tile.png

# Use a full JSON config (the same shape the GUI uses):
./godisplacementx randomize --seed 1 -o params.json
./godisplacementx generate --config params.json -o out.png
```

Run `./godisplacementx generate --help` for the full flag list. Explicit flags
override values loaded from `--config`.

### GUI

The GUI is built with Wails. Prerequisites: Go and the platform WebView
(WebView2 on Windows — usually preinstalled; WebKitGTK on Linux; WKWebView on
macOS). The frontend is static HTML + htmx, so no Node toolchain is needed to
build the app. Install the Wails CLI once:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Then, from the repo root:

```sh
wails dev      # hot-reload development
make gui       # or: wails build — produces build/bin/godisplacementx(.exe)
```

The resulting binary opens the GUI when launched with no arguments, and behaves
as the CLI when given a subcommand (on Windows it attaches to the parent console
so CLI output is visible).

## Project layout

```
main.go              Entry point: dispatches CLI vs GUI
app.go               Wails-bound backend (Generate, DefaultParams)
app_desktop.go       Native save dialog (desktop build)
gui.go / gui_stub.go GUI launch (behind the `desktop`/`bindings` build tags)
internal/gen/        Pure-Go generation engine (the core port + SIMD kernels)
internal/cli/        cobra CLI (generate / bundle / randomize / version)
frontend/            Static HTML + htmx UI (ported from Displacement X)
tools/rasterize-sprites/  Build-time SVG -> PNG rasterizer (Node + resvg)
```

`go build ./...` (no tags) builds the CLI only; the GUI code is behind the
`desktop` build tag that `wails build` / `wails dev` set automatically. The
`simd` build tag (with `GOEXPERIMENT=simd`) swaps in the SIMD compositing
kernels — see [docs/SIMD.md](docs/SIMD.md).

## Regenerating sprites

Sprite PNGs under `internal/gen/assets/sprites_png/` are generated from the SVGs
in `internal/gen/assets/sprites/` and committed, so a normal build needs no
Node. Only re-run this if the sprite SVGs change:

```sh
cd tools/rasterize-sprites
npm install
npm run rasterize
```

## Tests

```sh
make test        # go test ./...
make test-simd   # same, with GOEXPERIMENT=simd -tags simd (amd64)
```

## Credits

Original [Displacement X](https://github.com/satelllte/displacementx) by
@satelllte; sprites powered by JSplacement.
