package g

import (
	"context"
	"io"
)

type cloudConfigContextReader struct {
	ctx     context.Context
	limited io.LimitedReader
}

func (reader *cloudConfigContextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	// Buffered responses must also observe cancellation between chunks. A
	// blocked network read is still interrupted by the HTTP transport itself.
	const maxReadBytes = 32 << 10
	if len(buffer) > maxReadBytes {
		buffer = buffer[:maxReadBytes]
	}
	n, err := reader.limited.Read(buffer)
	if err == nil || err == io.EOF {
		if canceled := reader.ctx.Err(); canceled != nil {
			return n, canceled
		}
	}
	return n, err
}
