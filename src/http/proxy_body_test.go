package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"
)

func TestReadProxyHTTPResponseBodyPreservesActualBytes(t *testing.T) {
	for _, test := range []struct {
		name   string
		size   int
		length int64
	}{
		{"small", 128, 128},
		{"growth boundary", 512, 512},
		{"known", 65536, 65536},
		{"unknown", 65536, -1},
		{"zero length hint", 1024, 0},
		{"shorter than declared", 1024, 4096},
		{"longer than declared", 4096, 1024},
		{"empty known", 0, 1024},
		{"empty unknown", 0, -1},
		{"maximum", maxProxyResponseBytes, maxProxyResponseBytes},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload := bytes.Repeat([]byte("x"), test.size)
			body, err := readProxyHTTPResponseBody(&http.Response{
				ContentLength: test.length, Body: io.NopCloser(bytes.NewReader(payload)),
			})
			if err != nil || !bytes.Equal(body, payload) {
				t.Fatalf("body length = %d, want %d; error = %v", len(body), test.size, err)
			}
		})
	}
}

func TestReadProxyHTTPResponseBodyPreservesShortReadsAndFinalData(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 1024)
	for _, reader := range []io.Reader{
		iotest.OneByteReader(bytes.NewReader(payload)),
		iotest.HalfReader(bytes.NewReader(payload)),
		iotest.DataErrReader(bytes.NewReader(payload)),
	} {
		body, err := readProxyHTTPResponseBody(&http.Response{
			ContentLength: int64(len(payload)), Body: io.NopCloser(reader),
		})
		if err != nil || !bytes.Equal(body, payload) {
			t.Fatalf("body length = %d, want %d; error = %v", len(body), len(payload), err)
		}
	}
}

type proxyBodyErrorReader struct{ err error }

func (r proxyBodyErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestReadProxyHTTPResponseBodyDiscardsPartialReadFailures(t *testing.T) {
	readErr := errors.New("remote body failure")
	for _, length := range []int64{-1, 4096} {
		reader := io.MultiReader(bytes.NewReader([]byte(`{"status":true}`)), proxyBodyErrorReader{readErr})
		body, err := readProxyHTTPResponseBody(&http.Response{
			ContentLength: length, Body: io.NopCloser(reader),
		})
		if !errors.Is(err, readErr) || body != nil {
			t.Fatalf("body = %q, error = %v; want no body and original read error", body, err)
		}
	}
}

func TestReadProxyHTTPResponseBodyBoundsIncorrectLengthHints(t *testing.T) {
	payload := make([]byte, maxProxyResponseBytes+100)
	for _, length := range []int64{-1, 0, 1024, maxProxyResponseBytes} {
		reader := bytes.NewReader(payload)
		body, err := readProxyHTTPResponseBody(&http.Response{
			ContentLength: length, Body: io.NopCloser(reader),
		})
		if err == nil || body != nil {
			t.Fatalf("length hint %d: accepted oversized actual body", length)
		}
		if reader.Len() != 99 {
			t.Fatalf("length hint %d: unread bytes = %d, want 99", length, reader.Len())
		}
	}
}

func TestHandleProxyPreservesKnownLengthResponse(t *testing.T) {
	payload := append([]byte(`{"data":"`), bytes.Repeat([]byte("x"), 65536)...)
	payload = append(payload, []byte(`"}`)...)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer remote.Close()
	withProxyConfig(proxyTestConfig(t, remote.URL), func() {
		recorder := httptest.NewRecorder()
		handleProxy(recorder, proxyRequest(remote.URL+"/api/config.json"))
		if recorder.Code != http.StatusOK || !bytes.Equal(recorder.Body.Bytes(), payload) {
			t.Fatalf("proxy status = %d, body length = %d; want original %d-byte JSON", recorder.Code, recorder.Body.Len(), len(payload))
		}
	})
}

func TestHandleProxyRejectsTruncatedKnownLengthResponse(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "4096")
		_, _ = io.WriteString(w, `{"status":true}`)
	}))
	defer remote.Close()
	withProxyConfig(proxyTestConfig(t, remote.URL), func() {
		recorder := httptest.NewRecorder()
		handleProxy(recorder, proxyRequest(remote.URL+"/api/config.json"))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("proxy status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
		}
		if bytes.Contains(recorder.Body.Bytes(), []byte(`{"status":true}`)) {
			t.Fatal("proxy published a truncated remote response")
		}
	})
}

func TestHandleProxyCancelsPartialBodyAndReleasesSlot(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Length", "65536")
			_, _ = io.WriteString(w, `{"data":"`)
			w.(http.Flusher).Flush()
			close(started)
			<-r.Context().Done()
			close(canceled)
			return
		}
		_, _ = io.WriteString(w, `{"recovered":true}`)
	}))
	defer remote.Close()
	withProxyConfig(proxyTestConfig(t, remote.URL), func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		occupied := len(proxyRequestSlots)
		recorder := httptest.NewRecorder()
		done := make(chan struct{})
		go func() {
			handleProxy(recorder, proxyRequest(remote.URL+"/api/config.json").WithContext(ctx))
			close(done)
		}()
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("remote partial body did not start")
		}
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("proxy did not stop after partial body cancellation")
		}
		select {
		case <-canceled:
		case <-time.After(time.Second):
			t.Fatal("remote partial body was not canceled")
		}
		if recorder.Code != http.StatusServiceUnavailable || len(proxyRequestSlots) != occupied {
			t.Fatalf("proxy status = %d, occupied slots = %d; want failure and %d slots", recorder.Code, len(proxyRequestSlots), occupied)
		}
		recovered := httptest.NewRecorder()
		handleProxy(recovered, proxyRequest(remote.URL+"/api/config.json"))
		if recovered.Code != http.StatusOK || recovered.Body.String() != `{"recovered":true}` {
			t.Fatalf("subsequent proxy status = %d, body = %q", recovered.Code, recovered.Body.String())
		}
	})
}
