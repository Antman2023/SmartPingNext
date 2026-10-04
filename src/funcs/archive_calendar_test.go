package funcs

import (
	"database/sql"
	"reflect"
	"testing"
	"time"
)

func TestArchiveCutoffPreservesCalendarDateAcrossTimezoneTransitions(t *testing.T) {
	for _, tc := range []struct {
		name, zone, current, cutoff string
		days                        int
	}{
		{"ordinary date", "UTC", "2026-10-04 12:00", "2026-09-04", 30},
		{"local date near midnight", "Asia/Shanghai", "2026-10-04 00:01", "2026-09-04", 30},
		{"spring gap", "America/New_York", "2011-03-14 02:30", "2011-03-13", 1},
		{"fall rollback", "America/New_York", "2011-11-07 01:30", "2011-11-06", 1},
		{"midnight gap", "America/Sao_Paulo", "2018-11-05 00:30", "2018-11-04", 1},
		{"skipped date", "Pacific/Apia", "2011-12-31 12:00", "2011-12-30", 1},
		{"thirty days across skipped date", "Pacific/Apia", "2012-01-29 12:00", "2011-12-30", 30},
		{"new year", "UTC", "2026-01-01 00:00", "2025-12-31", 1},
		{"leap year", "UTC", "2024-03-01 12:00", "2024-02-29", 1},
		{"ordinary February", "UTC", "2025-03-01 12:00", "2025-02-28", 1},
		{"maximum configured days", "UTC", "2026-10-04 12:00", "1926-10-29", 36500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			location, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			now, err := time.ParseInLocation("2006-01-02 15:04", tc.current, location)
			if err != nil {
				t.Fatal(err)
			}
			if got := archiveCutoffDate(now, tc.days); got != tc.cutoff {
				t.Fatalf("cutoff=%q, want %q", got, tc.cutoff)
			}
		})
	}
}

func TestArchiveTimezoneTransitionsPreserveCutoffAndNewerRows(t *testing.T) {
	for _, tc := range []struct {
		zone, current, cutoff string
	}{
		{"America/Sao_Paulo", "2018-11-05 00:30", "2018-11-04"},
		{"Pacific/Apia", "2011-12-31 12:00", "2011-12-30"},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			location, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			now, err := time.ParseInLocation("2006-01-02 15:04", tc.current, location)
			if err != nil {
				t.Fatal(err)
			}
			calendarDay, err := time.Parse("2006-01-02", tc.cutoff)
			if err != nil {
				t.Fatal(err)
			}
			withFuncTestDB(t, []string{
				`CREATE TABLE pinglog (logtime TEXT); CREATE INDEX pinglog_logtime ON pinglog(logtime)`,
				`CREATE TABLE alertlog (logtime TEXT); CREATE INDEX alertlog_logtime ON alertlog(logtime)`,
				`CREATE TABLE mappinglog (logtime TEXT); CREATE INDEX mappinglog_logtime ON mappinglog(logtime)`,
			}, func(db *sql.DB) {
				db.SetMaxOpenConns(1)
				before := calendarDay.AddDate(0, 0, -1).Format("2006-01-02") + " 23:59"
				after := calendarDay.AddDate(0, 0, 1).Format("2006-01-02") + " 00:00"
				want := []string{tc.cutoff + " 00:00", tc.cutoff + " 23:59", after}
				for _, table := range []string{"pinglog", "alertlog", "mappinglog"} {
					for _, stamp := range append([]string{before}, want...) {
						if _, err := db.Exec("INSERT INTO "+table+" VALUES (?)", stamp); err != nil {
							t.Fatal(err)
						}
					}
				}
				if err := clearArchiveBeforeContext(t.Context(), archiveCutoffDate(now, 1)); err != nil {
					t.Fatal(err)
				}
				for _, table := range []string{"pinglog", "alertlog", "mappinglog"} {
					rows, err := db.Query("SELECT logtime FROM " + table + " ORDER BY logtime")
					if err != nil {
						t.Fatal(err)
					}
					var got []string
					for rows.Next() {
						var stamp string
						if err := rows.Scan(&stamp); err != nil {
							rows.Close()
							t.Fatal(err)
						}
						got = append(got, stamp)
					}
					if err := rows.Err(); err != nil {
						rows.Close()
						t.Fatal(err)
					}
					rows.Close()
					if !reflect.DeepEqual(got, want) {
						t.Errorf("%s retained=%v, want %v", table, got, want)
					}
				}
			})
		})
	}
}
