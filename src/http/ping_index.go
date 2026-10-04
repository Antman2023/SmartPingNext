package http

import (
	"context"
	"sort"
)

// pingTimelineIndex reuses the timeline for ordered minute labels. Most SQL
// rows arrive in minute order, so an adjacent match avoids even a binary search.
// Non-monotonic labels (for example during a clock rollback) keep map semantics.
type pingTimelineIndex struct {
	timestamps []string
	ordered    bool
	next       int
	positions  map[string]int
}

func newPingTimelineIndex(timestamps []string) pingTimelineIndex {
	index, _ := newPingTimelineIndexContext(context.Background(), timestamps)
	return index
}

func newPingTimelineIndexContext(ctx context.Context, timestamps []string) (pingTimelineIndex, error) {
	if err := ctx.Err(); err != nil {
		return pingTimelineIndex{}, err
	}
	index := pingTimelineIndex{timestamps: timestamps, ordered: true}
	// Match sorted-label detection without leaving a long scan uninterruptible.
	for position := len(timestamps) - 1; position > 0; position-- {
		if position&255 == 0 {
			if err := ctx.Err(); err != nil {
				return pingTimelineIndex{}, err
			}
		}
		if timestamps[position] < timestamps[position-1] {
			index.ordered = false
			break
		}
	}
	if !index.ordered {
		if err := ctx.Err(); err != nil {
			return pingTimelineIndex{}, err
		}
		// The HTTP handler constructs the index only after a sample is read.
		// Prepare fallback positions here so lookups never build an unchecked map.
		positions := make(map[string]int, len(timestamps))
		for position, stamp := range timestamps {
			if position&255 == 0 {
				if err := ctx.Err(); err != nil {
					return pingTimelineIndex{}, err
				}
			}
			positions[stamp] = position
		}
		index.positions = positions
	}
	if err := ctx.Err(); err != nil {
		return pingTimelineIndex{}, err
	}
	return index, nil
}

func (index *pingTimelineIndex) lookup(timestamp string) (int, bool) {
	if !index.ordered {
		position, found := index.positions[timestamp]
		return position, found
	}

	position := index.next
	if position < len(index.timestamps) && index.timestamps[position] == timestamp {
		// Preserve the old index's last-occurrence behavior for duplicate labels.
		for position+1 < len(index.timestamps) && index.timestamps[position+1] == timestamp {
			position++
		}
	} else {
		position = sort.Search(len(index.timestamps), func(i int) bool {
			return index.timestamps[i] > timestamp
		}) - 1
		if position < 0 || index.timestamps[position] != timestamp {
			return 0, false
		}
	}
	index.next = position + 1
	return position, true
}
