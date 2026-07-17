package funcs

import (
	"database/sql"
	"testing"
)

func TestClearArchiveBeforeRollsBackOnFailure(t *testing.T) {
	withFuncTestDB(t, []string{
		`CREATE TABLE alertlog (logtime TEXT);`,
		`CREATE TABLE mappinglog (logtime TEXT);`,
	}, func(db *sql.DB) {
		_, _ = db.Exec(`INSERT INTO alertlog(logtime) VALUES (?)`, "2020-01-01")
		_, _ = db.Exec(`INSERT INTO mappinglog(logtime) VALUES (?)`, "2020-01-01")

		if err := clearArchiveBefore("2021-01-01"); err == nil {
			t.Fatalf("clearArchiveBefore should fail when a table is missing")
		}

		for _, table := range []string{"alertlog", "mappinglog"} {
			var count int
			if err := db.QueryRow("SELECT count(1) FROM " + table).Scan(&count); err != nil {
				t.Fatalf("query %s failed: %v", table, err)
			}
			if count != 1 {
				t.Fatalf("%s row count = %d, want 1 after rollback", table, count)
			}
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
