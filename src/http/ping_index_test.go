package http

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"smartping/src/g"
	"testing"
	"time"
)

var pingIndexBenchmarkSum int

func TestPingTimelineIndexPreservesExactLastOccurrenceLookup(t *testing.T) {
	for name, timestamps := range map[string][]string{
		"empty":              nil,
		"single":             {"2026-10-04 00:00"},
		"ordered":            {"2026-10-03 23:59", "2026-10-04 00:00", "2026-10-04 00:01", "2026-10-04 00:02"},
		"ordered duplicates": {"2026-10-04 00:00", "2026-10-04 00:01", "2026-10-04 00:01", "2026-10-04 00:02"},
		"clock rollback":     {"2026-10-04 01:00", "2026-10-04 01:01", "2026-10-04 01:02", "2026-10-04 01:00", "2026-10-04 01:01"},
	} {
		t.Run(name, func(t *testing.T) {
			original := append([]string(nil), timestamps...)
			positions := make(map[string]int)
			for i, stamp := range timestamps {
				positions[stamp] = i
			}
			index := newPingTimelineIndex(timestamps)
			queries := append([]string{"", "2026-10-03 23:58", "2026-10-04 23:59", "2026-10-04 0:01"}, timestamps...)
			for round := 0; round < 5; round++ {
				for _, stamp := range queries {
					want, exists := positions[stamp]
					got, found := index.lookup(stamp)
					if got != want || found != exists {
						t.Fatalf("lookup(%q) = (%d, %t), want (%d, %t)", stamp, got, found, want, exists)
					}
				}
				rand.New(rand.NewSource(int64(round))).Shuffle(len(queries), func(i, j int) { queries[i], queries[j] = queries[j], queries[i] })
			}
			if !reflect.DeepEqual(timestamps, original) {
				t.Fatal("lookup modified the timeline order")
			}
		})
	}
}

