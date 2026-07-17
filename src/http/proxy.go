package http

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"smartping/src/g"
)

type proxyQueryRule struct {
	required map[string]struct{}
	allowed  map[string]struct{}
}

var proxyTargetRules = map[string]proxyQueryRule{
	"/api/config.json":   newProxyQueryRule(nil, nil),
	"/api/topology.json": newProxyQueryRule(nil, nil),
	"/api/alert.json":    newProxyQueryRule(nil, []string{"date"}),
	"/api/mapping.json":  newProxyQueryRule(nil, []string{"d"}),
	"/api/ping.json":     newProxyQueryRule([]string{"ip"}, []string{"starttime", "endtime"}),
	"/api/tools.json":    newProxyQueryRule([]string{"t"}, nil),
}

func newProxyQueryRule(required []string, optional []string) proxyQueryRule {
	allowed := make(map[string]struct{}, len(required)+len(optional))
	requiredMap := make(map[string]struct{}, len(required))
	for _, key := range required {
		requiredMap[key] = struct{}{}
		allowed[key] = struct{}{}
	}
	for _, key := range optional {
		allowed[key] = struct{}{}
	}
	return proxyQueryRule{
		required: requiredMap,
		allowed:  allowed,
	}
}

func validateProxyTarget(rawTarget string) (*url.URL, error) {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		return nil, errors.New("Url Param Error!")
	}

	targetURL, err := url.Parse(target)
	if err != nil || !targetURL.IsAbs() {
		return nil, errors.New("Url Param Error!")
	}
	if targetURL.User != nil || targetURL.Fragment != "" {
		return nil, errors.New("Proxy Target Not Allowed!")
	}

	scheme := strings.ToLower(targetURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, errors.New("Proxy Target Not Allowed!")
	}

	if !isConfiguredProxyTargetHost(targetURL.Hostname()) {
		return nil, errors.New("Proxy Target Host Not Allowed!")
	}

	if !isConfiguredProxyTargetPort(targetURL.Port(), scheme) {
		return nil, errors.New("Proxy Target Port Not Allowed!")
	}

	rule, ok := proxyTargetRules[targetURL.Path]
	if !ok {
		return nil, errors.New("Proxy Target API Not Allowed!")
	}

	queryValues := targetURL.Query()
	if err := rule.validate(queryValues); err != nil {
		return nil, err
	}

	targetURL.RawQuery = queryValues.Encode()
	return targetURL, nil
}

func (r proxyQueryRule) validate(values url.Values) error {
	for key := range values {
		if _, ok := r.allowed[key]; !ok {
			return errors.New("Proxy Target Query Not Allowed!")
		}
	}

	for key := range r.required {
		if strings.TrimSpace(values.Get(key)) == "" {
			return errors.New("Proxy Target Query Not Allowed!")
		}
	}

	return nil
}

func isConfiguredProxyTargetHost(host string) bool {
	normalizedHost := normalizeProxyTargetHost(host)
	if normalizedHost == "" {
		return false
	}

	for key, member := range g.Cfg.Network {
		if normalizeProxyTargetHost(key) == normalizedHost {
			return true
		}
		if normalizeProxyTargetHost(member.Addr) == normalizedHost {
			return true
		}
	}

	return false
}

func normalizeProxyTargetHost(host string) string {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return ""
	}

	parsedIP := net.ParseIP(host)
	if parsedIP == nil {
		return strings.ToLower(host)
	}

	if ipv4 := parsedIP.To4(); ipv4 != nil {
		return ipv4.String()
	}

	return parsedIP.String()
}

func isConfiguredProxyTargetPort(port string, scheme string) bool {
	if port == "" {
		return false
	}

	targetPort, err := strconv.Atoi(port)
	if err != nil || targetPort <= 0 {
		return false
	}

	return targetPort == g.Cfg.Port
}
