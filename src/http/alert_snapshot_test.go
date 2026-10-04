package http

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"smartping/src/g"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var alertSnapshotDriverID atomic.Uint64

// Intercept closing the date cursor to commit a write from a second SQLite
// connection before the endpoint starts reading the selected day's records.
// This introduces no timing sleeps or production query hooks.
type alertSnapshotDriver struct {
	base    driver.Driver
	onClose func()
}

func (d *alertSnapshotDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &alertSnapshotConn{Conn: conn, onClose: d.onClose}, nil
}

type alertSnapshotConn struct {
	driver.Conn
	onClose func()
}

func (c *alertSnapshotConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, options)
}

func (c *alertSnapshotConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}

func (c *alertSnapshotConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	if query == alertDatesQuery {
		return &alertSnapshotRows{Rows: rows, onClose: c.onClose}, nil
	}
	return rows, nil
}

type alertSnapshotRows struct {
	driver.Rows
	onClose func()
}

func (r *alertSnapshotRows) Close() error {
	err := r.Rows.Close()
	r.onClose()
	return err
}

func alertSnapshotTestEndpoint(t *testing.T, journalMode string, onDateClose func(*sql.DB)) (*sql.DB, *sql.DB, http.Handler) {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "alerts.db")
	writer, err := sql.Open("sqlite", filename)
	if err != nil {
		t.Fatal(err)
	}
	writer.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = writer.Close() })
	var mode string
	if err := writer.QueryRow("PRAGMA journal_mode=" + journalMode).Scan(&mode); err != nil || mode != journalMode {
		t.Fatalf("enable fixture journal: mode=%q, want=%q, err=%v", mode, journalMode, err)
	}
	if _, err := writer.Exec(`CREATE TABLE alertlog (logtime TEXT, targetname TEXT, targetip TEXT, tracert TEXT, UNIQUE(logtime, targetip));
		CREATE INDEX alertlog_date ON alertlog(date(logtime));
		CREATE INDEX alertlog_logtime ON alertlog(logtime)`); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	name := fmt.Sprintf("alert-snapshot-%d", alertSnapshotDriverID.Add(1))
	sql.Register(name, &alertSnapshotDriver{base: &sqlite.Driver{}, onClose: func() {
		once.Do(func() { onDateClose(writer) })
	}})
	reader, err := sql.Open(name, filename)
	if err != nil {
		t.Fatal(err)
	}
	reader.SetMaxOpenConns(1)
	oldDB, oldConfig := g.Db, g.ConfigSnapshot()
	g.Db = reader
	g.SetConfig(g.Config{Name: "local", Addr: "127.0.0.1", Network: map[string]g.NetworkMember{}})
	t.Cleanup(func() {
		_ = reader.Close()
		g.Db = oldDB
		g.SetConfig(oldConfig)
	})
	return reader, writer, newAppHandler()
}

func readAlertSnapshotResponse(t *testing.T, handler http.Handler, ctx context.Context) ([]string, []g.AlertLog) {
	t.Helper()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/alert.json?date=2026-09-02", nil).WithContext(ctx)
	handler.ServeHTTP(response, request)
	return decodeAlertSnapshotResponse(t, response)
}

func decodeAlertSnapshotResponse(t *testing.T, response *httptest.ResponseRecorder) ([]string, []g.AlertLog) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d, body=%s", response.Code, response.Body.String())
	}
	var sections []json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &sections); err != nil || len(sections) != 2 {
		t.Fatalf("invalid response sections=%d, err=%v", len(sections), err)
	}
	var dates []string
	var records []g.AlertLog
	if err := json.Unmarshal(sections[0], &dates); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(sections[1], &records); err != nil {
		t.Fatal(err)
	}
	return dates, records
}

func TestAlertEndpointReadsDatesAndRecordsFromOneSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name            string
		initialSelected bool
		mutation        string
	}{
		{"insert first record on selected day", false, `INSERT INTO alertlog VALUES ('2026-09-02 12:00', 'target', '192.0.2.2', '[]')`},
		{"archive removes selected day", true, `DELETE FROM alertlog WHERE logtime >= '2026-09-02'`},
		{"upsert changes existing record", true, `INSERT INTO alertlog VALUES ('2026-09-02 12:00', 'updated', '192.0.2.2', 'new trace')
			ON CONFLICT(logtime, targetip) DO UPDATE SET targetname=excluded.targetname, tracert=excluded.tracert`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := make(chan error, 1)
			reader, writer, handler := alertSnapshotTestEndpoint(t, "wal", func(writer *sql.DB) {
				_, err := writer.Exec(tc.mutation)
				mutated <- err
			})
			if _, err := writer.Exec(`INSERT INTO alertlog VALUES ('2026-09-01 12:00', 'earlier', '192.0.2.1', '[]')`); err != nil {
				t.Fatal(err)
			}
			wantDates := []string{"2026-09-01"}
			wantRecords := []g.AlertLog{}
			if tc.initialSelected {
				if _, err := writer.Exec(`INSERT INTO alertlog VALUES ('2026-09-02 12:00', 'original', '192.0.2.2', 'old trace')`); err != nil {
					t.Fatal(err)
				}
				wantDates = []string{"2026-09-02", "2026-09-01"}
				wantRecords = []g.AlertLog{{Logtime: "2026-09-02 12:00", Targetname: "original", Targetip: "192.0.2.2", Tracert: "old trace", Fromname: "local", Fromip: "127.0.0.1"}}
			}
			dates, records := readAlertSnapshotResponse(t, handler, context.Background())
			select {
			case err := <-mutated:
				if err != nil {
					t.Fatalf("write between queries failed: %v", err)
				}
			default:
				t.Fatal("write was not committed between date and data queries")
			}
			if !reflect.DeepEqual(dates, wantDates) || !reflect.DeepEqual(records, wantRecords) {
				t.Errorf("mixed response: dates=%v records=%+v; want dates=%v records=%+v", dates, records, wantDates, wantRecords)
			}
			if reader.Stats().InUse != 0 {
				t.Fatal("completed snapshot retained its database connection")
			}
			dates, records = readAlertSnapshotResponse(t, handler, context.Background())
			if tc.name == "archive removes selected day" {
				if !reflect.DeepEqual(dates, []string{"2026-09-01"}) || len(records) != 0 {
					t.Fatalf("next request did not see deletion: %v %+v", dates, records)
				}
			} else {
				if !reflect.DeepEqual(dates, []string{"2026-09-02", "2026-09-01"}) || len(records) != 1 {
					t.Fatalf("next request did not see committed write: %v %+v", dates, records)
				}
				if tc.name == "upsert changes existing record" && (records[0].Targetname != "updated" || records[0].Tracert != "new trace") {
					t.Fatalf("updated record not visible: %+v", records[0])
				}
			}
		})
	}
}

