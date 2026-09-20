package http

import (
	"net/http"
	"net/http/httptest"
	"smartping/src/static"
	"strings"
	"testing"
)

func TestStaticFileCachePolicyTracksResponseStatus(t *testing.T) {
	entries, err := static.Files.ReadDir("html/assets")
	if err != nil {
		t.Fatal(err)
	}
	asset := ""
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".js") {
			asset = "/assets/" + entry.Name()
			break
		}
	}
	if asset == "" {
		t.Fatal("no embedded JavaScript asset found")
	}
	withAuthMaps(nil, nil, func() {
		mux := http.NewServeMux()
		configIndexRoutes(mux)
		for _, tt := range []struct {
			name, path, method, byteRange, cache string
			status                               int
		}{
			{"asset", asset, http.MethodGet, "", immutableAssetCacheControl, http.StatusOK},
			{"asset HEAD", asset, http.MethodHead, "", immutableAssetCacheControl, http.StatusOK},
			{"partial asset", asset, http.MethodGet, "bytes=0-0", immutableAssetCacheControl, http.StatusPartialContent},
			{"invalid range", asset, http.MethodGet, "bytes=999999999999-", "no-store", http.StatusRequestedRangeNotSatisfiable},
			{"missing asset", "/assets/not-present.js", http.MethodGet, "", "no-store", http.StatusNotFound},
			{"missing file HEAD", "/not-present.ico", http.MethodHead, "", "no-store", http.StatusNotFound},
			{"SPA", "/config", http.MethodGet, "", indexCacheControl, http.StatusOK},
		} {
			t.Run(tt.name, func(t *testing.T) {
				request := httptest.NewRequest(tt.method, tt.path, nil)
				if tt.byteRange != "" {
					request.Header.Set("Range", tt.byteRange)
				}
				response := httptest.NewRecorder()
				mux.ServeHTTP(response, request)
				if response.Code != tt.status {
					t.Fatalf("status = %d, want %d", response.Code, tt.status)
				}
				if cache := response.Header().Get("Cache-Control"); cache != tt.cache {
					t.Fatalf("Cache-Control = %q, want %q", cache, tt.cache)
				}
			})
		}
	})
}
