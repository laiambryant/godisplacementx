package gen

import (
	"image/png"
	"io"
)

// EncodePNG writes the canvas as a PNG.
func EncodePNG(w io.Writer, c *Canvas) error {
	return png.Encode(w, c.NRGBA())
}