func TestAlertEndpointCancellationBetweenQueriesReleasesSnapshot(t *testing.T) {
	for _, mode := range []string{"wal", "delete"} {
		t.Run(mode, func(t *testing.T) {
			testAlertEndpointCancellationBetweenQueriesReleasesSnapshot(t, mode)
		})
	}
}

func testAlertEndpointCancellationBetweenQueriesReleasesSnapshot(t *testing.T, journalMode string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	closed := make(chan struct{})
	reader, writer, handler := alertSnapshotTestEndpoint(t, journalMode, func(_ *sql.DB) {
		cancel()
		close(closed)
	})
	if _, err := writer.Exec(`INSERT INTO alertlog VALUES ('2026-09-02 12:00', 'target', '192.0.2.2', '[]')`); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/alert.json?date=2026-09-02", nil).WithContext(ctx)
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("canceled status=%d, body=%s; want 500 without partial JSON", response.Code, response.Body.String())
	}
	select {
	case <-closed:
	default:
		t.Fatal("date cursor did not close before cancellation")
	}
	checkCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	var count int
	if err := reader.QueryRowContext(checkCtx, `SELECT count(*) FROM alertlog`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("snapshot did not release its sole connection: count=%d, err=%v", count, err)
	}
	if _, err := writer.Exec(`INSERT INTO alertlog VALUES ('2026-09-02 13:00', 'next', '192.0.2.3', '[]')`); err != nil {
		t.Fatal(err)
	}
	_, records := readAlertSnapshotResponse(t, handler, context.Background())
	if len(records) != 2 {
		t.Fatalf("uncanceled request did not recover: %+v", records)
	}
}

func TestAlertEndpointDefaultJournalReleasesReadLockBeforeReturning(t *testing.T) {
	const mutation = `INSERT INTO alertlog VALUES ('2026-09-02 12:00', 'new', '192.0.2.2', '[]')`
	attempted := make(chan error, 1)
	reader, writer, handler := alertSnapshotTestEndpoint(t, "delete", func(writer *sql.DB) {
		_, err := writer.Exec(mutation)
		attempted <- err
	})
	if _, err := writer.Exec(`INSERT INTO alertlog VALUES ('2026-09-01 12:00', 'earlier', '192.0.2.1', '[]')`); err != nil {
		t.Fatal(err)
	}
	writing := make(chan error, 1)
	response := &alertSnapshotWriteProbe{ResponseRecorder: httptest.NewRecorder(), beforeWrite: func() {
		_, err := writer.Exec(mutation)
		writing <- err
	}}
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/alert.json?date=2026-09-02", nil))
	dates, records := decodeAlertSnapshotResponse(t, response.ResponseRecorder)
	select {
	case err := <-attempted:
		var sqliteErr *sqlite.Error
		if !errors.As(err, &sqliteErr) || sqliteErr.Code() != sqlite3.SQLITE_BUSY {
			t.Fatalf("writer did not wait for the read snapshot: %v", err)
		}
	default:
		t.Fatal("concurrent write was not attempted")
	}
	if !reflect.DeepEqual(dates, []string{"2026-09-01"}) || len(records) != 0 {
		t.Fatalf("default journal returned mixed data: %v %+v", dates, records)
	}
	if reader.Stats().InUse != 0 {
		t.Fatal("response retained its read transaction")
	}
	select {
	case err := <-writing:
		if err != nil {
			t.Fatalf("response writing retained its read lock: %v", err)
		}
	default:
		t.Fatal("response writer did not attempt the post-snapshot write")
	}
	dates, records = readAlertSnapshotResponse(t, handler, context.Background())
	if !reflect.DeepEqual(dates, []string{"2026-09-02", "2026-09-01"}) || len(records) != 1 || records[0].Targetname != "new" {
		t.Fatalf("next response did not see the write after snapshot release: %v %+v", dates, records)
	}
}

type alertSnapshotWriteProbe struct {
	*httptest.ResponseRecorder
	beforeWrite func()
	once        sync.Once
}

func (w *alertSnapshotWriteProbe) WriteHeader(status int) {
	w.once.Do(w.beforeWrite)
	w.ResponseRecorder.WriteHeader(status)
}
