package archiver

import (
	"compress/flate"
	"io"
)

func deflateCompressorFor(level int) func(io.Writer) (io.WriteCloser, error) {
	if level < flate.BestSpeed {
		level = flate.DefaultCompression
	}
	if level > flate.BestCompression {
		level = flate.BestCompression
	}

	return func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, level)
	}
}
