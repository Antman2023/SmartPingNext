package http

import "sort"

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
	return pingTimelineIndex{timestamps: timestamps, ordered: sort.StringsAreSorted(timestamps)}
}

func (index *pingTimelineIndex) lookup(timestamp string) (int, bool) {
	if !index.ordered {
		if index.positions == nil {
			index.positions = make(map[string]int, len(index.timestamps))
			for position, stamp := range index.timestamps {
				index.positions[stamp] = position
			}
		}
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
