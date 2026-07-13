package gen

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"sync"
)

// Decoded sprite rasters are cached per pack for the lifetime of the process:
// the GUI renders repeatedly and RenderBundle's per-seed goroutines each call
// LoadSprites, and re-decoding ~60 embedded PNGs per Generate is pure waste.
// The cached base images are shared across SpriteSets and are never mutated
// after decoding (SpriteSet.Render only reads them), so handing out the same
// slices is safe.
var spriteCache = struct {
	mu    sync.Mutex
	packs map[SpritesPack][]*image.NRGBA
}{packs: map[SpritesPack][]*image.NRGBA{}}

// loadSpritePack returns the decoded base rasters of one pack, decoding it at
// most once per process.
func loadSpritePack(pack SpritesPack) ([]*image.NRGBA, error) {
	spriteCache.mu.Lock()
	defer spriteCache.mu.Unlock()
	if base, ok := spriteCache.packs[pack]; ok {
		return base, nil
	}

	count := spritePackCounts[pack]
	base := make([]*image.NRGBA, 0, count)
	for i := 1; i <= count; i++ {
		path := fmt.Sprintf("assets/sprites_png/%s/%d.png", pack, i)
		data, err := fs.ReadFile(spriteFS, path)
		if err != nil {
			return nil, fmt.Errorf("read sprite %s: %w", path, err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decode sprite %s: %w", path, err)
		}
		base = append(base, toNRGBA(img))
	}
	spriteCache.packs[pack] = base
	return base, nil
}
