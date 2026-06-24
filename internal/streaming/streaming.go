package streaming

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
)

// ConsumeNDJSON reads newline-delimited JSON from r and invokes onLine for each JSON line.
// It returns when onLine reports done, EOF is reached, context is canceled, or an error occurs.
func ConsumeNDJSON(ctx context.Context, r io.Reader, onLine func(line []byte) (done bool, err error)) error {
	reader := bufio.NewReader(r)
	readBuf := make([]byte, 4096)
	pending := make([]byte, 0, 4096)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		n, readErr := reader.Read(readBuf)
		if n > 0 {
			pending = append(pending, readBuf[:n]...)
			for {
				nl := bytes.IndexByte(pending, '\n')
				if nl < 0 {
					break
				}
				line := bytes.TrimSpace(pending[:nl])
				pending = pending[nl+1:]
				if len(line) == 0 {
					continue
				}
				done, err := onLine(line)
				if err != nil {
					return err
				}
				if done {
					return nil
				}
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				line := bytes.TrimSpace(pending)
				if len(line) == 0 {
					return nil
				}
				_, err := onLine(line)
				return err
			}
			return readErr
		}
	}
}

