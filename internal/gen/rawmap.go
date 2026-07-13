package gen

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// The .gdxraw interchange format skips the PNG round-trip entirely for
// consumers that re-upload pixels anyway (the Godot plugin): no filtering, no
// deflate, no decode on the other side. Little-endian, 16-byte header:
//
//	offset 0  magic   "GDXR"
//	offset 4  u8      version (1)
//	offset 5  u8      format: 0 = L8, 1 = RGB8, 2 = RGBA8
//	offset 6  u16     reserved (0)
//	offset 8  u32     width
//	offset 12 u32     height
//	offset 16 payload, tightly packed rows
//
// Grayscale emits store the red channel only (L8): a height field is an
// intensity map by definition (R=G=B by construction), and any alpha left by
// xor draws is a compositing artifact its consumers ignore. Colour/normal
// emits stay lossless: RGB8 when fully opaque, RGBA8 otherwise.

const (
	rawMagic   = "GDXR"
	rawVersion = 1

	rawFormatL8    = 0
	rawFormatRGB8  = 1
	rawFormatRGBA8 = 2
)

// rawExtension marks an emit path as .gdxraw output; WriteMapFile dispatches
// on it, so the CLI's --emit syntax needs no format field.
const rawExtension = ".gdxraw"

// WriteMapFile writes the canvas to path in the format its extension selects:
// .gdxraw for the raw interchange format, PNG otherwise (the fast PNG variant
// when fast is set).
func WriteMapFile(path string, c *Canvas, mode OutputMode, fast bool) error {
	if strings.HasSuffix(strings.ToLower(path), rawExtension) {
		return WriteRawFile(path, c, mode)
	}
	if fast {
		return writePNGFileWith(path, c, EncodePNGFast)
	}
	return WritePNGFile(path, c)
}

// WriteRawFile encodes the canvas into a .gdxraw file through a buffered writer.
func WriteRawFile(path string, c *Canvas, mode OutputMode) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return encodeBuffered(f, c, func(w io.Writer, c *Canvas) error {
		return EncodeRaw(w, c, mode)
	})
}

// EncodeRaw writes the canvas as a .gdxraw stream.
func EncodeRaw(w io.Writer, c *Canvas, mode OutputMode) error {
	if c.W <= 0 || c.H <= 0 || len(c.Pix) < c.W*c.H*4 {
		return io.ErrUnexpectedEOF
	}
	format := rawFormatFor(c, mode)
	if err := writeRawHeader(w, c.W, c.H, format); err != nil {
		return err
	}
	payload := rawPayload(c, format)
	_, err := w.Write(payload)
	return err
}

func rawFormatFor(c *Canvas, mode OutputMode) uint8 {
	switch mode {
	case OutputGrayscale, "":
		return rawFormatL8
	}
	if isOpaquePix(c.Pix) {
		return rawFormatRGB8
	}
	return rawFormatRGBA8
}

func writeRawHeader(w io.Writer, width, height int, format uint8) error {
	var header [16]byte
	copy(header[0:4], rawMagic)
	header[4] = rawVersion
	header[5] = format
	binary.LittleEndian.PutUint32(header[8:12], uint32(width))
	binary.LittleEndian.PutUint32(header[12:16], uint32(height))
	_, err := w.Write(header[:])
	return err
}

// rawPayload extracts the payload bytes for the chosen format. RGBA8 is the
// pixel buffer itself; L8/RGB8 drop channels into a fresh buffer, band-parallel.
func rawPayload(c *Canvas, format uint8) []uint8 {
	if format == rawFormatRGBA8 {
		return c.Pix
	}
	n := c.W * c.H
	bpp := 3
	if format == rawFormatL8 {
		bpp = 1
	}
	payload := make([]uint8, n*bpp)
	parallelBands(n, parallelMinPixels, func(lo, hi int) {
		if format == rawFormatL8 {
			for i := lo; i < hi; i++ {
				payload[i] = c.Pix[i*4]
			}
			return
		}
		for i := lo; i < hi; i++ {
			payload[i*3] = c.Pix[i*4]
			payload[i*3+1] = c.Pix[i*4+1]
			payload[i*3+2] = c.Pix[i*4+2]
		}
	})
	return payload
}

// readRaw decodes a .gdxraw stream. The production reader is the Godot plugin;
// this one exists so tests can pin the format.
func readRaw(r io.Reader) (width, height int, format uint8, payload []uint8, err error) {
	var header [16]byte
	if _, err = io.ReadFull(r, header[:]); err != nil {
		return
	}
	if string(header[0:4]) != rawMagic {
		err = fmt.Errorf("bad magic %q", header[0:4])
		return
	}
	if header[4] != rawVersion {
		err = fmt.Errorf("unsupported version %d", header[4])
		return
	}
	format = header[5]
	var bpp int
	switch format {
	case rawFormatL8:
		bpp = 1
	case rawFormatRGB8:
		bpp = 3
	case rawFormatRGBA8:
		bpp = 4
	default:
		err = fmt.Errorf("unsupported format %d", format)
		return
	}
	width = int(binary.LittleEndian.Uint32(header[8:12]))
	height = int(binary.LittleEndian.Uint32(header[12:16]))
	payload = make([]uint8, width*height*bpp)
	_, err = io.ReadFull(r, payload)
	return
}
