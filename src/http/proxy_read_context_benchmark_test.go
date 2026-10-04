package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func BenchmarkProxyResponseBodyContext(b *testing.B) {
	for _, size := range []int{0, 128, 64 << 10, 1 << 20, maxProxyResponseBytes} {
		payload := bytes.Repeat([]byte("x"), size)
		for _, knownLength := range []bool{false, true} {
			length := int64(-1)
			if knownLength {
				length = int64(size)
			}
			for _, mode := range []string{"Legacy", "Layered", "Background", "Active", "PreCanceled", "Deadline", "CancelFirstRead"} {
				b.Run(fmt.Sprintf("Size=%d/Known=%t/%s", size, knownLength, mode), func(b *testing.B) {
					ctx := context.Background()
					if mode == "Active" || mode == "Layered" || mode == "PreCanceled" {
						var cancel context.CancelFunc
						ctx, cancel = context.WithCancel(ctx)
						b.Cleanup(cancel)
						if mode == "PreCanceled" {
							cancel()
						}
					} else if mode == "Deadline" {
						var cancel context.CancelFunc
						ctx, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
						b.Cleanup(cancel)
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						requestContext := ctx
						var reader io.ReadCloser = io.NopCloser(bytes.NewReader(payload))
						var cancel context.CancelFunc
						if mode == "CancelFirstRead" {
							requestContext, cancel = context.WithCancel(context.Background())
							reader = &contextProxyReadBody{reader: bytes.NewReader(payload), onRead: cancel}
						}
						response := &http.Response{ContentLength: length, Body: reader}
						var body []byte
						var err error
						if mode == "Legacy" {
							body, err = readProxyHTTPResponseBody(response)
						} else if mode == "Layered" {
							body, err = readLayeredProxyBodyForBenchmark(requestContext, response)
						} else {
							body, err = readProxyHTTPResponseBodyContext(requestContext, response)
						}
						if cancel != nil {
							cancel()
						}
						if mode == "PreCanceled" || mode == "Deadline" || mode == "CancelFirstRead" {
							if !errors.Is(err, requestContext.Err()) || body != nil {
								b.Fatalf("canceled body=%d, error=%v", len(body), err)
							}
						} else if err != nil || !bytes.Equal(body, payload) {
							b.Fatalf("body=%d, error=%v", len(body), err)
						}
					}
				})
			}
		}
	}
}

// Retain the previous two-adapter implementation for an in-process comparison
// that checks the same cancellation points, chunks and response byte budget.
type layeredProxyReaderForBenchmark struct {
	ctx    context.Context
	reader io.Reader
}

func (reader layeredProxyReaderForBenchmark) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	if len(buffer) > 32<<10 {
		buffer = buffer[:32<<10]
	}
	count, err := reader.reader.Read(buffer)
	if err == nil || err == io.EOF {
		if canceled := reader.ctx.Err(); canceled != nil {
			return count, canceled
		}
	}
	return count, err
}

func readLayeredProxyBodyForBenchmark(ctx context.Context, response *http.Response) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if response == nil || response.Body == nil {
		return nil, errors.New("Proxy Response Body Missing!")
	}
	var reader io.Reader = response.Body
	if ctx.Done() != nil {
		reader = layeredProxyReaderForBenchmark{ctx: ctx, reader: reader}
	}
	body, err := readProxyResponseBodyWithLength(reader, response.ContentLength)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return body, nil
}
