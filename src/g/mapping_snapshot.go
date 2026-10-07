package g

import "context"

type MappingRoundConfig struct {
	Base     map[string]int
	Chinamap map[string]map[string][]string
}

// MappingRoundSnapshotContext copies map targets and their scheduling settings
// from one version, without copying unrelated network members or alert rules.
// Waiting for CfgLock remains uninterruptible.
func MappingRoundSnapshotContext(ctx context.Context) (MappingRoundConfig, error) {
	if err := ctx.Err(); err != nil {
		return MappingRoundConfig{}, err
	}
	CfgLock.RLock()
	defer CfgLock.RUnlock()
	// Reuse the existing map copy semantics, including nil inner maps becoming
	// empty maps and cancellation checks while copying long address lists.
	cloned, err := cloneConfigContext(ctx, Config{Chinamap: Cfg.Chinamap})
	if err != nil {
		return MappingRoundConfig{}, err
	}
	result := MappingRoundConfig{Base: make(map[string]int, 2), Chinamap: cloned.Chinamap}
	for _, key := range []string{"MappingConcurrency", "MappingProbeCount"} {
		if value, exists := Cfg.Base[key]; exists {
			result.Base[key] = value
		}
	}
	if err := ctx.Err(); err != nil {
		return MappingRoundConfig{}, err
	}
	return result, nil
}
