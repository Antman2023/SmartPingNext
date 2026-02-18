package funcs

import (
	"database/sql"
	"smartping/src/g"
	"testing"
	"time"
)

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

			if !CheckAlertStatus(v) {
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

			if CheckAlertStatus(v) {
				t.Fatalf("CheckAlertStatus should return false when count > threshold")
			}
		})
	})
}

func TestAlertStorage(t *testing.T) {
	schema := []string{
		`CREATE TABLE alertlog (logtime TEXT, targetip TEXT, targetname TEXT, tracert TEXT);`,
	}

	withFuncTestDB(t, schema, func(db *sql.DB) {
		item := g.AlertLog{
			Logtime:    time.Now().Format("2006-01-02 15:04"),
			Targetip:   "8.8.8.8",
			Targetname: "google-dns",
			Tracert:    "[]",
		}

		AlertStorage(item)

		var cnt int
		err := db.QueryRow(`SELECT count(1) FROM alertlog WHERE targetip = ? AND targetname = ?`, item.Targetip, item.Targetname).Scan(&cnt)
		if err != nil {
			t.Fatalf("query inserted alert failed: %v", err)
		}
		if cnt != 1 {
			t.Fatalf("inserted row count = %d, want 1", cnt)
		}
	})
}
