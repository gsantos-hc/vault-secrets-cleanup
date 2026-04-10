package audit

import (
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ulikunitz/xz"
)

type compressedReadCloser struct {
	file   *os.File
	reader io.Reader
}

func (c *compressedReadCloser) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *compressedReadCloser) Close() error {
	return c.file.Close()
}

func OpenFile(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	switch {
	case strings.HasSuffix(path, ".gz"):
		gz, err := gzip.NewReader(file)
		if err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return &compressedReadCloser{file: file, reader: gz}, nil
	case strings.HasSuffix(path, ".bz2"):
		bz2 := bzip2.NewReader(file)
		return &compressedReadCloser{file: file, reader: bz2}, nil
	case strings.HasSuffix(path, ".xz"):
		xzr, err := xz.NewReader(file)
		if err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("failed to create xz reader: %w", err)
		}
		return &compressedReadCloser{file: file, reader: xzr}, nil
	default:
		return file, nil
	}
}
