package funcs

import (
	"context"
	"database/sql"
	"errors"
	"smartping/src/g"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type observedStorageContext struct {
	context.Context
	once        sync.Once
	checked     chan error
	waitingOnce sync.Once
	waiting     chan struct{}
}

func (ctx *observedStorageContext) Err() error {
	err := ctx.Context.Err()
	ctx.once.Do(func() { ctx.checked <- err })
	return err
}

func (ctx *observedStorageContext) Done() <-chan struct{} {
	ctx.waitingOnce.Do(func() { close(ctx.waiting) })
	return ctx.Context.Done()
}

func TestStorageContextExitsWhileDatabaseWriteLockIsHeld(t *testing.T) {
	schema := []string{
		`CREATE TABLE pinglog (logtime TEXT, target TEXT, maxdelay TEXT, mindelay TEXT, avgdelay TEXT, sendpk INTEGER, revcpk INTEGER, losspk INTEGER, UNIQUE(logtime, target));`,
		`CREATE TABLE alertlog (logtime TEXT, targetip TEXT, targetname TEXT, tracert TEXT, UNIQUE(logtime, targetip));`,
		`CREATE TABLE mappinglog (logtime TEXT UNIQUE, mapjson TEXT);`,
	}
	operations := []struct {
		name         string
		table        string
		returnsError bool
		archive      bool
		run          func(context.Context) error
	}{
		{"ping", "pinglog", false, false, func(ctx context.Context) error {
			PingStorageContext(ctx, g.PingSt{SendPk: 1, RevcPk: 1}, "192.0.2.1", "2026-10-04 12:00")
			return nil
		}},
		{"alert", "alertlog", true, false, func(ctx context.Context) error {
			return AlertStorageContext(ctx, g.AlertLog{Logtime: "2026-10-04 12:00", Targetip: "192.0.2.1", Tracert: "[]"})
		}},
		{"mapping", "mappinglog", true, false, MapPingStorageContext},
		{"archive batch", "pinglog", true, true, func(ctx context.Context) error {
			return clearArchiveTableBeforeContext(ctx, "pinglog", "2021-01-01")
		}},
		{"archive job", "", false, true, func(ctx context.Context) error {
			ClearArchiveContext(ctx)
			return nil
		}},
	}
	for _, operation := range operations {
		for _, deadline := range []bool{false, true} {
			name := operation.name + "/cancel"
			if deadline {
				name = operation.name + "/deadline"
			}
			t.Run(name, func(t *testing.T) {
				withFuncTestDB(t, schema, func(db *sql.DB) {
					for _, table := range []string{"pinglog", "alertlog", "mappinglog"} {
						if _, err := db.Exec("INSERT INTO " + table + " (logtime) VALUES ('2000-01-01')"); err != nil {
							t.Fatal(err)
						}
					}
					ctx, cancel := context.WithCancel(context.Background())
					wantErr := context.Canceled
					if deadline {
						cancel()
						ctx, cancel = context.WithTimeout(context.Background(), 150*time.Millisecond)
						wantErr = context.DeadlineExceeded
					}
					observed := &observedStorageContext{Context: ctx, checked: make(chan error, 1), waiting: make(chan struct{})}
					g.DLock.Lock()
					held := true
					done := make(chan error, 1)
					finished := false
					defer func() {
						cancel()
						if held {
							g.DLock.Unlock()
						}
						if !finished {
							select {
							case <-done:
							case <-time.After(time.Second):
								t.Error("storage worker did not stop after cleanup")
							}
						}
					}()
					go func() { done <- operation.run(observed) }()
					select {
					case err := <-observed.checked:
						if err != nil {
							t.Fatalf("storage started with an already expired context: %v", err)
						}
					case <-time.After(time.Second):
						t.Fatal("storage did not start")
					}
					select {
					case <-observed.waiting:
					case <-time.After(time.Second):
						t.Fatal("storage did not enter the cancellable lock queue")
					}
					if !deadline {
						cancel()
					}
					select {
					case err := <-done:
						finished = true
						if operation.returnsError && !errors.Is(err, wantErr) {
							t.Errorf("storage error = %v, want %v", err, wantErr)
						}
					case <-time.After(500 * time.Millisecond):
						t.Error("canceled storage waited for another database writer to release its lock")
					}
					g.DLock.Unlock()
					held = false
					if !finished {
						select {
						case <-done:
							finished = true
						case <-time.After(time.Second):
							t.Fatal("storage did not exit after releasing the lock")
						}
					}
					if operation.name == "archive job" && atomic.LoadInt32(&archiveRunning) != 0 {
						t.Fatal("canceled archive retained its running guard")
					}
					for _, table := range []string{"pinglog", "alertlog", "mappinglog"} {
						var count int
						if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil {
							t.Fatal(err)
						}
						if count != 1 {
							t.Fatalf("canceled storage modified %s: %d rows, want 1", table, count)
						}
					}
					// A subsequent uncanceled operation must reuse the lock and persist
					// or delete its records normally after the queued work was canceled.
					if err := operation.run(context.Background()); err != nil {
						t.Fatal(err)
					}
					for _, table := range []string{"pinglog", "alertlog", "mappinglog"} {
						want := 1
						if operation.table == table {
							want = 2
						}
						if operation.archive && (operation.table == table || operation.table == "") {
							want = 0
						}
						var count int
						if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil {
							t.Fatal(err)
						}
						if count != want {
							t.Fatalf("next operation left %s with %d rows, want %d", table, count, want)
						}
					}
				})
			})
		}
	}
}
