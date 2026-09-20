package funcs

import (
	"context"
	"database/sql"
	"errors"
	"smartping/src/g"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClearArchiveContinuesAfterIndependentTableFailures(t *testing.T) {
	withFuncTestDB(t, []string{`CREATE TABLE pinglog (logtime TEXT);`}, func(db *sql.DB) {
		if _, err := db.Exec(`INSERT INTO pinglog(logtime) VALUES (?), (?), (?)`,
			"2020-12-31 23:59", "2021-01-01 00:00", "2022-01-01 00:00"); err != nil {
			t.Fatal(err)
		}
		err := clearArchiveBefore("2021-01-01")
		if err == nil || !strings.Contains(err.Error(), "alertlog") || !strings.Contains(err.Error(), "mappinglog") {
			t.Fatalf("expected both failed table names, got %v", err)
		}
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM pinglog`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("remaining rows = %d, want cutoff and newer rows retained", count)
		}
	})
}

func TestClearArchiveBeforeCancellationPreservesRows(t *testing.T) {
	withFuncTestDB(t, []string{`CREATE TABLE pinglog (logtime TEXT);`}, func(db *sql.DB) {
		if _, err := db.Exec(`INSERT INTO pinglog(logtime) VALUES ('2020-01-01')`); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := clearArchiveBeforeContext(ctx, "2021-01-01"); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM pinglog`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("canceled cleanup left %d rows, want 1", count)
		}
	})
}

func TestClearArchiveContextDoesNotStartWhenCanceled(t *testing.T) {
	atomic.StoreInt32(&archiveRunning, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ClearArchiveContext(ctx)
	if got := atomic.LoadInt32(&archiveRunning); got != 0 {
		t.Fatalf("archiveRunning = %d after canceled call, want 0", got)
	}
}

func TestClearArchiveSkipsOverlappingRun(t *testing.T) {
	atomic.StoreInt32(&archiveRunning, 1)
	defer atomic.StoreInt32(&archiveRunning, 0)

	ClearArchive()
	if got := atomic.LoadInt32(&archiveRunning); got != 1 {
		t.Fatalf("archiveRunning = %d, want existing cleanup to remain active", got)
	}
}

func TestClearArchiveRunsAndReleasesGuard(t *testing.T) {
	withFuncTestDB(t, []string{
		`CREATE TABLE alertlog (logtime TEXT);`,
		`CREATE TABLE mappinglog (logtime TEXT);`,
		`CREATE TABLE pinglog (logtime TEXT);`,
	}, func(db *sql.DB) {
		g.Cfg.Base = map[string]int{"Archive": 30}
		for _, table := range []string{"alertlog", "mappinglog", "pinglog"} {
			_, _ = db.Exec("INSERT INTO "+table+"(logtime) VALUES (?), (?)", "2000-01-01", "2999-01-01")
		}

		ClearArchive()
		if got := atomic.LoadInt32(&archiveRunning); got != 0 {
			t.Fatalf("archiveRunning = %d after success, want 0", got)
		}
		for _, table := range []string{"alertlog", "mappinglog", "pinglog"} {
			var count int
			if err := db.QueryRow("SELECT count(1) FROM " + table).Scan(&count); err != nil {
				t.Fatalf("query %s failed: %v", table, err)
			}
			if count != 1 {
				t.Fatalf("%s row count = %d, want 1", table, count)
			}
		}
	})
}

func TestClearArchiveReleasesGuardAfterFailure(t *testing.T) {
	withFuncTestDB(t, []string{
		`CREATE TABLE alertlog (logtime TEXT);`,
	}, func(_ *sql.DB) {
		g.Cfg.Base = map[string]int{"Archive": 30}
		ClearArchive()
		if got := atomic.LoadInt32(&archiveRunning); got != 0 {
			t.Fatalf("archiveRunning = %d after failure, want 0", got)
		}
	})
}

func TestClearArchiveBeforeReturnsFailure(t *testing.T) {
	withFuncTestDB(t, []string{
		`CREATE TABLE alertlog (logtime TEXT);`,
		`CREATE TABLE mappinglog (logtime TEXT);`,
	}, func(db *sql.DB) {
		_, _ = db.Exec(`INSERT INTO alertlog(logtime) VALUES (?)`, "2020-01-01")
		_, _ = db.Exec(`INSERT INTO mappinglog(logtime) VALUES (?)`, "2020-01-01")

		if err := clearArchiveBefore("2021-01-01"); err == nil {
			t.Fatalf("clearArchiveBefore should fail when a table is missing")
		}
	})
}

func TestClearArchiveBeforeCommitsAllDeletes(t *testing.T) {
	withFuncTestDB(t, []string{
		`CREATE TABLE alertlog (logtime TEXT);`,
		`CREATE TABLE mappinglog (logtime TEXT);`,
		`CREATE TABLE pinglog (logtime TEXT);`,
	}, func(db *sql.DB) {
		for _, table := range []string{"alertlog", "mappinglog", "pinglog"} {
			_, _ = db.Exec("INSERT INTO "+table+"(logtime) VALUES (?)", "2020-01-01")
			_, _ = db.Exec("INSERT INTO "+table+"(logtime) VALUES (?)", "2022-01-01")
		}

		if err := clearArchiveBefore("2021-01-01"); err != nil {
			t.Fatalf("clearArchiveBefore returned error: %v", err)
		}

		for _, table := range []string{"alertlog", "mappinglog", "pinglog"} {
			var count int
			if err := db.QueryRow("SELECT count(1) FROM " + table).Scan(&count); err != nil {
				t.Fatalf("query %s failed: %v", table, err)
			}
			if count != 1 {
				t.Fatalf("%s row count = %d, want 1", table, count)
			}
		}
	})
}

func TestClearArchiveTableBeforeDeletesMultipleBatches(t *testing.T) {
	withFuncTestDB(t, []string{
		`CREATE TABLE pinglog (logtime TEXT);`,
	}, func(db *sql.DB) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("begin fixture transaction: %v", err)
		}
		stmt, err := tx.Prepare(`INSERT INTO pinglog(logtime) VALUES (?)`)
		if err != nil {
			t.Fatalf("prepare fixture insert: %v", err)
		}
		for i := 0; i < archiveDeleteBatchSize*2+5; i++ {
			if _, err := stmt.Exec("2020-01-01"); err != nil {
				t.Fatalf("insert archived row: %v", err)
			}
		}
		_ = stmt.Close()
		if _, err := tx.Exec(`INSERT INTO pinglog(logtime) VALUES (?)`, "2022-01-01"); err != nil {
			t.Fatalf("insert retained row: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit fixture transaction: %v", err)
		}

		if err := clearArchiveTableBefore("pinglog", "2021-01-01"); err != nil {
			t.Fatalf("clearArchiveTableBefore returned error: %v", err)
		}
		var count int
		if err := db.QueryRow(`SELECT count(1) FROM pinglog`).Scan(&count); err != nil {
			t.Fatalf("query retained rows: %v", err)
		}
		if count != 1 {
			t.Fatalf("retained row count = %d, want 1", count)
		}
	})
}
