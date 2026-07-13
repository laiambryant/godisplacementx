package gen

import (
	"bytes"
	"encoding/binary"
	"hash/adler32"
	"hash/crc32"
	"io"
	"sync/atomic"

	"github.com/klauspost/compress/flate"
)

// Row-parallel PNG encoder. The stdlib encoder filters and deflates the whole
// image on one core, which made PNG writing the dominant cost of a render once
// generation itself was parallelised. This writer keeps the exact same pixel
// semantics (8-bit RGB for opaque images, RGBA otherwise, adaptive per-row
// filters, a standard zlib stream split across IDAT chunks) but processes
// fixed-height row bands concurrently: each band is filtered and deflated
// independently, the deflate segments are byte-aligned with a sync flush so
// their concatenation is one valid stream, and the zlib checksum is stitched
// from per-band adler32 values. Band boundaries are fixed (not core-count
// dependent), so the emitted bytes are deterministic on every machine.
//
// Decoded output is pinned against the canvas pixels by TestEncodePNGRoundTrip.

// pngBandRows is the fixed band height. 128 rows keeps dozens of bands in
// flight at large resolutions while staying irrelevant below ~256².
const pngBandRows = 128

const adlerModulus = 65521

// adlerCombine merges adler32(b) into adler32(a) given len(b), as zlib's
// adler32_combine does, so bands can be checksummed independently.
func adlerCombine(a, b uint32, lenB int) uint32 {
	rem := uint32(lenB % adlerModulus)
	sum1 := a & 0xffff
	sum2 := (rem * sum1) % adlerModulus
	sum1 += (b & 0xffff) + adlerModulus - 1
	sum2 += ((a >> 16) & 0xffff) + ((b >> 16) & 0xffff) + adlerModulus - rem
	if sum1 >= adlerModulus {
		sum1 -= adlerModulus
	}
	if sum1 >= adlerModulus {
		sum1 -= adlerModulus
	}
	if sum2 >= adlerModulus<<1 {
		sum2 -= adlerModulus << 1
	}
	if sum2 >= adlerModulus {
		sum2 -= adlerModulus
	}
	return sum1 | sum2<<16
}

// isOpaquePix reports whether every alpha byte of a straight-RGBA buffer is
// 255, scanning bands in parallel.
func isOpaquePix(pix []uint8) bool {
	var translucent atomic.Bool
	parallelBands(len(pix)/4, parallelMinPixels, func(lo, hi int) {
		if translucent.Load() {
			return
		}
		for i := lo; i < hi; i++ {
			if pix[i*4+3] != 255 {
				translucent.Store(true)
				return
			}
		}
	})
	return !translucent.Load()
}

// extractRow writes one image row as PNG scanline bytes (RGB or RGBA).
func extractRow(dst []uint8, pix []uint8, y, w, bpp int) {
	src := pix[y*w*4 : (y+1)*w*4]
	if bpp == 4 {
		copy(dst, src)
		return
	}
	for x := 0; x < w; x++ {
		dst[x*3] = src[x*4]
		dst[x*3+1] = src[x*4+1]
		dst[x*3+2] = src[x*4+2]
	}
}

