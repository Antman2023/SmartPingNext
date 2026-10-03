package g

// ConfigMetadata contains only immutable scalar settings. Reading it does not
// copy every network node, topology rule and map probe target.
type ConfigMetadata struct {
	Ver       string
	Port      int
	Name      string
	Addr      string
	Toollimit int
	Password  string
}

// ConfigMetadataSnapshot reads related settings from one configuration version.
// It is for internal consumers, not an API response (it includes the password).
func ConfigMetadataSnapshot() ConfigMetadata {
	CfgLock.RLock()
	defer CfgLock.RUnlock()
	return ConfigMetadata{
		Ver: Cfg.Ver, Port: Cfg.Port, Name: Cfg.Name, Addr: Cfg.Addr,
		Toollimit: Cfg.Toollimit, Password: Cfg.Password,
	}
}

// LocalNetworkSnapshot copies just the local member and its rules under the
// same lock as the local address. Callers may mutate the returned member safely.
func LocalNetworkSnapshot() (string, NetworkMember) {
	CfgLock.RLock()
	defer CfgLock.RUnlock()
	return Cfg.Addr, cloneNetworkMember(Cfg.Network[Cfg.Addr])
}
