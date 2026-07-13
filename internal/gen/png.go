package gen

import (
	"bufio"
	"io"
	"os"
)

// WritePNGFile encodes the canvas into a PNG file through a buffered writer,
// closing it cleanly on success or error. The encoding itself is the parallel
// encoder in pngfast.go.
func WritePNGFile(path string, c *Canvas) error {
	return writePNGFileWith(path, c, EncodePNG)
}

func writePNGFileWith(path string, c *Canvas, encode func(io.Writer, *Canvas) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return encodeBuffered(f, c, encode)
}

func encodeBuffered(f io.WriteCloser, c *Canvas, encode func(io.Writer, *Canvas) error) error {
	bw := bufio.NewWriterSize(f, 1<<16)
	if err := encode(bw, c); err != nil {
		f.Close()
		return err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
