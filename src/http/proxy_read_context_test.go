package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

type contextProxyReadBody struct {
	reader    io.Reader
	onRead    func()
	reads     int
	closed    int
	maxRead   int
	readBytes int
}

func (body *contextProxyReadBody) Read(buffer []byte) (int, error) {
	body.reads++
	if len(buffer) > body.maxRead {
		body.maxRead = len(buffer)
	}
	n, err := body.reader.Read(buffer)
	body.readBytes += n
	if body.onRead != nil {
		body.onRead()
	}
	return n, err
}

func (body *contextProxyReadBody) Close() error { body.closed++; return nil }

func TestProxyReadContextSkipsFinishedRequestsBeforeReading(t *testing.T) {
	for _, expired := range []bool{false, true} {
		for _, length := range []int64{-1, 0, 1024, maxProxyResponseBytes} {
			t.Run(fmt.Sprintf("Expired=%t/Length=%d", expired, length), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if expired {
					ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				}
				defer cancel()
				reader := &contextProxyReadBody{reader: strings.NewReader(`{"status":true}`)}
				body, err := readProxyHTTPResponseBodyContext(ctx, &http.Response{ContentLength: length, Body: reader})
				if !errors.Is(err, ctx.Err()) || body != nil || reader.reads != 0 {
					t.Fatalf("body=%d bytes, error=%v, reads=%d; want cancellation without a read", len(body), err, reader.reads)
				}
			})
		}
	}
}

func TestProxyReadContextDiscardsCanceledChunksAndFinalData(t *testing.T) {
	const readBatch = 32 << 10
	for _, length := range []int64{-1, 0, 256 << 10} {
		for _, finalData := range []bool{false, true} {
			t.Run(fmt.Sprintf("Length=%d/FinalData=%t", length, finalData), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				payload := bytes.Repeat([]byte("x"), 256<<10)
				var source io.Reader = bytes.NewReader(payload)
				if finalData {
					source = iotest.DataErrReader(bytes.NewReader(payload[:128]))
				}
				reader := &contextProxyReadBody{reader: source, onRead: cancel}
				body, err := readProxyHTTPResponseBodyContext(ctx, &http.Response{ContentLength: length, Body: reader})
				if !errors.Is(err, context.Canceled) || body != nil || reader.reads != 1 || reader.maxRead > readBatch {
					t.Fatalf("body=%d, error=%v, reads=%d, largest buffer=%d; want canceled first bounded read", len(body), err, reader.reads, reader.maxRead)
				}
				if reader.readBytes > readBatch {
					t.Fatalf("read %d bytes after cancellation checkpoint", reader.readBytes)
				}
			})
		}
	}
}

func TestProxyReadContextPreservesActiveResultsAndReadErrors(t *testing.T) {
	payload := bytes.Repeat([]byte("value <&> \"\\节点"), 128)
	for _, length := range []int64{-1, 0, 512, int64(len(payload)), int64(len(payload) + 4096)} {
		for _, kind := range []string{"normal", "one-byte", "half", "final-data", "failure"} {
			t.Run(fmt.Sprintf("Length=%d/%s", length, kind), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				var reader io.Reader = bytes.NewReader(payload)
				readErr := errors.New("remote read failure")
				switch kind {
				case "one-byte":
					reader = iotest.OneByteReader(reader)
				case "half":
					reader = iotest.HalfReader(reader)
				case "final-data":
					reader = iotest.DataErrReader(reader)
				case "failure":
					reader = io.MultiReader(reader, proxyBodyErrorReader{err: readErr})
				}
				body, err := readProxyHTTPResponseBodyContext(ctx, &http.Response{ContentLength: length, Body: io.NopCloser(reader)})
				if kind == "failure" {
					if !errors.Is(err, readErr) || body != nil {
						t.Fatalf("returned partial failure: bytes=%d, error=%v", len(body), err)
					}
				} else if err != nil || !bytes.Equal(body, payload) {
					t.Fatalf("bytes=%d, error=%v; want original %d bytes", len(body), err, len(payload))
				}
			})
		}
	}
}

