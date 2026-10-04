package http

import (
	"testing"
	"time"
)

func TestPingEndpointIncludesRollbackMinutesBeforeItsStartLabel(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	db, handler := pingIndexTestEndpoint(t, location)
	if _, err := db.Exec(`INSERT INTO pinglog VALUES
		('2011-11-06 00:59', '192.0.2.1', '99', '99', '99', '99'),
		('2011-11-06 01:00', '192.0.2.1', '0', '0', '0', '0'),
		('2011-11-06 01:15', '192.0.2.1', '15', '10', '12', '0'),
		('2011-11-06 01:50', '192.0.2.1', '50', '40', '45', '0'),
		('2011-11-06 01:59', '192.0.2.1', '0', '-1', '0', '100'),
		('2011-11-06 02:10', '192.0.2.1', '20', '10', '15', '0'),
		('2011-11-06 02:11', '192.0.2.1', '99', '99', '99', '99'),
		('2011-11-06 01:15', '192.0.2.2', '99', '99', '99', '99')`); err != nil {
		t.Fatal(err)
	}
	result := readPingIndexTestHistory(t, handler, "2011-11-06 01:50", "2011-11-06 02:10")
	measured := map[int]map[string]string{
		10: {"maxdelay": "0", "mindelay": "0", "avgdelay": "0", "losspk": "0"},
		25: {"maxdelay": "15", "mindelay": "10", "avgdelay": "12", "losspk": "0"},
		60: {"maxdelay": "50", "mindelay": "40", "avgdelay": "45", "losspk": "0"},
		69: {"maxdelay": "0", "mindelay": "0", "avgdelay": "0", "losspk": "100"},
		80: {"maxdelay": "20", "mindelay": "10", "avgdelay": "15", "losspk": "0"},
	}
	assertPingTimezoneHistory(t, result, location, "2011-11-06 01:50", 81, measured)
}

func TestPingEndpointIncludesRollbackMinutesAfterItsEndLabel(t *testing.T) {
	for _, tc := range []struct {
		zone, start, end, stamp string
		size, position          int
	}{
		{"Europe/Berlin", "2011-10-30 01:50", "2011-10-30 02:10", "2011-10-30 02:40", 81, 50},
		{"Australia/Lord_Howe", "2011-04-03 01:20", "2011-04-03 01:40", "2011-04-03 01:50", 51, 30},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			location, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			db, handler := pingIndexTestEndpoint(t, location)
			if _, err := db.Exec(`INSERT INTO pinglog VALUES (?, '192.0.2.1', '20', '10', '15', '0')`, tc.stamp); err != nil {
				t.Fatal(err)
			}
			result := readPingIndexTestHistory(t, handler, tc.start, tc.end)
			assertPingTimezoneHistory(t, result, location, tc.start, tc.size, map[int]map[string]string{
				tc.position: {"maxdelay": "20", "mindelay": "10", "avgdelay": "15", "losspk": "0"},
			})
		})
	}
}

func TestPingEndpointTimelineBoundsPreserveOrdinaryAndForwardGapSamples(t *testing.T) {
	for _, tc := range []struct {
		zone, start, end, gap string
	}{
		{"UTC", "2026-01-01 10:00", "2026-01-01 10:02", ""},
		{"Asia/Shanghai", "2026-01-01 10:00", "2026-01-01 10:02", ""},
		{"America/New_York", "2011-03-13 01:59", "2011-03-13 03:01", "2011-03-13 02:30"},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			location, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			db, handler := pingIndexTestEndpoint(t, location)
			for _, stamp := range []string{tc.start, tc.end} {
				if _, err := db.Exec(`INSERT INTO pinglog VALUES (?, '192.0.2.1', '0', '0', '0', '0')`, stamp); err != nil {
					t.Fatal(err)
				}
			}
			if tc.gap != "" {
				if _, err := db.Exec(`INSERT INTO pinglog VALUES (?, '192.0.2.1', '99', '99', '99', '99')`, tc.gap); err != nil {
					t.Fatal(err)
				}
			}
			result := readPingIndexTestHistory(t, handler, tc.start, tc.end)
			assertPingTimezoneHistory(t, result, location, tc.start, 3, map[int]map[string]string{
				0: {"maxdelay": "0", "mindelay": "0", "avgdelay": "0", "losspk": "0"},
				2: {"maxdelay": "0", "mindelay": "0", "avgdelay": "0", "losspk": "0"},
			})
		})
	}
}

func assertPingTimezoneHistory(t *testing.T, result map[string][]string, location *time.Location, startLabel string, size int, measured map[int]map[string]string) {
	t.Helper()
	start, err := time.ParseInLocation("2006-01-02 15:04", startLabel, location)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"lastcheck", "maxdelay", "mindelay", "avgdelay", "losspk"} {
		values := result[key]
		if len(values) != size {
			t.Fatalf("%s has %d values, want %d", key, len(values), size)
		}
		for i, value := range values {
			want := "-"
			if key == "lastcheck" {
				want = start.Add(time.Duration(i) * time.Minute).Format("2006-01-02 15:04")
			} else if sample := measured[i]; sample != nil {
				want = sample[key]
			}
			if value != want {
				t.Errorf("%s[%d]=%q at %s, want %q", key, i, value, result["lastcheck"][i], want)
			}
		}
	}
}
