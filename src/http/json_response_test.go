package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"smartping/src/g"
	"strconv"
	"strings"
	"testing"
)

func TestRenderJsonPreservesEncodedLengthAndConnectionReuse(t *testing.T) {
	for _, text := range []string{"中<&>\\\"", strings.Repeat("中<&>\\\"", 8192)} {
		t.Run(strconv.Itoa(len(text))+"BytesInput", func(t *testing.T) {
			value := map[string]string{"text": text}
			want, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				RenderJson(w, value)
			}))
			defer server.Close()
			for attempt := 0; attempt < 2; attempt++ {
				reused := false
				request, err := http.NewRequest(http.MethodGet, server.URL, nil)
				if err != nil {
					t.Fatal(err)
				}
				request = request.WithContext(httptrace.WithClientTrace(request.Context(), &httptrace.ClientTrace{
					GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused },
				}))
				response, err := server.Client().Do(request)
				if err != nil {
					t.Fatal(err)
				}
				body, readErr := io.ReadAll(response.Body)
				closeErr := response.Body.Close()
				if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK || !bytes.Equal(body, want) {
					t.Fatalf("response status = %d, read error = %v, close error = %v, body length = %d; want %d original JSON bytes", response.StatusCode, readErr, closeErr, len(body), len(want))
				}
				if response.ContentLength != int64(len(want)) || len(response.TransferEncoding) != 0 {
					t.Fatalf("Content-Length = %d, Transfer-Encoding = %v; want %d and no chunked encoding", response.ContentLength, response.TransferEncoding, len(want))
				}
				if attempt > 0 && !reused {
					t.Fatal("complete JSON response did not allow connection reuse")
				}
			}
		})
	}
}

func TestNodeConfigAndProxyPreserveLargeJSONWithDeclaredLength(t *testing.T) {
	remoteMux := http.NewServeMux()
	configApiRoutes(remoteMux)
	remote := httptest.NewServer(remoteMux)
	defer remote.Close()
	proxy := httptest.NewServer(http.HandlerFunc(handleProxy))
	defer proxy.Close()
	cfg := proxyTestConfig(t, remote.URL)
	cfg.Name = strings.Repeat("SmartPing中<&>\\\"", 4096)
	cfg.Password = "private-config-password"
	withProxyConfig(cfg, func() {
		withAuthMaps(map[string]bool{"127.0.0.1": true}, nil, func() {
			fetch := func(target string) []byte {
				t.Helper()
				response, err := remote.Client().Get(target)
				if err != nil {
					t.Fatal(err)
				}
				body, readErr := io.ReadAll(response.Body)
				closeErr := response.Body.Close()
				if response.StatusCode != http.StatusOK || readErr != nil || closeErr != nil {
					t.Fatalf("response status = %d, read error = %v, close error = %v", response.StatusCode, readErr, closeErr)
				}
				if response.ContentLength != int64(len(body)) || len(response.TransferEncoding) != 0 {
					t.Fatalf("Content-Length = %d, body length = %d, Transfer-Encoding = %v", response.ContentLength, len(body), response.TransferEncoding)
				}
				if !strings.HasPrefix(response.Header.Get("Content-Type"), "application/json") {
					t.Fatalf("response Content-Type = %q", response.Header.Get("Content-Type"))
				}
				return body
			}
			direct := fetch(remote.URL + "/api/config.json")
			proxied := fetch(proxy.URL + "/api/proxy.json?g=" + url.QueryEscape(remote.URL+"/api/config.json"))
			if !bytes.Equal(direct, proxied) {
				t.Fatal("proxy changed the production node JSON response")
			}
			var result g.Config
			if err := json.Unmarshal(proxied, &result); err != nil {
				t.Fatal(err)
			}
			if result.Name != cfg.Name || result.Password != "" {
				t.Fatal("large config response changed text or exposed the config password")
			}
		})
	})
}
