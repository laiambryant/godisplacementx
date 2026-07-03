// Pre-rasterizes the bundled sprite SVGs into PNGs at a fixed base resolution.
// The runtime (internal/gen/sprites.go) embeds these PNGs instead of the SVGs,
// because a faithful, browser-grade SVG renderer (resvg) is required: some of
// the original sprites use clipPath/<use> that lightweight Go rasterizers drop.
//
// Usage:
//   npm install            # installs @resvg/resvg-js
//   npm run rasterize      # or: node rasterize.mjs [srcDir] [dstDir] [size]
//
// Defaults: srcDir=../../internal/gen/assets/sprites
//           dstDir=../../internal/gen/assets/sprites_png
//           size=512
import { Resvg } from '@resvg/resvg-js';
import { readFileSync, writeFileSync, mkdirSync, readdirSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';

const here = dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1'));
const SRC = resolve(process.argv[2] || join(here, '../../internal/gen/assets/sprites'));
const DST = resolve(process.argv[3] || join(here, '../../internal/gen/assets/sprites_png'));
const SIZE = parseInt(process.argv[4] || '512', 10);

let count = 0;
for (const pack of readdirSync(SRC)) {
  const packDir = join(SRC, pack);
  let files;
  try {
    files = readdirSync(packDir);
  } catch {
    continue;
  }
  for (const f of files) {
    if (!f.endsWith('.svg')) continue;
    const svg = readFileSync(join(packDir, f), 'utf8');
    const r = new Resvg(svg, {
      fitTo: { mode: 'width', value: SIZE },
      background: 'rgba(0,0,0,0)',
    });
    const png = r.render().asPng();
    const out = join(DST, pack, f.replace(/\.svg$/, '.png'));
    mkdirSync(dirname(out), { recursive: true });
    writeFileSync(out, png);
    count++;
  }
}
console.log(`rasterized ${count} sprites @${SIZE}px -> ${DST}`);
