package http

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"smartping/src/g"
	"testing"
	"time"
)

func TestTopologyAPIDistinguishesUnknownHealthyAndAlert(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE pinglog (logtime TEXT, target TEXT, avgdelay TEXT, losspk TEXT)`); err != nil {
		t.Fatal(err)
	}
	oldDB := g.Db
	g.Db = db
	defer func() { g.Db = oldDB }()
	var rules []map[string]string
	for _, target := range []string{"unknown", "healthy", "alert"} {
		rules = append(rules, map[string]string{
			"Addr": target, "Thdchecksec": "600", "Thdoccnum": "1", "Thdavgdelay": "200", "Thdloss": "30",
		})
	}
	for target, delay := range map[string]string{"healthy": "20", "alert": "250"} {
		if _, err := db.Exec(`INSERT INTO pinglog VALUES (?, ?, ?, '0')`, time.Now().Format("2006-01-02 15:04"), target, delay); err != nil {
			t.Fatal(err)
		}
	}
	withProxyConfig(g.Config{Addr: "127.0.0.1", Network: map[string]g.NetworkMember{
		"127.0.0.1": {Topology: rules},
	}}, func() {
		withAuthMaps(map[string]bool{"127.0.0.1": true}, nil, func() {
			mux := http.NewServeMux()
			configApiRoutes(mux)
			request := httptest.NewRequest(http.MethodGet, "/api/topology.json", nil)
			request.RemoteAddr = "127.0.0.1:1234"
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
			var states map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &states); err != nil {
				t.Fatal(err)
			}
			for target, want := range map[string]string{"unknown": "unknown", "healthy": "true", "alert": "false"} {
				if states[target] != want {
					t.Errorf("%s = %q, want %q", target, states[target], want)
				}
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			response = httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("database error status = %d, want 500", response.Code)
			}
		})
	})
}
