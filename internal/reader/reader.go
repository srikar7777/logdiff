// Package reader provides a streaming, context-aware log line reader.
package reader

import (
	"bufio"
	"context"
	"fmt"
	"os"
)

// DefaultBufSize is the default scanner buffer — 1 MiB, enough for any
// reasonable log line while keeping memory per-file predictable.
const DefaultBufSize = 1 << 20 // 1 MiB

// ReadLines opens path and streams every line over the returned channel.
// bufSize controls the scanner buffer; pass 0 for DefaultBufSize.
// Both channels are closed when the goroutine exits.
// Any error (file open, scan error) is sent on errs (buffered, cap 1).
func ReadLines(ctx context.Context, path string, bufSize int) (<-chan string, <-chan error) {
	if bufSize <= 0 {
		bufSize = DefaultBufSize
	}

	lines := make(chan string, 1000)
	errs := make(chan error, 1)

	go func() {
		defer close(lines)
		defer close(errs)

		f, err := os.Open(path)
		if err != nil {
			errs <- fmt.Errorf("reader: open %q: %w", path, err)
			return
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, bufSize), bufSize)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case lines <- scanner.Text():
			}
		}

		if err := scanner.Err(); err != nil {
			errs <- fmt.Errorf("reader: scan %q: %w", path, err)
		}
	}()

	return lines, errs
}
