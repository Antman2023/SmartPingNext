package http

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

const legacyAlertDatesQuery = "SELECT DISTINCT date(logtime) AS ldate FROM alertlog ORDER BY date(logtime) DESC"

func alertDatesTestDatabase(t testing.TB) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE alertlog (logtime TEXT);
		CREATE INDEX alertlog_date ON alertlog(date(logtime));`); err != nil {
		t.Fatal(err)
	}
	return db
}

func readAlertDates(t testing.TB, db *sql.DB, query string) []sql.NullString {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var dates []sql.NullString
	for rows.Next() {
		var date sql.NullString
		if err := rows.Scan(&date); err != nil {
			t.Fatal(err)
		}
		dates = append(dates, date)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return dates
}

func TestAlertDateIndexSeeksPreserveDistinctQueryResults(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		stamps []any
	}{
		{"empty", nil},
		{"duplicates and gaps", []any{"2026-09-01 12:00", "2026-09-01 23:59", "2026-10-04 00:00", "2026-09-01 12:00", "2024-02-29 01:30"}},
		{"SQLite accepted formats", []any{"2026-09-01", "2026-09-01T23:59:59.123Z", "2026-09-01 23:59:00-08:00", "2026-09-02 00:00:00+08:00", "2451544.5", "-0001-01-01 00:00"}},
		{"malformed records", []any{nil, "", "invalid timestamp", "2026-09-01 12:00", "invalid timestamp"}},
		{"only malformed records", []any{nil, "invalid timestamp"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := alertDatesTestDatabase(t)
			for _, stamp := range scenario.stamps {
				if _, err := db.Exec("INSERT INTO alertlog VALUES (?)", stamp); err != nil {
					t.Fatal(err)
				}
			}
			want := readAlertDates(t, db, legacyAlertDatesQuery)
			got := readAlertDates(t, db, alertDatesQuery)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("date index seeks = %+v, legacy distinct = %+v", got, want)
			}
		})
	}
}

func TestAlertDateIndexSeeksSupportLongArchives(t *testing.T) {
	db := alertDatesTestDatabase(t)
	if _, err := db.Exec(`WITH RECURSIVE days(n) AS (VALUES(0) UNION ALL SELECT n+1 FROM days WHERE n < 1999)
		INSERT INTO alertlog SELECT datetime('2020-01-01', '+' || n || ' days') FROM days`); err != nil {
		t.Fatal(err)
	}
	got, want := readAlertDates(t, db, alertDatesQuery), readAlertDates(t, db, legacyAlertDatesQuery)
	if len(got) != 2000 || !reflect.DeepEqual(got, want) {
		t.Fatalf("long archive returned %d dates, want 2000 matching legacy output", len(got))
	}
}

func TestAlertDateIndexSeeksReflectInsertionsAndArchiveDeletion(t *testing.T) {
	db := alertDatesTestDatabase(t)
	for _, mutation := range []string{
		"INSERT INTO alertlog VALUES ('2026-09-01 12:00'), ('2026-09-01 23:59'), ('2026-09-03 00:00')",
		"INSERT INTO alertlog VALUES ('2026-09-02 00:00'), ('2026-09-05 01:00')",
		"DELETE FROM alertlog WHERE logtime = '2026-09-01 12:00'",
		"DELETE FROM alertlog WHERE logtime < '2026-09-02'",
		"DELETE FROM alertlog",
		"INSERT INTO alertlog VALUES ('2026-10-04 00:00')",
	} {
		if _, err := db.Exec(mutation); err != nil {
			t.Fatal(err)
		}
		got, want := readAlertDates(t, db, alertDatesQuery), readAlertDates(t, db, legacyAlertDatesQuery)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("after %s: date seeks = %+v, legacy distinct = %+v", mutation, got, want)
		}
	}
}

func TestAlertDateIndexSeeksUseExistingExpressionIndex(t *testing.T) {
	db := alertDatesTestDatabase(t)
	rows, err := db.Query("EXPLAIN QUERY PLAN " + alertDatesQuery)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	// The max-date lookups and malformed-date check must all search the same
	// expression index; scanning the small recursive date output is expected.
	if got := strings.Count(plan.String(), "SEARCH alertlog USING INDEX alertlog_date"); got != 3 || strings.Contains(plan.String(), "SCAN alertlog") {
		t.Fatalf("date lookups did not use three indexed searches: %s", plan.String())
	}
}

func BenchmarkAlertDateQueries(b *testing.B) {
	for _, perDay := range []int{1, 100, 5000} {
		b.Run(fmt.Sprintf("31days/%d_per_day", perDay), func(b *testing.B) {
			db := alertDatesTestDatabase(b)
			tx, err := db.Begin()
			if err != nil {
				b.Fatal(err)
			}
			defer tx.Rollback()
			stmt, err := tx.Prepare("INSERT INTO alertlog VALUES (?)")
			if err != nil {
				b.Fatal(err)
			}
			defer stmt.Close()
			for day := 0; day < 31; day++ {
				stamp := time.Date(2026, 9, 1+day, 12, 0, 0, 0, time.UTC).Format("2006-01-02 15:04")
				for i := 0; i < perDay; i++ {
					if _, err := stmt.Exec(stamp); err != nil {
						b.Fatal(err)
					}
				}
			}
			if err := tx.Commit(); err != nil {
				b.Fatal(err)
			}
			for _, candidate := range []struct{ name, query string }{{"legacy_distinct", legacyAlertDatesQuery}, {"index_seeks", alertDatesQuery}} {
				b.Run(candidate.name, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if dates := readAlertDates(b, db, candidate.query); len(dates) != 31 {
							b.Fatal("dates lost")
						}
					}
				})
			}
		})
	}
}
