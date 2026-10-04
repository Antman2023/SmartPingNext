package http

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"modernc.org/sqlite"
)

type contextJSONValue struct {
	value     any
	onMarshal func()
}

func (value contextJSONValue) MarshalJSON() ([]byte, error) {
	value.onMarshal()
	return json.Marshal(value.value)
}

func TestJSONContextSkipsEncodingWhenAlreadyFinished(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprintf("Expired=%t", expired), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			if expired {
				cancel()
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			} else {
				cancel()
			}
			defer cancel()
			encoded := false
			response := httptest.NewRecorder()
			renderJSONContext(ctx, response, contextJSONValue{value: strings.Repeat("large response", 4096),
				onMarshal: func() { encoded = true }})
			if encoded || response.Code != http.StatusInternalServerError || response.Body.String() != ctx.Err().Error()+"\n" {
				t.Fatalf("encoded=%t, status=%d, bytes=%d; want canceled error without encoding", encoded, response.Code, response.Body.Len())
			}
		})
	}
}

func TestJSONContextDiscardsEncodingCanceledBeforeSubmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	response := httptest.NewRecorder()
	renderJSONContext(ctx, response, contextJSONValue{value: strings.Repeat("large response", 4096), onMarshal: cancel})
	if response.Code != http.StatusInternalServerError || response.Body.String() != "context canceled\n" || response.Header().Get("Content-Length") != "" {
		t.Fatalf("status=%d, length=%q, bytes=%d; want only a cancellation error", response.Code, response.Header().Get("Content-Length"), response.Body.Len())
	}
}

func TestJSONContextPreservesSuccessAndMarshalFailures(t *testing.T) {
	value := map[string]string{"text": "节点 <&> \"\\"}
	want, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	renderJSONContext(context.Background(), response, value)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), want) || response.Header().Get("Content-Length") != fmt.Sprint(len(want)) {
		t.Fatalf("success status=%d, bytes=%s", response.Code, response.Body.Bytes())
	}
	response = httptest.NewRecorder()
	renderJSONContext(context.Background(), response, make(chan int))
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "json: unsupported type") {
		t.Fatalf("marshal failure status=%d, body=%s", response.Code, response.Body.String())
	}
}

var responseCancellationDriverID atomic.Uint64

type responseCancellationDriver struct {
	base           driver.Driver
	afterRowsClose func()
	afterRowsNext  func()
	afterCommit    func()
}

func (d *responseCancellationDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &responseCancellationConn{Conn: conn, afterRowsClose: d.afterRowsClose, afterRowsNext: d.afterRowsNext, afterCommit: d.afterCommit}, nil
}

type responseCancellationConn struct {
	driver.Conn
	afterRowsClose func()
	afterRowsNext  func()
	afterCommit    func()
}

func (c *responseCancellationConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}

func (c *responseCancellationConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
	if err != nil || (c.afterRowsClose == nil && c.afterRowsNext == nil) {
		return rows, err
	}
	return &responseCancellationRows{Rows: rows, afterClose: c.afterRowsClose, afterNext: c.afterRowsNext}, nil
}

func (c *responseCancellationConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, options)
	if err != nil || !options.ReadOnly || c.afterCommit == nil {
		return tx, err
	}
	return &responseCancellationTx{Tx: tx, afterCommit: c.afterCommit}, nil
}

type responseCancellationRows struct {
	driver.Rows
	afterClose func()
	afterNext  func()
}

func (r *responseCancellationRows) Next(values []driver.Value) error {
	err := r.Rows.Next(values)
	if err == nil && r.afterNext != nil {
		r.afterNext()
	}
	return err
}

func (r *responseCancellationRows) Close() error {
	err := r.Rows.Close()
	if r.afterClose != nil {
		r.afterClose()
	}
	return err
}

type responseCancellationTx struct {
	driver.Tx
	afterCommit func()
}

func (tx *responseCancellationTx) Commit() error {
	err := tx.Tx.Commit()
	if err == nil {
		tx.afterCommit()
	}
	return err
}

func TestReadAPIsRejectCancellationAfterDatabaseRead(t *testing.T) {
	for _, endpoint := range []string{"ping", "alert", "mapping"} {
		t.Run(endpoint, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var once sync.Once
			hook := func() { once.Do(cancel) }
			wrapped := &responseCancellationDriver{base: &sqlite.Driver{}}
			if endpoint == "alert" {
				wrapped.afterCommit = hook
			} else {
				wrapped.afterRowsClose = hook
			}
			name := fmt.Sprintf("response-cancellation-%d", responseCancellationDriverID.Add(1))
			sql.Register(name, wrapped)
			db, handler := historyReadFixtureWithDriver(t, name)
			start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			seedHistoryPing(t, db, start, 3)
			seedHistoryAlerts(t, db, 3)
			if _, err := db.Exec(`CREATE TABLE mappinglog (logtime TEXT, mapjson TEXT);
				INSERT INTO mappinglog VALUES ('2026-01-01 00:00','{"ctcc":[{"name":"广东","value":0}],"cucc":[],"cmcc":[]}')`); err != nil {
				t.Fatal(err)
			}
			paths := map[string]string{
				"ping":    "/api/ping.json?ip=192.0.2.1&starttime=2026-01-01+00%3A00&endtime=2026-01-01+00%3A02",
				"alert":   "/api/alert.json?date=2026-01-01",
				"mapping": "/api/mapping.json?d=2026-01-01+00%3A00",
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, paths[endpoint], nil).WithContext(ctx))
			if ctx.Err() == nil {
				t.Fatal("read completion did not cancel the request")
			}
			if response.Code != http.StatusInternalServerError || json.Valid(response.Body.Bytes()) {
				t.Fatalf("canceled read returned status=%d, body=%s", response.Code, response.Body.String())
			}
			if db.Stats().InUse != 0 {
				t.Fatal("canceled response retained the database connection")
			}
			var one int
			if err := db.QueryRow("SELECT 1").Scan(&one); err != nil || one != 1 {
				t.Fatalf("pool did not recover: value=%d err=%v", one, err)
			}
			response = httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, paths[endpoint], nil))
			if response.Code != http.StatusOK || !json.Valid(response.Body.Bytes()) {
				t.Fatalf("subsequent response status=%d, body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestReadAPIsWithoutRowsRespectCanceledContexts(t *testing.T) {
	_, handler := historyReadFixture(t)
	for _, path := range []string{"/api/config.json", "/api/topology.json"} {
		t.Run(path, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx))
			if response.Code != http.StatusInternalServerError || json.Valid(response.Body.Bytes()) {
				t.Fatalf("%s: status=%d, body=%s", path, response.Code, response.Body.String())
			}
		})
	}
}
