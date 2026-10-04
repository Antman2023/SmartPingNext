package http

import (
	"context"
	"strings"
	"time"
)

type pingHistoryTimeline struct {
	lastcheck, maxdelay, mindelay, avgdelay, losspk []string
	populated                                       []bool
	queryStartLabel, queryEndLabel                  string
}

func newPingHistoryTimelineContext(ctx context.Context, start time.Time, size int) (pingHistoryTimeline, error) {
	if err := ctx.Err(); err != nil {
		return pingHistoryTimeline{}, err
	}
	timeline := pingHistoryTimeline{
		lastcheck: make([]string, size), maxdelay: make([]string, size), mindelay: make([]string, size),
		avgdelay: make([]string, size), losspk: make([]string, size), populated: make([]bool, size),
	}
	const minuteLayout = "2006-01-02 15:04"
	var labels strings.Builder
	var minuteBuffer [32]byte
	cursor := start.Unix()
	for i := 0; i < size; i++ {
		// Check between bounded batches rather than for every formatted minute.
		if i%256 == 0 {
			if i > 0 {
				if err := ctx.Err(); err != nil {
					return pingHistoryTimeline{}, err
				}
			}
			// Keep label storage bounded by the same cancellation batch. Reset
			// releases the builder's buffer without changing earlier strings.
			labels.Reset()
			labels.Grow(min(size-i, 256) * len(minuteLayout))
		}
		// Append to a request-local string buffer. Builder appends preserve all
		// previously returned strings, including when a wider year grows it.
		offset := labels.Len()
		labels.Write(time.Unix(cursor, 0).In(start.Location()).AppendFormat(minuteBuffer[:0], minuteLayout))
		stamp := labels.String()[offset:]
		timeline.lastcheck[i] = stamp
		// A clock rollback can put interior labels outside the endpoint
		// labels. Bound the SQL scan by every minute in the actual timeline.
		if i == 0 || stamp < timeline.queryStartLabel {
			timeline.queryStartLabel = stamp
		}
		if i == 0 || stamp > timeline.queryEndLabel {
			timeline.queryEndLabel = stamp
		}
		// Missing samples must remain gaps, not healthy zero-valued measurements.
		timeline.maxdelay[i] = "-"
		timeline.mindelay[i] = "-"
		timeline.avgdelay[i] = "-"
		timeline.losspk[i] = "-"
		cursor += 60
	}
	if err := ctx.Err(); err != nil {
		return pingHistoryTimeline{}, err
	}
	return timeline, nil
}
