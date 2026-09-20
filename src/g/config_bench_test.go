package g

import (
	"database/sql"
	"fmt"
	"testing"
)

func BenchmarkAlertDateQueries(b *testing.B) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE alertlog (logtime TEXT);
		WITH RECURSIVE entries(n) AS (VALUES(0) UNION ALL SELECT n+1 FROM entries WHERE n<99999)
		INSERT INTO alertlog SELECT printf('2026-09-%02d %02d:%02d', n%30+1, n%24, n%60) FROM entries;
		CREATE INDEX alert_dates ON alertlog(date(logtime));`); err != nil {
		b.Fatal(err)
	}
	for _, query := range []struct{ name, sql string }{
		{"group", "SELECT date(logtime) FROM alertlog GROUP BY date(logtime) ORDER BY date(logtime) DESC"},
		{"distinct", "SELECT DISTINCT date(logtime) FROM alertlog ORDER BY date(logtime) DESC"},
	} {
		b.Run(query.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rows, err := db.Query(query.sql)
				if err != nil {
					b.Fatal(err)
				}
				count := 0
				for rows.Next() {
					var date string
					if err := rows.Scan(&date); err != nil {
						rows.Close()
						b.Fatal(err)
					}
					count++
					if date != fmt.Sprintf("2026-09-%02d", 31-count) {
						rows.Close()
						b.Fatalf("unexpected date %q at position %d", date, count)
					}
				}
				err = rows.Err()
				rows.Close()
				if err != nil || count != 30 {
					b.Fatalf("date count %d, error %v", count, err)
				}
			}
		})
	}
}