type contextProxyReadTransport struct {
	body       *contextProxyReadBody
	beforeRead func()
	length     int64
}

func TestProxyReadContextPreservesLimitsAndMissingBodies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, response := range []*http.Response{nil, {ContentLength: 1024}} {
		body, err := readProxyHTTPResponseBodyContext(ctx, response)
		if body != nil || err == nil || err.Error() != "Proxy Response Body Missing!" {
			t.Fatalf("missing response: bytes=%d, error=%v", len(body), err)
		}
	}
	payload := make([]byte, maxProxyResponseBytes+100)
	for _, length := range []int64{-1, 0, 1024, maxProxyResponseBytes, maxProxyResponseBytes + 1} {
		t.Run(fmt.Sprint(length), func(t *testing.T) {
			reader := &contextProxyReadBody{reader: bytes.NewReader(payload)}
			body, err := readProxyHTTPResponseBodyContext(ctx, &http.Response{ContentLength: length, Body: reader})
			if body != nil || err == nil || err.Error() != "Proxy Response Too Large!" {
				t.Fatalf("size limit: bytes=%d, error=%v", len(body), err)
			}
			wantBytes := maxProxyResponseBytes + 1
			if length > maxProxyResponseBytes {
				wantBytes = 0
			}
			if reader.readBytes != wantBytes || reader.maxRead > 32<<10 || reader.closed != 0 {
				t.Fatalf("read bytes=%d, largest buffer=%d, closes=%d; want %d, <=32768, 0", reader.readBytes, reader.maxRead, reader.closed, wantBytes)
			}
		})
	}
}

func (transport *contextProxyReadTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport.beforeRead != nil {
		transport.beforeRead()
	}
	return &http.Response{StatusCode: http.StatusOK, Request: request, Header: make(http.Header),
		ContentLength: transport.length, Body: transport.body}, nil
}

func TestProxyReadContextHandlerClosesCanceledBodyAndRecovers(t *testing.T) {
	for _, beforeRead := range []bool{false, true} {
		t.Run(fmt.Sprintf("BeforeRead=%t", beforeRead), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			payload := `{"data":"` + strings.Repeat("x", 128<<10) + `"}`
			reader := &contextProxyReadBody{reader: strings.NewReader(payload)}
			transport := &contextProxyReadTransport{body: reader, length: maxProxyResponseBytes}
			if beforeRead {
				transport.beforeRead = cancel
			} else {
				reader.onRead = cancel
			}
			previous := http.DefaultTransport
			http.DefaultTransport = transport
			t.Cleanup(func() { http.DefaultTransport = previous })
			const remote = "http://192.0.2.1:8899"
			withProxyConfig(proxyTestConfig(t, remote), func() {
				slots := len(proxyRequestSlots)
				response := httptest.NewRecorder()
				handleProxy(response, proxyRequest(remote+"/api/config.json").WithContext(ctx))
				if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "context canceled") || strings.Contains(response.Body.String(), `{"data":`) {
					t.Fatalf("status=%d, bytes=%d; want only a cancellation error", response.Code, response.Body.Len())
				}
				wantReads := 1
				if beforeRead {
					wantReads = 0
				}
				if reader.reads != wantReads || reader.closed != 1 || len(proxyRequestSlots) != slots {
					t.Fatalf("reads=%d, closed=%d, slots=%d; want %d, 1, %d", reader.reads, reader.closed, len(proxyRequestSlots), wantReads, slots)
				}
				reader.reader = strings.NewReader(payload)
				reader.onRead = nil
				transport.beforeRead = nil
				response = httptest.NewRecorder()
				handleProxy(response, proxyRequest(remote+"/api/config.json"))
				if response.Code != http.StatusOK || response.Body.String() != payload || reader.closed != 2 || len(proxyRequestSlots) != slots {
					t.Fatalf("recovery status=%d, closed=%d, slots=%d", response.Code, reader.closed, len(proxyRequestSlots))
				}
			})
		})
	}
}