func pingIndexTestEndpoint(t *testing.T, location *time.Location) (*sql.DB, http.Handler) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE pinglog (logtime TEXT, target TEXT, maxdelay TEXT, mindelay TEXT, avgdelay TEXT, losspk TEXT);
		CREATE INDEX pinglog_target_logtime ON pinglog(target, logtime);`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	oldConfig, oldDB, oldTimezone := g.ConfigSnapshot(), g.Db, g.LocalTimezone
	g.Db, g.LocalTimezone = db, location
	g.SetConfig(g.Config{Addr: "127.0.0.1", Network: map[string]g.NetworkMember{}})
	t.Cleanup(func() {
		db.Close()
		g.Db, g.LocalTimezone = oldDB, oldTimezone
		g.SetConfig(oldConfig)
	})
	return db, newAppHandler()
}

func readPingIndexTestHistory(t *testing.T, handler http.Handler, start, end string) map[string][]string {
	t.Helper()
	query := url.Values{"ip": {"192.0.2.1"}, "starttime": {start}, "endtime": {end}}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/ping.json?"+query.Encode(), nil))
	if response.Code != http.StatusOK {
		t.Fatalf("history status = %d: %s", response.Code, response.Body.String())
	}
	var result map[string][]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPingEndpointSparseMaximumRangePreservesPositionsAndGaps(t *testing.T) {
	db, handler := pingIndexTestEndpoint(t, time.UTC)
	if _, err := db.Exec(`INSERT INTO pinglog VALUES
		('2026-02-01 00:00', '192.0.2.1', '0', '0', '0', '100'),
		('2026-01-01 00:00', '192.0.2.1', '0', '0', '0', '0'),
		('2026-01-16 12:00', '192.0.2.1', '20', '10', '15', '0'),
		('2026-01-01 00:01', '192.0.2.2', '99', '99', '99', '99'),
		('2026-01-16 12:0', '192.0.2.1', '99', '99', '99', '99');`); err != nil {
		t.Fatal(err)
	}
	result := readPingIndexTestHistory(t, handler, "2026-01-01 00:00", "2026-02-01 00:00")
	if len(result["lastcheck"]) != maxPingRangeMinutes+1 {
		t.Fatalf("timeline length = %d", len(result["lastcheck"]))
	}
	for key, values := range result {
		if len(values) != len(result["lastcheck"]) {
			t.Fatalf("%s has %d values", key, len(values))
		}
	}
	for i, stamp := range result["lastcheck"] {
		wantStamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute).Format("2006-01-02 15:04")
		if stamp != wantStamp {
			t.Fatalf("timestamp %d = %q, want %q", i, stamp, wantStamp)
		}
		for key, values := range result {
			if key == "lastcheck" {
				continue
			}
			want := "-"
			switch i {
			case 0:
				want = "0"
			case 15*24*60 + 12*60:
				want = map[string]string{"maxdelay": "20", "mindelay": "10", "avgdelay": "15", "losspk": "0"}[key]
			case maxPingRangeMinutes:
				want = "0"
				if key == "losspk" {
					want = "100"
				}
			}
			if values[i] != want {
				t.Fatalf("%s[%d] = %q, want %q", key, i, values[i], want)
			}
		}
	}
}

func TestPingEndpointClockRollbackKeepsLastRepeatedMinute(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	db, handler := pingIndexTestEndpoint(t, location)
	if _, err := db.Exec(`INSERT INTO pinglog VALUES
		('2011-11-06 01:00', '192.0.2.1', '14', '14', '14', '0'),
		('2011-11-06 01:59', '192.0.2.1', '0', '0', '0', '100');`); err != nil {
		t.Fatal(err)
	}
	result := readPingIndexTestHistory(t, handler, "2011-11-06 00:58", "2011-11-06 02:02")
	for _, repeated := range []string{"2011-11-06 01:00", "2011-11-06 01:59"} {
		positions := []int{}
		for i, stamp := range result["lastcheck"] {
			if stamp == repeated {
				positions = append(positions, i)
			}
		}
		if len(positions) != 2 {
			t.Fatalf("rollback minute %q appeared %d times", repeated, len(positions))
		}
		for _, key := range []string{"maxdelay", "mindelay", "avgdelay", "losspk"} {
			if result[key][positions[0]] != "-" {
				t.Fatalf("first repeated %s was populated", key)
			}
			want := "14"
			if key == "losspk" {
				want = "0"
			}
			if repeated == "2011-11-06 01:59" {
				want = "0"
				if key == "losspk" {
					want = "100"
				}
			}
			if result[key][positions[1]] != want {
				t.Fatalf("last repeated %s = %q, want %q", key, result[key][positions[1]], want)
			}
		}
	}
}

// Include index construction: it happens anew for each HTTP query.
func BenchmarkPingTimelineIndex(b *testing.B) {
	for _, minutes := range []int{360, 31 * 24 * 60} {
		timestamps := make([]string, minutes+1)
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := range timestamps {
			timestamps[i] = start.Add(time.Duration(i) * time.Minute).Format("2006-01-02 15:04")
		}
		for _, step := range []int{minutes + 1, 60, 1} {
			queries := make([]string, 0, (minutes+1)/step+1)
			for i := 0; i < len(timestamps); i += step {
				queries = append(queries, timestamps[i])
			}
			for _, legacy := range []bool{true, false} {
				method := "timeline"
				if legacy {
					method = "map"
				}
				b.Run(fmt.Sprintf("%s/minutes=%d/samples=%d", method, minutes, len(queries)), func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						value := 0
						if legacy {
							positions := make(map[string]int, len(timestamps))
							for i, stamp := range timestamps {
								positions[stamp] = i
							}
							for _, stamp := range queries {
								value += positions[stamp]
							}
						} else {
							index := newPingTimelineIndex(timestamps)
							for _, stamp := range queries {
								position, found := index.lookup(stamp)
								if !found {
									b.Fatal("sample disappeared from the index")
								}
								value += position
							}
						}
						pingIndexBenchmarkSum = value
					}
				})
			}
		}
	}
}
