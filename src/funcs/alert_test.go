package funcs

import (
	"database/sql"
	"smartping/src/g"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartAlertSkipsOverlappingCheck(t *testing.T) {
	atomic.StoreInt32(&alertRunning, 1)
	defer atomic.StoreInt32(&alertRunning, 0)

	StartAlert()
	if got := atomic.LoadInt32(&alertRunning); got != 1 {
		t.Fatalf("alertRunning = %d, want existing check to remain active", got)
	}
}

func withFuncTestDB(t *testing.T, schema []string, fn func(db *sql.DB)) {
	t.Helper()
	oldDB := g.Db
	oldCfg := g.Cfg

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite failed: %v", err)
	}
	defer db.Close()

	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("init schema failed: %v", err)
		}
	}

	g.Db = db
	g.Cfg = g.Config{Name: "from", Addr: "127.0.0.1"}

	defer func() {
		g.Db = oldDB
		g.Cfg = oldCfg
	}()

	fn(db)
}

func TestCheckAlertStatus(t *testing.T) {
	schema := []string{
		`CREATE TABLE pinglog (logtime TEXT, target TEXT, avgdelay TEXT, losspk TEXT);`,
	}

	t.Run("true when count less or equal threshold", func(t *testing.T) {
		withFuncTestDB(t, schema, func(db *sql.DB) {
			now := time.Now().Format("2006-01-02 15:04")
			_, _ = db.Exec(`INSERT INTO pinglog(logtime,target,avgdelay,losspk) VALUES (?,?,?,?)`, now, "1.1.1.1", "250", "0")

			v := map[string]string{
				"Thdchecksec": "600",
				"Addr":        "1.1.1.1",
				"Thdavgdelay": "200",
				"Thdloss":     "30",
				"Thdoccnum":   "1",
			}

			healthy, err := CheckAlertStatus(v)
			if err != nil {
				t.Fatalf("CheckAlertStatus returned error: %v", err)
			}
			if !healthy {
				t.Fatalf("CheckAlertStatus should return true when count <= threshold")
			}
		})
	})

	t.Run("false when count greater than threshold", func(t *testing.T) {
		withFuncTestDB(t, schema, func(db *sql.DB) {
			now := time.Now().Format("2006-01-02 15:04")
			_, _ = db.Exec(`INSERT INTO pinglog(logtime,target,avgdelay,losspk) VALUES (?,?,?,?)`, now, "2.2.2.2", "300", "0")
			_, _ = db.Exec(`INSERT INTO pinglog(logtime,target,avgdelay,losspk) VALUES (?,?,?,?)`, now, "2.2.2.2", "320", "0")

			v := map[string]string{
				"Thdchecksec": "600",
				"Addr":        "2.2.2.2",
				"Thdavgdelay": "200",
				"Thdloss":     "30",
				"Thdoccnum":   "1",
			}

			healthy, err := CheckAlertStatus(v)
			if err != nil {
				t.Fatalf("CheckAlertStatus returned error: %v", err)
			}
			if healthy {
				t.Fatalf("CheckAlertStatus should return false when count > threshold")
			}
		})
	})

	t.Run("true when no matching rows", func(t *testing.T) {
		withFuncTestDB(t, schema, func(db *sql.DB) {
			v := map[string]string{
				"Thdchecksec": "600",
				"Addr":        "9.9.9.9",
				"Thdavgdelay": "200",
				"Thdloss":     "30",
				"Thdoccnum":   "1",
			}

			healthy, err := CheckAlertStatus(v)
			if err != nil {
				t.Fatalf("CheckAlertStatus returned error: %v", err)
			}
			if !healthy {
				t.Fatalf("CheckAlertStatus should return true when count is zero and threshold is one")
			}
		})
	})

	t.Run("database error is returned", func(t *testing.T) {
		withFuncTestDB(t, schema, func(db *sql.DB) {
			if err := db.Close(); err != nil {
				t.Fatalf("close test database failed: %v", err)
			}
			_, err := CheckAlertStatus(map[string]string{
				"Thdchecksec": "600",
				"Addr":        "9.9.9.9",
				"Thdavgdelay": "200",
				"Thdloss":     "30",
				"Thdoccnum":   "1",
			})
			if err == nil {
				t.Fatalf("CheckAlertStatus should return database error")
			}
		})
	})
}

func TestAlertWindowStartUsesMinuteSamples(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 57, 0, time.Local)
	tests := []struct {
		name          string
		windowSeconds int
		want          time.Time
	}{
		{name: "sub-minute legacy window", windowSeconds: 1, want: time.Date(2026, 7, 31, 12, 0, 0, 0, time.Local)},
		{name: "fifteen minute window", windowSeconds: 900, want: time.Date(2026, 7, 31, 11, 46, 0, 0, time.Local)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := alertWindowStart(now, tt.windowSeconds); !got.Equal(tt.want) {
				t.Fatalf("alertWindowStart = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlertStorage(t *testing.T) {
	schema := []string{
		`CREATE TABLE alertlog (logtime TEXT, targetip TEXT, targetname TEXT, tracert TEXT, UNIQUE(logtime, targetip));`,
	}

	withFuncTestDB(t, schema, func(db *sql.DB) {
		item := g.AlertLog{
			Logtime:    time.Now().Format("2006-01-02 15:04"),
			Targetip:   "8.8.8.8",
			Targetname: "google-dns",
			Tracert:    "[]",
		}

		AlertStorage(item)
		item.Targetname = "updated-name"
		item.Tracert = `[{"Host":"192.0.2.1"}]`
		AlertStorage(item)

		var cnt int
		var targetname, tracert string
		err := db.QueryRow(`SELECT count(1), targetname, tracert FROM alertlog WHERE targetip = ? GROUP BY targetname, tracert`, item.Targetip).Scan(&cnt, &targetname, &tracert)
		if err != nil {
			t.Fatalf("query inserted alert failed: %v", err)
		}
		if cnt != 1 {
			t.Fatalf("inserted row count = %d, want 1", cnt)
		}
		if targetname != item.Targetname || tracert != item.Tracert {
			t.Fatalf("duplicate alert was not updated: targetname=%q tracert=%q", targetname, tracert)
		}
	})
}