func absInt8Sum(row []uint8) int {
	sum := 0
	for _, v := range row {
		d := int(int8(v))
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return sum
}

func pngAbs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func paethPredictor(a, b, c int) int {
	p := a + b - c
	pa := pngAbs(p - a)
	pb := pngAbs(p - b)
	pc := pngAbs(p - c)
	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}

// rowFilterScratch holds the five per-filter candidate buffers so a band can
// filter every row without reallocating.
type rowFilterScratch struct {
	candidates [5][]uint8
}

func newRowFilterScratch(rowBytes int) *rowFilterScratch {
	s := &rowFilterScratch{}
	for f := range s.candidates {
		s.candidates[f] = make([]uint8, rowBytes)
	}
	return s
}

// filterRow picks the smallest-sum-of-residuals filter for one scanline (the
// stdlib heuristic) and writes the filter tag plus filtered bytes into dst.
// cur and prev are the raw scanline bytes; prev is all zeros for the first row.
func filterRow(dst, cur, prev []uint8, bpp int, s *rowFilterScratch) {
	n := len(cur)
	candidates := s.candidates
	copy(candidates[0], cur)
	for i := 0; i < n; i++ {
		var left, upLeft int
		if i >= bpp {
			left = int(cur[i-bpp])
			upLeft = int(prev[i-bpp])
		}
		up := int(prev[i])
		c := int(cur[i])
		candidates[1][i] = uint8(c - left)
		candidates[2][i] = uint8(c - up)
		candidates[3][i] = uint8(c - (left+up)/2)
		candidates[4][i] = uint8(c - paethPredictor(left, up, upLeft))
	}
	best := 0
	bestSum := absInt8Sum(candidates[0][:n])
	for f := 1; f < 5; f++ {
		if sum := absInt8Sum(candidates[f][:n]); sum < bestSum {
			best, bestSum = f, sum
		}
	}
	dst[0] = uint8(best)
	copy(dst[1:], candidates[best][:n])
}

// filterRowUp writes one scanline with the fixed Up filter: no per-row
// candidate scoring, just the residual against the previous row. Used by the
// fast encoder, where speed beats the few percent of extra deflate ratio.
func filterRowUp(dst, cur, prev []uint8) {
	dst[0] = 2 // Up
	for i, v := range cur {
		dst[1+i] = v - prev[i]
	}
}

// pngBand is one encoded row band: a byte-aligned deflate segment plus the
// adler32 of the filtered bytes it compresses.
type pngBand struct {
	deflated bytes.Buffer
	adler    uint32
	rawLen   int
}

// encodeBand filters rows [y0, y1) and deflates them into one segment. Every
// band ends with a sync flush (byte-aligned, resumable), so concatenating the
// segments in order forms a single valid deflate stream. fastFilter swaps the
// adaptive per-row filter heuristic for the fixed Up filter and drops the
// deflate matcher to Huffman-only.
func encodeBand(band *pngBand, pix []uint8, w, y0, y1, bpp int, fastFilter bool) {
	rowLen := 1 + w*bpp
	filtered := make([]uint8, (y1-y0)*rowLen)
	var scratch *rowFilterScratch
	if !fastFilter {
		scratch = newRowFilterScratch(w * bpp)
	}
	cur := make([]uint8, w*bpp)
	prev := make([]uint8, w*bpp)
	if y0 > 0 {
		extractRow(prev, pix, y0-1, w, bpp)
	}
	for y := y0; y < y1; y++ {
		extractRow(cur, pix, y, w, bpp)
		row := filtered[(y-y0)*rowLen : (y-y0+1)*rowLen]
		if fastFilter {
			filterRowUp(row, cur, prev)
		} else {
			filterRow(row, cur, prev, bpp, scratch)
		}
		cur, prev = prev, cur
	}

	band.adler = adler32.Checksum(filtered)
	band.rawLen = len(filtered)
	level := flate.BestSpeed
	if fastFilter {
		level = flate.HuffmanOnly
	}
	// flate only fails through its sink, and bytes.Buffer never does.
	fw, _ := flate.NewWriter(&band.deflated, level)
	_, _ = fw.Write(filtered)
	_ = fw.Flush()
}

func writeChunk(w io.Writer, kind string, data []byte) error {
	var head [8]byte
	binary.BigEndian.PutUint32(head[:4], uint32(len(data)))
	copy(head[4:], kind)
	crc := crc32.NewIEEE()
	crc.Write(head[4:])
	crc.Write(data)
	var tail [4]byte
	binary.BigEndian.PutUint32(tail[:], crc.Sum32())
	if _, err := w.Write(head[:]); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err := w.Write(tail[:])
	return err
}

// deflateFinalBlock returns the terminating final-empty-block bytes that close
// a deflate stream assembled from sync-flushed segments.
func deflateFinalBlock() []byte {
	var buf bytes.Buffer
	fw, _ := flate.NewWriter(&buf, flate.BestSpeed)
	fw.Close()
	return buf.Bytes()
}

// EncodePNG writes the canvas as an 8-bit PNG (RGB when fully opaque, RGBA
// otherwise), encoding row bands in parallel. The byte stream is deterministic
// for a given canvas.
func EncodePNG(w io.Writer, c *Canvas) error {
	return encodePNG(w, c, false)
}

// EncodePNGFast is the fast-mode PNG encoder: identical pixels, but fixed Up
// filtering and Huffman-only deflate, trading file size for encode speed.
func EncodePNGFast(w io.Writer, c *Canvas) error {
	return encodePNG(w, c, true)
}

func encodePNG(w io.Writer, c *Canvas, fastFilter bool) error {
	width, height := c.W, c.H
	if width <= 0 || height <= 0 || len(c.Pix) < width*height*4 {
		return io.ErrUnexpectedEOF
	}
	bpp := 4
	colorType := uint8(6) // RGBA
	if isOpaquePix(c.Pix) {
		bpp = 3
		colorType = 2 // RGB
	}

	bandCount := (height + pngBandRows - 1) / pngBandRows
	bands := make([]pngBand, bandCount)
	parallelBands(bandCount, 1, func(lo, hi int) {
		for b := lo; b < hi; b++ {
			y0 := b * pngBandRows
			y1 := min(y0+pngBandRows, height)
			encodeBand(&bands[b], c.Pix, width, y0, y1, bpp, fastFilter)
		}
	})

	if _, err := w.Write([]byte("\x89PNG\r\n\x1a\n")); err != nil {
		return err
	}
	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:4], uint32(width))
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(height))
	ihdr[8] = 8 // bit depth
	ihdr[9] = colorType
	if err := writeChunk(w, "IHDR", ihdr[:]); err != nil {
		return err
	}

	// One IDAT per band: the first carries the zlib header, the last carries
	// the stream terminator and the stitched checksum. Decoders concatenate
	// IDAT payloads, reconstructing the single zlib stream.
	adler := uint32(1)
	for i := range bands {
		payload := bands[i].deflated.Bytes()
		if i == 0 {
			payload = append([]byte{0x78, 0x01}, payload...)
		}
		adler = adlerCombine(adler, bands[i].adler, bands[i].rawLen)
		if i == len(bands)-1 {
			payload = append(payload, deflateFinalBlock()...)
			var sum [4]byte
			binary.BigEndian.PutUint32(sum[:], adler)
			payload = append(payload, sum[:]...)
		}
		if err := writeChunk(w, "IDAT", payload); err != nil {
			return err
		}
	}

	return writeChunk(w, "IEND", nil)
}
