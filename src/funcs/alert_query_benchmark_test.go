package funcs

import (
	"database/sql"
	"fmt"
	"testing"
	"time"
)

// Keep the previous aggregate query as a baseline for the SQL-only benchmark.
const aggregateAlertStatusReferenceQuery = `SELECT count(1), coalesce(sum(CASE WHEN cast(avgdelay as double) > ? OR cast(losspk as double) >= ? THEN 1 ELSE 0 END), 0) FROM (
	SELECT avgdelay, losspk FROM pinglog
	WHERE target = ? AND logtime >= ? AND logtime <= ?
	ORDER BY logtime DESC LIMIT ?
)`

func BenchmarkAlertStatusQueries(b *testing.B) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	b.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE pinglog (logtime TEXT, target TEXT, avgdelay TEXT, losspk TEXT);
		CREATE INDEX pinglog_target_logtime ON pinglog(target, logtime)`); err != nil {
		b.Fatal(err)
	}
	now := time.Date(2026, 10, 4, 12, 0, 30, 0, time.Local)
	tx, err := db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	defer tx.Rollback()
	insert, err := tx.Prepare(`INSERT INTO pinglog VALUES (?, ?, ?, '0')`)
	if err != nil {
		b.Fatal(err)
	}
	defer insert.Close()
	for _, samples := range []int{10, 60, 600, 1440} {
		for _, pattern := range []string{"healthy", "newestBad", "newestTenBad", "oldestBad", "sparse"} {
			target := fmt.Sprintf("%d-%s", samples, pattern)
			rows := samples
			if pattern == "sparse" {
				rows = 1
			}
			for sample := 0; sample < rows; sample++ {
				delay := "10"
				if pattern == "newestBad" && sample == 0 || pattern == "newestTenBad" && sample < 10 || pattern == "oldestBad" && sample == samples-1 {
					delay = "300"
				}
				if _, err := insert.Exec(now.Add(-time.Duration(sample)*time.Minute).Format("2006-01-02 15:04"), target, delay); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	for _, samples := range []int{10, 60, 600, 1440} {
		for _, pattern := range []string{"healthy", "newestBad", "newestTenBad", "oldestBad", "sparse", "missing"} {
			args := []any{"200", "30", fmt.Sprintf("%d-%s", samples, pattern),
				now.Add(-time.Duration(samples) * time.Minute).Format("2006-01-02 15:04"),
				now.Format("2006-01-02 15:04"), samples}
			want := 1
			occurrences := 1
			if pattern == "newestTenBad" {
				occurrences = 10
			}
			if pattern == "newestBad" || pattern == "newestTenBad" || pattern == "oldestBad" {
				want = 0
			} else if pattern == "missing" {
				want = -1
			}
			for _, early := range []bool{false, true} {
				name := "Aggregate"
				query := aggregateAlertStatusReferenceQuery
				queryArgs := args
				if early {
					name = "EarlyExit"
					query = alertStatusEarlyExitQuery
					queryArgs = append(append([]any(nil), args...), occurrences-1)
				}
				b.Run(fmt.Sprintf("%dSamples/%s/%s", samples, pattern, name), func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						var state int
						var err error
						if early {
							err = db.QueryRow(query, queryArgs...).Scan(&state)
						} else {
							var total, bad int
							err = db.QueryRow(query, queryArgs...).Scan(&total, &bad)
							state = 1
							if total == 0 {
								state = -1
							} else if bad >= occurrences {
								state = 0
							}
						}
						if err != nil || state != want {
							b.Fatalf("state=%d, err=%v; want %d", state, err, want)
						}
					}
				})
			}
		}
	}
}
