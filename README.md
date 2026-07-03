# godisplacementx

A Go port of [Displacement X](https://displacementx.pages.dev/) — a procedural
generator of grayscale **displacement / height maps** (a sci-fi "JSplacement"
aesthetic) for 3D software such as Blender, Cinema4D and Octane.

It ships as a **single executable** that is both:

- a **CLI** for headless / scripted generation, and
- a **cross-platform GUI** (Windows / macOS / Linux) built with
  [Wails](https://wails.io/) that reuses the original React UI.

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

## Running

### CLI

```sh
# Build the CLI (pure Go, cross-compiles without a C toolchain):
go build -o godisplacementx .

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

The GUI is built with Wails. Prerequisites: Go, Node.js, and the platform
WebView (WebView2 on Windows — usually preinstalled; WebKitGTK on Linux;
WKWebView on macOS). Install the Wails CLI once:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Then, from the repo root:

```sh
wails dev      # hot-reload development
wails build    # produces build/bin/godisplacementx(.exe)
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
internal/gen/        Pure-Go generation engine (the core port)
internal/cli/        cobra CLI (generate / randomize / version)
frontend/            Vite + React UI (ported from Displacement X)
tools/rasterize-sprites/  Build-time SVG -> PNG rasterizer (Node + resvg)
```

`go build ./...` (no tags) builds the CLI only; the GUI code is behind the
`desktop` build tag that `wails build` / `wails dev` set automatically.

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
go test ./...
```

## Credits

Original [Displacement X](https://github.com/satelllte/displacementx) by
@satelllte; sprites powered by JSplacement.
