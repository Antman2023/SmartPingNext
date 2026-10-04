package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type proxyCompletionTransport struct {
	body   string
	cancel context.CancelFunc
	closed atomic.Int32
}

func (transport *proxyCompletionTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: request,
		ContentLength: int64(len(transport.body)), Body: &proxyCompletionBody{
			Reader: strings.NewReader(transport.body), cancel: transport.cancel, closed: &transport.closed,
		}}, nil
}

type proxyCompletionBody struct {
	*strings.Reader
	cancel context.CancelFunc
	closed *atomic.Int32
}

func (body *proxyCompletionBody) Read(buffer []byte) (int, error) {
	count, err := body.Reader.Read(buffer)
	if err == io.EOF {
		body.cancel()
	}
	return count, err
}

func (body *proxyCompletionBody) Close() error {
	body.closed.Add(1)
	return nil
}

func TestProxyRejectsCanceledCompletedBodyAndReleasesResources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	transport := &proxyCompletionTransport{body: `{"text":"` + strings.Repeat("完整响应", 8192) + `"}`, cancel: cancel}
	previousTransport := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	const remote = "http://192.0.2.1:8899"
	withProxyConfig(proxyTestConfig(t, remote), func() {
		occupied := len(proxyRequestSlots)
		response := httptest.NewRecorder()
		handleProxy(response, proxyRequest(remote+"/api/config.json").WithContext(ctx))
		if ctx.Err() == nil {
			t.Fatal("completed body did not cancel the request")
		}
		if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "完整响应") {
			t.Fatalf("status=%d, bytes=%d; want only a cancellation error", response.Code, response.Body.Len())
		}
		if transport.closed.Load() != 1 || len(proxyRequestSlots) != occupied {
			t.Fatalf("closed bodies=%d, slots=%d; want 1 and %d", transport.closed.Load(), len(proxyRequestSlots), occupied)
		}
		response = httptest.NewRecorder()
		handleProxy(response, proxyRequest(remote+"/api/config.json"))
		if response.Code != http.StatusOK || response.Body.String() != transport.body || transport.closed.Load() != 2 || len(proxyRequestSlots) != occupied {
			t.Fatalf("subsequent status=%d, closed=%d, slots=%d", response.Code, transport.closed.Load(), len(proxyRequestSlots))
		}
	})
}
