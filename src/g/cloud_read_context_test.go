package g

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

type cloudReadEventBody struct {
	data          string
	cancel        context.CancelFunc
	reads, closes int
	withEOF       bool
}

type cloudReaderFunc func([]byte) (int, error)

func (read cloudReaderFunc) Read(buffer []byte) (int, error) { return read(buffer) }

func TestCloudReadContextBoundsAndErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, reader := range []io.Reader{strings.NewReader("config"), iotest.OneByteReader(strings.NewReader("config")), iotest.DataErrReader(strings.NewReader("config"))} {
		body, err := readCloudConfigBodyContext(ctx, reader)
		if err != nil || string(body) != "config" {
			t.Fatalf("normal read = %q, %v", body, err)
		}
	}
	for _, scenario := range []struct {
		size   int
		length int64
	}{
		{maxCloudConfigBytes, 1}, {maxCloudConfigBytes + 1, 1},
		{maxCloudConfigBytes, maxCloudConfigBytes}, {maxCloudConfigBytes + 1, maxCloudConfigBytes},
		{1024, 512}, {512, 1024}, {0, 1024},
	} {
		t.Run(fmt.Sprintf("size=%d/length=%d", scenario.size, scenario.length), func(t *testing.T) {
			size := scenario.size
			input := strings.Repeat("x", size)
			source := strings.NewReader(input)
			reader := cloudReaderFunc(func(buffer []byte) (int, error) {
				if len(buffer) > 32<<10 {
					t.Fatalf("read buffer = %d, exceeds cancellation batch", len(buffer))
				}
				return source.Read(buffer)
			})
			body, err := readCloudConfigHTTPResponseBodyContext(ctx, &http.Response{Body: io.NopCloser(reader), ContentLength: scenario.length})
			if size <= maxCloudConfigBytes {
				if err != nil || string(body) != input {
					t.Fatalf("limit-sized read: length=%d error=%v", len(body), err)
				}
			} else if err == nil || body != nil {
				t.Fatal("oversized actual body accepted")
			}
		})
	}
	failed, stop := context.WithCancel(context.Background())
	defer stop()
	body, err := readCloudConfigBodyContext(failed, cloudReaderFunc(func(buffer []byte) (int, error) {
		stop()
		return copy(buffer, "partial"), io.ErrUnexpectedEOF
	}))
	if body != nil || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("original read error lost: body=%q error=%v", body, err)
	}
	for _, expired := range []bool{false, true} {
		finished, finish := context.WithCancel(context.Background())
		finish()
		if expired {
			finished, finish = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		}
		reader := cloudReaderFunc(func([]byte) (int, error) { t.Fatal("finished context read the body"); return 0, io.EOF })
		body, err := readCloudConfigHTTPResponseBodyContext(finished, &http.Response{Body: io.NopCloser(reader), ContentLength: 1})
		finish()
		if body != nil || !errors.Is(err, finished.Err()) {
			t.Fatalf("finished read = %q, %v", body, err)
		}
	}
}

func (body *cloudReadEventBody) Read(buffer []byte) (int, error) {
	body.reads++
	n := copy(buffer, body.data)
	body.data = body.data[n:]
	body.cancel()
	if body.withEOF || len(body.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}

func (body *cloudReadEventBody) Close() error { body.closes++; return nil }

type cloudReadTransport func(*http.Request) (*http.Response, error)

func (transport cloudReadTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestCloudDownloadCancellationSkipsJSONAndClosesBody(t *testing.T) {
	withGlobalConfigState(t, func() {
		before := ConfigSnapshot()
		for _, scenario := range []struct {
			name, data          string
			beforeRead, withEOF bool
			length              int64
		}{
			{"before read", `{broken`, true, false, -1},
			{"during read", strings.Repeat("x", 100000), false, false, -1},
			{"final invalid JSON", `{broken`, false, true, -1},
			{"final valid JSON", `{"Name":"downloaded"}`, false, true, -1},
			{"before preallocation", `{broken`, true, false, maxCloudConfigBytes},
			{"during preallocated read", strings.Repeat("x", 100000), false, false, maxCloudConfigBytes},
			{"preallocated final JSON", `{"Name":"downloaded"}`, false, true, 1024},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				body := &cloudReadEventBody{data: scenario.data, cancel: cancel, withEOF: scenario.withEOF}
				HttpClient = &http.Client{Transport: cloudReadTransport(func(*http.Request) (*http.Response, error) {
					if scenario.beforeRead {
						cancel()
					}
					return &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: scenario.length}, nil
				})}
				got, err := SaveCloudConfigContext(ctx, "http://example.test/config")
				if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, Config{}) {
					t.Errorf("canceled download parsed or returned data: name length=%d error=%v", len(got.Name), err)
				}
				wantReads := 1
				if scenario.beforeRead {
					wantReads = 0
				}
				if body.reads != wantReads || body.closes != 1 {
					t.Errorf("reads=%d closes=%d, want %d reads and one close", body.reads, body.closes, wantReads)
				}
				if !reflect.DeepEqual(before, ConfigSnapshot()) {
					t.Fatal("canceled download changed active configuration")
				}
			})
		}
	})
}
