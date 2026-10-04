package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"smartping/src/g"
	"testing"
	"time"
)

func TestHistoryRowsPreserveDistinctStoredValues(t *testing.T) {
	t.Run("dense ping measurements", func(t *testing.T) {
		db, handler := historyReadFixture(t)
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		const count = 361
		seedHistoryPing(t, db, start, count)
		for attempt := 0; attempt < 2; attempt++ {
			result := readPingIndexTestHistory(t, handler, "2026-01-01 00:00", "2026-01-01 06:00")
			for _, field := range []string{"lastcheck", "maxdelay", "mindelay", "avgdelay", "losspk"} {
				if len(result[field]) != count {
					t.Fatalf("%s has %d entries, want %d", field, len(result[field]), count)
				}
			}
			for i := 0; i < count; i++ {
				max, min, avg, loss := historyPingValues(i)
				if min == "-1" {
					min = "0"
				}
				want := map[string]string{"lastcheck": start.Add(time.Duration(i) * time.Minute).Format("2006-01-02 15:04"),
					"maxdelay": max, "mindelay": min, "avgdelay": avg, "losspk": loss}
				for field, value := range want {
					if result[field][i] != value {
						t.Fatalf("attempt=%d, %s[%d]=%q, want %q", attempt, field, i, result[field][i], value)
					}
				}
			}
		}
	})
	t.Run("alert records and archive dates", func(t *testing.T) {
		db, handler := historyReadFixture(t)
		want := seedHistoryAlerts(t, db, 512)
		if _, err := db.Exec(`INSERT INTO alertlog VALUES
			('2026-01-02 00:00','other day','192.0.2.1',''),
			('2026-01-03 00:00','newer day','192.0.2.1','')`); err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 2; attempt++ {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/alert.json?date=2026-01-01", nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d: %s", response.Code, response.Body.String())
			}
			var sections []json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &sections); err != nil || len(sections) != 2 {
				t.Fatalf("sections=%d, error=%v", len(sections), err)
			}
			var dates []string
			var records []g.AlertLog
			if err := json.Unmarshal(sections[0], &dates); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(sections[1], &records); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(dates, []string{"2026-01-03", "2026-01-02", "2026-01-01"}) || len(records) != len(want) {
				t.Fatalf("dates=%v, records=%d; want three dates and %d records", dates, len(records), len(want))
			}
			byKey := make(map[string]g.AlertLog, len(records))
			for _, record := range records {
				byKey[record.Logtime+"/"+record.Targetip] = record
			}
			for _, expected := range want {
				if got := byKey[expected.Logtime+"/"+expected.Targetip]; got != expected {
					t.Fatalf("attempt=%d: record=%+v, want %+v", attempt, got, expected)
				}
			}
			if len(byKey) != len(want) {
				t.Fatalf("distinct records=%d, want %d", len(byKey), len(want))
			}
		}
	})
}
