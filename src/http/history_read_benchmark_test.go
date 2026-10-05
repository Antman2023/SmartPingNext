package http

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"smartping/src/g"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func historyReadFixture(t testing.TB) (*sql.DB, http.Handler) {
	return historyReadFixtureWithDriver(t, "sqlite")
}

func historyReadFixtureWithDriver(t testing.TB, driverName string) (*sql.DB, http.Handler) {
	return historyReadFixtureWithDSN(t, driverName, ":memory:")
}

func historyReadFixtureWithDSN(t testing.TB, driverName, dsn string) (*sql.DB, http.Handler) {
	t.Helper()
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE pinglog (logtime TEXT, target TEXT, maxdelay TEXT, mindelay TEXT, avgdelay TEXT, losspk TEXT, UNIQUE(logtime,target));
		CREATE INDEX pinglog_target_logtime ON pinglog(target,logtime);
		CREATE TABLE alertlog (logtime TEXT, targetname TEXT, targetip TEXT, tracert TEXT, UNIQUE(logtime,targetip));
		CREATE INDEX alertlog_logtime ON alertlog(logtime);
		CREATE INDEX alertlog_date ON alertlog(date(logtime));`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	oldConfig, oldDB, oldZone := g.ConfigSnapshot(), g.Db, g.LocalTimezone
	oldOutput := logrus.StandardLogger().Out
	g.Db, g.LocalTimezone = db, time.UTC
	g.SetConfig(g.Config{Name: "local <节点>", Addr: "127.0.0.1", Network: map[string]g.NetworkMember{}})
	logrus.SetOutput(io.Discard)
	t.Cleanup(func() {
		logrus.SetOutput(oldOutput)
		g.Db, g.LocalTimezone = oldDB, oldZone
		g.SetConfig(oldConfig)
		db.Close()
	})
	return db, newAppHandler()
}

func historyPingValues(index int) (max, min, avg, loss string) {
	max = strconv.Itoa(index % 257)
	min = strconv.Itoa(index % 31)
	if index%7 == 0 {
		min = "-1"
	}
	avg = fmt.Sprintf("%d.5", index%97)
	loss = strconv.Itoa(index % 101)
	return
}

func seedHistoryPing(t testing.TB, db *sql.DB, start time.Time, count int) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	statement, err := tx.Prepare("INSERT INTO pinglog VALUES(?,?,?,?,?,?)")
	if err != nil {
		t.Fatal(err)
	}
	defer statement.Close()
	for i := 0; i < count; i++ {
		max, min, avg, loss := historyPingValues(i)
		if _, err := statement.Exec(start.Add(time.Duration(i)*time.Minute).Format("2006-01-02 15:04"), "192.0.2.1", max, min, avg, loss); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func seedHistoryAlerts(t testing.TB, db *sql.DB, count int) []g.AlertLog {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	statement, err := tx.Prepare("INSERT INTO alertlog VALUES(?,?,?,?)")
	if err != nil {
		t.Fatal(err)
	}
	defer statement.Close()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	records := make([]g.AlertLog, count)
	traces := []string{"", "trace failed", "[]", "路径 <&> \"\\"}
	for i := range records {
		records[i] = g.AlertLog{Logtime: start.Add(time.Duration(i/250) * time.Minute).Format("2006-01-02 15:04"),
			Targetname: fmt.Sprintf("target %d", i), Targetip: fmt.Sprintf("192.0.2.%d", i%250+1),
			Tracert: traces[i%len(traces)], Fromname: "local <节点>", Fromip: "127.0.0.1"}
		row := records[i]
		if _, err := statement.Exec(row.Logtime, row.Targetname, row.Targetip, row.Tracert); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return records
}

func benchmarkHistoryResponse(b *testing.B, handler http.Handler, request *http.Request, want any) {
	b.Helper()
	expected, err := json.Marshal(want)
	if err != nil {
		b.Fatal(err)
	}
	check := func(response *httptest.ResponseRecorder) {
		b.Helper()
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), expected) {
			b.Fatalf("history status=%d, bytes=%d; want status=200 and %d original JSON bytes", response.Code, response.Body.Len(), len(expected))
		}
	}
	initial := httptest.NewRecorder()
	handler.ServeHTTP(initial, request)
	check(initial)
	b.ReportAllocs()
	b.ResetTimer()
	var last *httptest.ResponseRecorder
	for i := 0; i < b.N; i++ {
		last = httptest.NewRecorder()
		handler.ServeHTTP(last, request)
		if last.Code != http.StatusOK {
			b.Fatalf("history status=%d", last.Code)
		}
	}
	b.StopTimer()
	check(last)
}

func BenchmarkPingHistoryRead(b *testing.B) {
	db, handler := historyReadFixture(b)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seedHistoryPing(b, db, start, maxPingRangeMinutes+1)
	for _, minutes := range []int{360, 1440, maxPingRangeMinutes} {
		for _, populated := range []bool{false, true} {
			b.Run(fmt.Sprintf("%dMinutes/Populated=%t", minutes, populated), func(b *testing.B) {
				ip := "192.0.2.2"
				if populated {
					ip = "192.0.2.1"
				}
				query := url.Values{"ip": {ip}, "starttime": {start.Format("2006-01-02 15:04")},
					"endtime": {start.Add(time.Duration(minutes) * time.Minute).Format("2006-01-02 15:04")}}
				want := map[string][]string{}
				for _, field := range []string{"lastcheck", "maxdelay", "mindelay", "avgdelay", "losspk"} {
					want[field] = make([]string, minutes+1)
				}
				for i := 0; i <= minutes; i++ {
					want["lastcheck"][i] = start.Add(time.Duration(i) * time.Minute).Format("2006-01-02 15:04")
					max, min, avg, loss := "-", "-", "-", "-"
					if populated {
						max, min, avg, loss = historyPingValues(i)
						if min == "-1" {
							min = "0"
						}
					}
					want["maxdelay"][i], want["mindelay"][i] = max, min
					want["avgdelay"][i], want["losspk"][i] = avg, loss
				}
				request := httptest.NewRequest(http.MethodGet, "/api/ping.json?"+query.Encode(), nil)
				benchmarkHistoryResponse(b, handler, request, want)
			})
		}
	}
}

func BenchmarkAlertHistoryRead(b *testing.B) {
	for _, count := range []int{0, 512, 4096} {
		b.Run(fmt.Sprintf("%dRecords", count), func(b *testing.B) {
			db, handler := historyReadFixture(b)
			records := seedHistoryAlerts(b, db, count)
			dates := []string{}
			if count > 0 {
				dates = []string{"2026-01-01"}
			}
			request := httptest.NewRequest(http.MethodGet, "/api/alert.json?date=2026-01-01", nil)
			benchmarkHistoryResponse(b, handler, request, []any{dates, records})
		})
	}
}
