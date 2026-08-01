package funcs

import (
	"database/sql"
	"testing"
)

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
