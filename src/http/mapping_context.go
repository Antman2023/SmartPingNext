package http

import (
	"context"
	"encoding/json"
	"errors"
	"smartping/src/g"
)

func decodeMappingData(raw string) (map[string][]g.MapVal, error) {
	return decodeMappingDataContext(context.Background(), raw)
}

func decodeMappingDataContext(ctx context.Context, raw string) (map[string][]g.MapVal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Pointers distinguish an actual zero measurement from missing or null fields.
	type storedSample struct {
		Name  *string  `json:"name"`
		Value *float64 `json:"value"`
	}
	stored := make(map[string][]storedSample)
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, err
	}
	// Standard JSON decoding cannot be interrupted; skip normalization if the
	// request ended during decoding, and never return a partially copied result.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	normalized := emptyMappingData()
	for carrier := range normalized {
		samples := stored[carrier]
		var values []g.MapVal
		for index, sample := range samples {
			if index&255 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			if sample.Name == nil || sample.Value == nil {
				return nil, errors.New("mapping sample is missing a name or value")
			}
			if *sample.Value < 0 {
				return nil, errors.New("mapping sample has a negative delay")
			}
			if index == 0 {
				// Allocate only after the first sample passes cancellation and
				// validation, using the already decoded carrier length.
				values = make([]g.MapVal, len(samples))
			}
			values[index] = g.MapVal{Name: *sample.Name, Value: *sample.Value}
		}
		if len(values) > 0 {
			normalized[carrier] = values
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return normalized, nil
}
