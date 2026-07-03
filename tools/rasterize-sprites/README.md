# rasterize-sprites

Build-time tool that converts the bundled sprite SVGs
(`internal/gen/assets/sprites/<pack>/<n>.svg`) into PNGs
(`internal/gen/assets/sprites_png/<pack>/<n>.png`) at a fixed base resolution
(512px). The Go runtime embeds the PNGs.

Why a Node tool and not Go: the sprites must render exactly as they do in the
original browser app. [`@resvg/resvg-js`](https://github.com/yisibl/resvg-js) is
a browser-grade static SVG renderer that handles `clipPath`/`<use>`; lightweight
Go SVG rasterizers drop those elements and produce solid blobs for several of
the "classic" sprites.

You only need to run this if the sprite SVGs change.

```sh
cd tools/rasterize-sprites
npm install
npm run rasterize
```

The generated PNGs are committed to the repo, so a normal build does not require
Node.
