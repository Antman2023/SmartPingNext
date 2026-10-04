package g

import "context"

// ConfigSnapshotContext copies one configuration version for a request.
func ConfigSnapshotContext(ctx context.Context) (Config, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, err
	}
	CfgLock.RLock()
	defer CfgLock.RUnlock()
	return cloneConfigContext(ctx, Cfg)
}

// Waiting for CfgLock is not interruptible; check again before allocating.
func cloneConfigContext(ctx context.Context, config Config) (Config, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, err
	}
	if ctx.Done() == nil {
		return cloneConfig(config), nil
	}
	cloned := config
	var err error
	if cloned.Mode, err = cloneSnapshotMapContext(ctx, config.Mode, nil); err != nil {
		return Config{}, err
	}
	if cloned.Base, err = cloneSnapshotMapContext(ctx, config.Base, nil); err != nil {
		return Config{}, err
	}
	if cloned.Topology, err = cloneSnapshotMapContext(ctx, config.Topology, nil); err != nil {
		return Config{}, err
	}
	cloned.Network, err = cloneSnapshotMapContext(ctx, config.Network, func(member NetworkMember) (NetworkMember, error) {
		copy := member
		var err error
		if copy.Ping, err = cloneSnapshotStringsContext(ctx, member.Ping); err != nil {
			return NetworkMember{}, err
		}
		if member.Topology != nil {
			copy.Topology = make([]map[string]string, len(member.Topology))
			for i, rule := range member.Topology {
				copy.Topology[i], err = cloneSnapshotMapContext(ctx, rule, nil)
				if err != nil {
					return NetworkMember{}, err
				}
			}
		}
		return copy, ctx.Err()
	})
	if err != nil {
		return Config{}, err
	}
	cloned.Chinamap, err = cloneSnapshotMapContext(ctx, config.Chinamap, func(providers map[string][]string) (map[string][]string, error) {
		// Match cloneConfig: nil inner maps become empty maps.
		if providers == nil {
			return make(map[string][]string), ctx.Err()
		}
		return cloneSnapshotMapContext(ctx, providers, func(addresses []string) ([]string, error) {
			return cloneSnapshotStringsContext(ctx, addresses)
		})
	})
	if err != nil {
		return Config{}, err
	}
	if err := ctx.Err(); err != nil {
		return Config{}, err
	}
	return cloned, nil
}

func cloneSnapshotMapContext[K comparable, V any](ctx context.Context, values map[K]V, copyValue func(V) (V, error)) (map[K]V, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if values == nil {
		return nil, nil
	}
	cloned := make(map[K]V, len(values))
	i := 0
	for key, value := range values {
		if i%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		i++
		if copyValue != nil {
			var err error
			value, err = copyValue(value)
			if err != nil {
				return nil, err
			}
		}
		cloned[key] = value
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return cloned, nil
}

func cloneSnapshotStringsContext(ctx context.Context, values []string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if values == nil {
		return nil, nil
	}
	cloned := make([]string, len(values))
	for start := 0; start < len(values); start += 256 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		end := start + 256
		if end > len(values) {
			end = len(values)
		}
		copy(cloned[start:end], values[start:end])
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return cloned, nil
}
