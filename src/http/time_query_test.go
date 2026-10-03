package http

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"smartping/src/g"
	"strings"
	"testing"
	"time"
)

func TestMinuteQueriesRejectNonexistentLocalTimes(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2011, 3, 13, 12, 0, 0, 0, location)
	for _, input := range []string{"2011-03-13 02:30", "2011-03-13 2:30", "2011-03-13  02:30"} {
		t.Run(input, func(t *testing.T) {
			for _, boundary := range []string{"starttime", "endtime"} {
				values := url.Values{"starttime": {"2011-03-13 01:30"}, "endtime": {"2011-03-13 03:30"}}
				values.Set(boundary, input)
				if _, _, err := resolvePingTimeRange(values, now, location); err == nil {
					t.Errorf("nonexistent %s %q was accepted", boundary, input)
				}
			}
			if key, err := resolveMappingDataKey(url.Values{"d": {input}}, now, location); err == nil {
				t.Errorf("nonexistent mapping minute %q became %q", input, key)
			}
		})
	}
}

func TestMinuteQueriesPreserveAcceptedSpellingsAndRepeatedLocalTimes(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		location *time.Location
		minute   string
	}{
		{time.UTC, "2026-09-01 08:05"},
		{time.FixedZone("UTC+8", 8*60*60), "2026-09-01 08:05"},
		{newYork, "2011-03-13 01:30"},
		{newYork, "2011-03-13 03:30"},
		{newYork, "2011-11-06 01:30"},
	} {
		for _, input := range []string{
			scenario.minute,
			scenario.minute[:11] + strings.TrimPrefix(scenario.minute[11:13], "0") + scenario.minute[13:],
			strings.Replace(scenario.minute, " ", "  ", 1),
		} {
			t.Run(scenario.location.String()+"/"+input, func(t *testing.T) {
				values := url.Values{"starttime": {input}, "endtime": {input}}
				start, end, err := resolvePingTimeRange(values, time.Now(), scenario.location)
				if err != nil || !start.Equal(end) || start.Format("2006-01-02 15:04") != scenario.minute {
					t.Fatalf("valid minute %q became %v to %v: %v", input, start, end, err)
				}
				key, err := resolveMappingDataKey(url.Values{"d": {input}}, time.Now(), scenario.location)
				if err != nil || key != scenario.minute {
					t.Fatalf("valid mapping minute %q became %q: %v", input, key, err)
				}
			})
		}
	}
}

func timeQueryTestEndpoint(t *testing.T, location *time.Location) (*sql.DB, http.Handler) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE pinglog (logtime TEXT, target TEXT, maxdelay TEXT, mindelay TEXT, avgdelay TEXT, losspk TEXT);
		CREATE TABLE mappinglog (logtime TEXT, mapjson TEXT);
		CREATE TABLE alertlog (logtime TEXT, targetip TEXT, targetname TEXT, tracert TEXT);`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	oldDB, oldLocation, oldConfig := g.Db, g.LocalTimezone, g.ConfigSnapshot()
	g.Db, g.LocalTimezone = db, location
	g.SetConfig(g.Config{Addr: "127.0.0.1", Network: map[string]g.NetworkMember{}})
	t.Cleanup(func() {
		db.Close()
		g.Db, g.LocalTimezone = oldDB, oldLocation
		g.SetConfig(oldConfig)
	})
	return db, newAppHandler()
}

func TestMinuteQueryEndpointsRejectLocalTimeGapsBeforeDatabaseAccess(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	db, handler := timeQueryTestEndpoint(t, location)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, query := range []struct {
		path    string
		values  url.Values
		message string
	}{
		{"/api/ping.json", url.Values{"ip": {"192.0.2.1"}, "starttime": {"2011-03-13 02:30"}, "endtime": {"2011-03-13 03:30"}}, "Invalid Time Range!"},
		{"/api/ping.json", url.Values{"ip": {"192.0.2.1"}, "starttime": {"2011-03-13 01:30"}, "endtime": {"2011-03-13 02:30"}}, "Invalid Time Range!"},
		{"/api/mapping.json", url.Values{"d": {"2011-03-13 02:30"}}, "Invalid Mapping Time!"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, query.path+"?"+query.values.Encode(), nil))
		if response.Code != http.StatusNotAcceptable || strings.TrimSpace(response.Body.String()) != query.message {
			t.Errorf("gap query %s = %d %q, want 406 %q before database access", query.path, response.Code, response.Body.String(), query.message)
		}
	}
}

func TestAlertDateQueriesPreserveCalendarDateAcrossMidnightTransitions(t *testing.T) {
	for _, scenario := range []struct {
		zone string
		date string
	}{
		{"America/Sao_Paulo", "2018-11-04"},
		{"Pacific/Apia", "2011-12-30"},
	} {
		t.Run(scenario.zone, func(t *testing.T) {
			location, err := time.LoadLocation(scenario.zone)
			if err != nil {
				t.Fatal(err)
			}
			db, handler := timeQueryTestEndpoint(t, location)
			day, err := time.Parse("2006-01-02", scenario.date)
			if err != nil {
				t.Fatal(err)
			}
			for _, stamp := range []string{
				day.AddDate(0, 0, -1).Format("2006-01-02") + " 23:59",
				scenario.date + " 00:00", scenario.date + " 23:59",
				day.AddDate(0, 0, 1).Format("2006-01-02") + " 00:00",
			} {
				// Include legacy calendar-labelled rows even when a local date was
				// skipped: a date filter must never silently select a different day.
				if _, err := db.Exec(`INSERT INTO alertlog VALUES (?, '192.0.2.1', 'target', '[]')`, stamp); err != nil {
					t.Fatal(err)
				}
			}
			for _, input := range []string{scenario.date, "alertlog-" + scenario.date} {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/alert.json?"+url.Values{"date": {input}}.Encode(), nil))
				if response.Code != http.StatusOK {
					t.Fatalf("date query = %d %q", response.Code, response.Body.String())
				}
				var sections []json.RawMessage
				if err := json.Unmarshal(response.Body.Bytes(), &sections); err != nil {
					t.Fatal(err)
				}
				if len(sections) != 2 {
					t.Fatal("invalid alert response sections")
				}
				var logs []g.AlertLog
				if err := json.Unmarshal(sections[1], &logs); err != nil {
					t.Fatal(err)
				}
				if len(logs) != 2 {
					t.Errorf("date %q returned %+v, want requested calendar date", input, logs)
				}
				stamps := make(map[string]bool)
				for _, log := range logs {
					stamps[log.Logtime] = true
				}
				if !stamps[scenario.date+" 00:00"] || !stamps[scenario.date+" 23:59"] {
					t.Errorf("date %q did not include its complete midnight-to-midnight range: %+v", input, logs)
				}
			}
		})
	}
}
