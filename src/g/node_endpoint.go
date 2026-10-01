package g

import (
	"net"
	"strings"
)

// ConfiguredNodeEndpoint checks the host and port against one configuration
// version without cloning topology rules, ping lists or map probe targets.
func ConfiguredNodeEndpoint(host string, port int) (hostAllowed, portAllowed bool) {
	host = normalizeEndpointHost(host)
	CfgLock.RLock()
	defer CfgLock.RUnlock()
	portAllowed = port > 0 && port == Cfg.Port
	if host == "" {
		return false, portAllowed
	}
	if _, exists := Cfg.Network[host]; exists {
		return true, portAllowed
	}
	for key, member := range Cfg.Network {
		if normalizeEndpointHost(key) == host || normalizeEndpointHost(member.Addr) == host {
			return true, portAllowed
		}
	}
	return false, portAllowed
}

func normalizeEndpointHost(host string) string {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if parsedIP := net.ParseIP(host); parsedIP != nil {
		if ipv4 := parsedIP.To4(); ipv4 != nil {
			return ipv4.String()
		}
		return parsedIP.String()
	}
	return strings.ToLower(host)
}
