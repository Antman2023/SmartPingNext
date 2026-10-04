package funcs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func withIndexedAlertTestDB(t *testing.T, fn func(*sql.DB)) {
	t.Helper()
	withFuncTestDB(t, []string{
		`CREATE TABLE pinglog (logtime TEXT, target TEXT, avgdelay TEXT, losspk TEXT, UNIQUE(logtime, target))`,
		`CREATE INDEX pinglog_target_logtime ON pinglog(target, logtime)`,
	}, func(db *sql.DB) {
		db.SetMaxOpenConns(1)
		fn(db)
	})
}

func TestAlertStatusQueriesPreserveWindowAndThresholdBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 30, 0, time.Local)
	type sample struct {
		minute int
		delay  string
		loss   string
		target string
	}
	for _, seconds := range []int{1, 59, 60, 61, 600, 35940, 36000, 36059, 86400} {
		t.Run(fmt.Sprintf("%dSeconds", seconds), func(t *testing.T) {
			withIndexedAlertTestDB(t, func(db *sql.DB) {
				limit := max(seconds/60, 1)
				denseHealthy := make([]sample, limit)
				denseBad := make([]sample, limit)
				for i := range denseHealthy {
					denseHealthy[i] = sample{minute: -i, delay: "20", loss: "0"}
					denseBad[i] = sample{minute: -i, delay: "300", loss: "0"}
				}
				cases := []struct {
					name      string
					samples   []sample
					occ       int
					avg, loss string
					healthy   bool
					noSamples bool
				}{
					{name: "missing", noSamples: true},
					{name: "zero measurement", samples: []sample{{0, "0", "0", ""}}, healthy: true},
					{name: "delay equal is healthy", samples: []sample{{0, "200", "0", ""}}, healthy: true},
					{name: "delay strictly greater alerts", samples: []sample{{0, "200.01", "0", ""}}},
					{name: "loss equal alerts", samples: []sample{{0, "0", "30", ""}}},
					{name: "loss just below is healthy", samples: []sample{{0, "200", "29.99", ""}}, healthy: true},
					{name: "decimal threshold equal is healthy", samples: []sample{{0, "200.25", "29.49", ""}}, avg: "200.25", loss: "29.5", healthy: true},
					{name: "decimal threshold exceeded alerts", samples: []sample{{0, "200.26", "29.49", ""}}, avg: "200.25", loss: "29.5"},
					{name: "decimal loss equal alerts", samples: []sample{{0, "0", "29.5", ""}}, loss: "29.5"},
					{name: "zero loss threshold alerts", samples: []sample{{0, "0", "0", ""}}, loss: "0"},
					{name: "grace sample counts", samples: []sample{{-limit, "300", "0", ""}}},
					{name: "expired samples are unknown", samples: []sample{{-limit - 1, "300", "100", ""}}, noSamples: true},
					{name: "future samples are unknown", samples: []sample{{1, "300", "100", ""}}, noSamples: true},
					{name: "other target is unknown", samples: []sample{{0, "300", "100", "other"}}, noSamples: true},
					{name: "bad grace cannot exceed latest sample cap", samples: append(append([]sample(nil), denseHealthy...), sample{-limit, "300", "100", ""}), healthy: true},
					{name: "two bad samples at occurrence threshold", samples: []sample{{0, "300", "0", ""}, {-1, "0", "30", ""}}, occ: 2, healthy: limit < 2},
					{name: "occurrences below threshold are healthy", samples: []sample{{0, "300", "0", ""}, {-1, "0", "30", ""}}, occ: 3, healthy: true},
					{name: "delay and loss on one row count once", samples: []sample{{0, "300", "100", ""}, {-1, "0", "0", ""}}, occ: 2, healthy: true},
					{name: "nonadjacent bad samples reach threshold", samples: []sample{{0, "300", "0", ""}, {-10, "0", "30", ""}, {-300, "300", "0", ""}}, occ: 3, healthy: limit < 300},
					{name: "all samples meet occurrence threshold", samples: denseBad, occ: limit},
					{name: "legacy occurrence count beyond sample cap", samples: denseBad, occ: limit + 1, healthy: true},
					{name: "out of window failures do not contaminate healthy", samples: []sample{{0, "20", "0", ""}, {1, "300", "100", ""}, {-limit - 1, "300", "100", ""}}, healthy: true},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						occ := max(tc.occ, 1)
						avg, loss := tc.avg, tc.loss
						if avg == "" {
							avg = "200"
						}
						if loss == "" {
							loss = "30"
						}
						rule := map[string]string{"Addr": tc.name, "Thdchecksec": strconv.Itoa(seconds), "Thdoccnum": strconv.Itoa(occ), "Thdavgdelay": avg, "Thdloss": loss}
						tx, err := db.Begin()
						if err != nil {
							t.Fatal(err)
						}
						defer tx.Rollback()
						for _, row := range tc.samples {
							target := tc.name
							if row.target != "" {
								target = row.target
							}
							if _, err := tx.Exec(`INSERT INTO pinglog VALUES (?, ?, ?, ?)`, now.Add(time.Duration(row.minute)*time.Minute).Format("2006-01-02 15:04"), target, row.delay, row.loss); err != nil {
								t.Fatal(err)
							}
						}
						if err := tx.Commit(); err != nil {
							t.Fatal(err)
						}
						healthy, err := checkAlertStatusAtContext(context.Background(), rule, now)
						if healthy != tc.healthy || errors.Is(err, ErrNoAlertSamples) != tc.noSamples || err != nil && !tc.noSamples {
							t.Fatalf("automatic status=%v, err=%v; want healthy=%v, noSamples=%v", healthy, err, tc.healthy, tc.noSamples)
						}
						args := []any{avg, loss, tc.name,
							now.Add(-time.Duration(limit) * time.Minute).Format("2006-01-02 15:04"), now.Format("2006-01-02 15:04"), limit}
						var total, bad int
						if err := db.QueryRow(aggregateAlertStatusReferenceQuery, args...).Scan(&total, &bad); err != nil {
							t.Fatal(err)
						}
						var state int
						if err := db.QueryRow(alertStatusEarlyExitQuery, append(args, occ-1)...).Scan(&state); err != nil {
							t.Fatal(err)
						}
						wantState := 0
						if tc.noSamples {
							wantState = -1
						} else if tc.healthy {
							wantState = 1
						}
						if state != wantState || (total == 0) != tc.noSamples || (total > 0 && bad < occ) != tc.healthy {
							t.Fatalf("early state=%d, reference samples=%d/bad=%d; want state=%d", state, total, bad, wantState)
						}
					})
				}
			})
		})
	}
}

