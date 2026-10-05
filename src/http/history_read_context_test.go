package http

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"modernc.org/sqlite"
)

type historyReadContextDriver struct {
	wrapQuery func(string, driver.Rows) driver.Rows
}

func (d *historyReadContextDriver) Open(name string) (driver.Conn, error) {
	conn, err := (&sqlite.Driver{}).Open(name)
	if err != nil {
		return nil, err
	}
	return &historyReadContextConn{
		responseCancellationConn: &responseCancellationConn{Conn: conn},
		wrapQuery:                d.wrapQuery,
	}, nil
}

type historyReadContextConn struct {
	*responseCancellationConn
	wrapQuery func(string, driver.Rows) driver.Rows
}

func (c *historyReadContextConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return c.wrapQuery(query, rows), nil
}

// Arm cancellation at a row event, then deliver it at the next context check.
// This makes application batch boundaries independent of SQLite's cancellation
// goroutine and does not add production query hooks or timing sleeps.
type historyReadOnCheckContext struct {
	context.Context
	cancel      context.CancelFunc
	rows        *atomic.Int32
	cancelAfter int32
}

func (ctx *historyReadOnCheckContext) Err() error {
	if ctx.rows.Load() >= ctx.cancelAfter {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestHistoryReadsCheckCancellationBetweenRowBatches(t *testing.T) {
	for _, phase := range []string{"ping", "alert dates", "alert records"} {
		for _, cancelAfter := range []int32{1, 300} {
			for _, onCheck := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/After=%d/OnCheck=%t", phase, cancelAfter, onCheck), func(t *testing.T) {
					var enabled atomic.Bool
					var rowsRead, rowsClosed atomic.Int32
					base, cancel := context.WithCancel(context.Background())
					defer cancel()
					var ctx context.Context = base
					if onCheck {
						ctx = &historyReadOnCheckContext{Context: base, cancel: cancel, rows: &rowsRead, cancelAfter: cancelAfter}
					}
					name := fmt.Sprintf("history-batch-cancel-%d", responseCancellationDriverID.Add(1))
					sql.Register(name, &historyReadContextDriver{wrapQuery: func(query string, rows driver.Rows) driver.Rows {
						matches := (phase == "ping" && strings.HasPrefix(query, "SELECT logtime,maxdelay")) ||
							(phase == "alert dates" && query == alertDatesQuery) ||
							(phase == "alert records" && strings.HasPrefix(query, "select logtime,targetname,targetip,tracert"))
						if !enabled.Load() || !matches {
							return rows
						}
						return &responseCancellationRows{Rows: rows,
							afterNext: func() {
								if read := rowsRead.Add(1); !onCheck && read == cancelAfter {
									cancel()
								}
							},
							afterClose: func() { rowsClosed.Add(1) },
						}
					}})
					// A canceled SQL transaction can discard its connection. Keep the
					// fixture on disk so reopening the pool preserves schema and data.
					db, handler := historyReadFixtureWithDSN(t, name, filepath.Join(t.TempDir(), "history.db"))
					start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
					seedHistoryPing(t, db, start, 1024)
					seedHistoryAlerts(t, db, 1024)
					if phase == "alert dates" {
						if _, err := db.Exec(`WITH RECURSIVE dates(day,n) AS (
						SELECT '2020-01-01',0 UNION ALL SELECT date(day,'+1 day'),n+1 FROM dates WHERE n < 1023
					) INSERT INTO alertlog SELECT day || ' 00:00','archive','192.0.2.1','' FROM dates`); err != nil {
							t.Fatal(err)
						}
					}
					path, wantBody := "/api/alert.json?date=2026-01-01", "Read alert data failed\n"
					if phase == "ping" {
						path = "/api/ping.json?ip=192.0.2.1&starttime=2026-01-01+00%3A00&endtime=2026-01-02+00%3A00"
						wantBody = "Read ping data failed\n"
					} else if phase == "alert dates" {
						wantBody = "Read alert dates failed\n"
					}
					enabled.Store(true)
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx))
					enabled.Store(false)
					wantRows := int32(1 + ((cancelAfter-1+255)/256)*256)
					read := rowsRead.Load()
					if ctx.Err() != context.Canceled || read < cancelAfter || read > wantRows || (onCheck && read != wantRows) || response.Code != http.StatusInternalServerError || response.Body.String() != wantBody {
						t.Errorf("rows=%d want=%d, status=%d body=%q", rowsRead.Load(), wantRows, response.Code, response.Body.String())
					}
					// Waiting for a real query also joins any asynchronous SQL rollback.
					probe, stop := context.WithTimeout(context.Background(), time.Second)
					defer stop()
					var one int
					if err := db.QueryRowContext(probe, "SELECT 1").Scan(&one); err != nil || one != 1 {
						t.Fatalf("database connection did not recover: %v", err)
					}
					if rowsClosed.Load() != 1 || db.Stats().InUse != 0 {
						t.Fatalf("rows closed=%d, connections in use=%d", rowsClosed.Load(), db.Stats().InUse)
					}
					response = httptest.NewRecorder()
					handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
					if response.Code != http.StatusOK || !json.Valid(response.Body.Bytes()) {
						t.Fatalf("subsequent read failed: status=%d body=%s", response.Code, response.Body.String())
					}
				})
			}
		}
	}
}
