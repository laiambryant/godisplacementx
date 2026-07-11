package gen

import (
	"runtime"
	"sync"
)

// parallelMinPixels is the amount of per-pixel work below which splitting a
// pass across goroutines costs more than it saves. 16Ki pixels of compositing
// is ~15µs against ~5µs of spawn/sync; tuned on BenchmarkGenerate, where this
// value wins ~15-25% on 2048+ canvases and costs ≲5% on sub-millisecond small
// ones.
const parallelMinPixels = 1 << 14

// parallelBands splits [0, n) into contiguous bands of at least minPerBand
// units and runs fn on the bands concurrently, blocking until all complete.
// The bands are disjoint and every unit is processed exactly once, so any
// per-pixel-independent pass stays bit-exact regardless of the split. Small
// work runs inline with no goroutines.
func parallelBands(n, minPerBand int, fn func(lo, hi int)) {
	if minPerBand < 1 {
		minPerBand = 1
	}
	workers := runtime.GOMAXPROCS(0)
	if maxBands := n / minPerBand; workers > maxBands {
		workers = maxBands
	}
	if workers <= 1 {
		fn(0, n)
		return
	}
	band := (n + workers - 1) / workers
	var wg sync.WaitGroup
	for lo := 0; lo < n; lo += band {
		hi := min(lo+band, n)
		wg.Go(func() {
			fn(lo, hi)
		})
	}
	wg.Wait()
}

// minRowsPerBand returns the row-band granularity that keeps at least
// parallelMinPixels of work per band for rows of rowPixels pixels each.
func minRowsPerBand(rowPixels int) int {
	if rowPixels < 1 {
		return 1
	}
	return (parallelMinPixels + rowPixels - 1) / rowPixels
}