func TestLongWindowAlertStatusErrorsRemainDistinct(t *testing.T) {
	withIndexedAlertTestDB(t, func(db *sql.DB) {
		rule := map[string]string{"Addr": "192.0.2.1", "Thdchecksec": "36000", "Thdoccnum": "1", "Thdavgdelay": "200", "Thdloss": "30"}
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		expired, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer expire()
		for _, tc := range []struct {
			name string
			ctx  context.Context
			want error
		}{{"canceled", canceled, context.Canceled}, {"deadline", expired, context.DeadlineExceeded}} {
			t.Run(tc.name, func(t *testing.T) {
				healthy, err := CheckAlertStatusContext(tc.ctx, rule)
				if healthy || !errors.Is(err, tc.want) || errors.Is(err, ErrNoAlertSamples) {
					t.Fatalf("status=%v, err=%v; want %v", healthy, err, tc.want)
				}
			})
		}
		for _, key := range []string{"Thdchecksec", "Thdoccnum"} {
			original := rule[key]
			for _, invalid := range []string{"0", "-1", "abc"} {
				rule[key] = invalid
				healthy, err := CheckAlertStatusContext(context.Background(), rule)
				if healthy || err == nil || !strings.Contains(err.Error(), "invalid "+key) {
					t.Fatalf("%s=%q: status=%v, err=%v", key, invalid, healthy, err)
				}
			}
			rule[key] = original
		}
		if _, err := db.Exec(`DROP TABLE pinglog`); err != nil {
			t.Fatal(err)
		}
		healthy, err := CheckAlertStatusContext(context.Background(), rule)
		if healthy || err == nil || errors.Is(err, ErrNoAlertSamples) || !strings.Contains(err.Error(), rule["Addr"]) {
			t.Fatalf("database error: status=%v, err=%v", healthy, err)
		}
	})
}

func TestAlertStatusCancellationWhileWaitingForDatabaseConnection(t *testing.T) {
	for _, seconds := range []string{"600", "36000"} {
		t.Run(seconds, func(t *testing.T) {
			withIndexedAlertTestDB(t, func(db *sql.DB) {
				conn, err := db.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				before := db.Stats().WaitCount
				go func() {
					_, err := CheckAlertStatusContext(ctx, map[string]string{
						"Addr": "192.0.2.1", "Thdchecksec": seconds, "Thdoccnum": "1", "Thdavgdelay": "200", "Thdloss": "30",
					})
					done <- err
				}()
				deadline := time.NewTimer(time.Second)
				defer deadline.Stop()
				ticker := time.NewTicker(time.Millisecond)
				defer ticker.Stop()
				for db.Stats().WaitCount == before {
					select {
					case <-ticker.C:
					case <-deadline.C:
						cancel()
						<-done
						t.Fatal("query did not wait for the occupied connection")
					}
				}
				cancel()
				select {
				case err := <-done:
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("query error=%v; want context canceled", err)
					}
				case <-time.After(time.Second):
					_ = conn.Close()
					<-done
					t.Fatal("cancellation did not release the waiting query")
				}
				if err := conn.Close(); err != nil {
					t.Fatal(err)
				}
				if err := db.PingContext(context.Background()); err != nil {
					t.Fatalf("connection pool did not recover: %v", err)
				}
			})
		})
	}
}

func TestLongWindowAlertQueryUsesTargetTimeIndex(t *testing.T) {
	withIndexedAlertTestDB(t, func(db *sql.DB) {
		rows, err := db.Query("EXPLAIN QUERY PLAN "+alertStatusEarlyExitQuery,
			"200", "30", "192.0.2.1", "2026-10-04 02:00", "2026-10-04 12:00", 600, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		searches := 0
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(detail, "SEARCH pinglog") && strings.Contains(detail, "pinglog_target_logtime") {
				searches++
			}
			if strings.Contains(detail, "SCAN pinglog") || strings.Contains(detail, "TEMP B-TREE") {
				t.Fatalf("query scans or sorts the full history: %s", detail)
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if searches != 2 {
			t.Fatalf("indexed target/time searches=%d; want 2", searches)
		}
	})
}
