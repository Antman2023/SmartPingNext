package g

import "context"

// PingTarget retains the configured key for diagnostics and its resolved node
// address. Missing nodes have an empty address and are skipped by the round.
type PingTarget struct {
	Key  string
	Addr string
}

type PingRoundConfig struct {
	Base    map[string]int
	Targets []PingTarget
}

// PingRoundSnapshotContext reads scheduling settings and ordered targets from
// one config version. It does not copy topology rules or map-probe data.
// Waiting for CfgLock remains uninterruptible.
func PingRoundSnapshotContext(ctx context.Context) (PingRoundConfig, error) {
	if err := ctx.Err(); err != nil {
		return PingRoundConfig{}, err
	}
	CfgLock.RLock()
	defer CfgLock.RUnlock()
	if err := ctx.Err(); err != nil {
		return PingRoundConfig{}, err
	}
	result := PingRoundConfig{Base: make(map[string]int, 4)}
	for _, key := range []string{"PingCount", "PingIntervalMs", "PingTimeoutMs", "PingStaggerMs"} {
		if value, exists := Cfg.Base[key]; exists {
			result.Base[key] = value
		}
	}
	if targets := Cfg.Network[Cfg.Addr].Ping; targets != nil {
		result.Targets = make([]PingTarget, len(targets))
		for i, key := range targets {
			if err := ctx.Err(); err != nil {
				return PingRoundConfig{}, err
			}
			result.Targets[i] = PingTarget{Key: key, Addr: Cfg.Network[key].Addr}
		}
	}
	if err := ctx.Err(); err != nil {
		return PingRoundConfig{}, err
	}
	return result, nil
}
