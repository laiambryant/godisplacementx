package gen

import (
	"bufio"
	"image/png"
	"io"
	"os"
	"sync"
)

// pngEncoder is the shared encoder for every PNG this program writes.
// BestSpeed encodes ~1.6x faster than the default level for ~15% larger files
// (measured on generated 2048² maps, where deflate dominates the end-to-end
// render cost); the pixel content is identical either way. The buffer pool
// recycles the encoder's per-image scratch rows across encodes, which bundle
// renders and GUI previews issue repeatedly at the same size.
var pngEncoder = png.Encoder{
	CompressionLevel: png.BestSpeed,
	BufferPool:       sharedPNGBuffers{},
}

var pngBufferPool sync.Pool

type sharedPNGBuffers struct{}

func (sharedPNGBuffers) Get() *png.EncoderBuffer {
	b, _ := pngBufferPool.Get().(*png.EncoderBuffer)
	return b
}

func (sharedPNGBuffers) Put(b *png.EncoderBuffer) { pngBufferPool.Put(b) }

// EncodePNG writes the canvas as a PNG.
func EncodePNG(w io.Writer, c *Canvas) error {
	return pngEncoder.Encode(w, c.NRGBA())
}

// WritePNGFile encodes the canvas into a PNG file through a buffered writer,
// closing it cleanly on success or error.
func WritePNGFile(path string, c *Canvas) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 1<<16)
	if err := EncodePNG(bw, c); err != nil {
		f.Close()
		return err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
