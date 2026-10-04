package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"smartping/src/g"
	"testing"
)

type configCopyCancelContext struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (ctx *configCopyCancelContext) Err() error {
	ctx.checks++
	if ctx.checks == 40 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestConfigAPIRejectsCanceledCopyAndRecoversWithoutPassword(t *testing.T) {
	_, handler := historyReadFixture(t)
	config := validTestConfig()
	config.Password = "private-config-password"
	config.Chinamap = map[string]map[string][]string{"江苏": {"ctcc": make([]string, 8192)}}
	for i := range config.Chinamap["江苏"]["ctcc"] {
		config.Chinamap["江苏"]["ctcc"][i] = "192.0.2.1"
	}
	g.SetConfig(config)
	for _, duringCopy := range []bool{false, true} {
		base, cancel := context.WithCancel(context.Background())
		var ctx context.Context = base
		if duringCopy {
			ctx = &configCopyCancelContext{Context: base, cancel: cancel}
		} else {
			cancel()
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/config.json", nil).WithContext(ctx))
		cancel()
		if response.Code != http.StatusInternalServerError || response.Body.String() != "context canceled\n" {
			t.Fatalf("duringCopy=%t: status=%d, body=%s", duringCopy, response.Code, response.Body.String())
		}
		if g.ConfigSnapshot().Password != config.Password {
			t.Fatal("canceled request changed the stored password")
		}
	}
	want := g.ConfigSnapshot()
	want.Password = ""
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/config.json", nil).WithContext(ctx))
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), wantJSON) || bytes.Contains(response.Body.Bytes(), []byte(config.Password)) {
		t.Fatalf("subsequent configuration response: status=%d, bytes=%d", response.Code, response.Body.Len())
	}
}
