package http

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"smartping/src/g"
)

const (
	maxProxyResponseBytes      = 16 << 20
	maxProxyTimeoutSeconds     = 60
	maxConcurrentProxyRequests = 32
)

var proxyRequestSlots = make(chan struct{}, maxConcurrentProxyRequests)

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

func normalizeProxyTimeout(seconds int) int {
	if seconds < 1 {
		return 10
	}
	if seconds > maxProxyTimeoutSeconds {
		return maxProxyTimeoutSeconds
	}
	return seconds
}

func acquireProxyRequest() bool {
	select {
	case proxyRequestSlots <- struct{}{}:
		return true
	default:
		return false
	}
}

func releaseProxyRequest() {
	<-proxyRequestSlots
}

func readProxyResponseBody(reader io.Reader) ([]byte, error) {
	limited := io.LimitReader(reader, maxProxyResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(body) > maxProxyResponseBytes {
		return nil, errors.New("Proxy Response Too Large!")
	}
	return body, nil
}

func readProxyHTTPResponseBody(response *http.Response) ([]byte, error) {
	if response == nil || response.Body == nil {
		return nil, errors.New("Proxy Response Body Missing!")
	}
	if response.ContentLength > maxProxyResponseBytes {
		return nil, errors.New("Proxy Response Too Large!")
	}
	return readProxyResponseBody(response.Body)
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

	config := g.ConfigSnapshot()
	if !isConfiguredProxyTargetHost(targetURL.Hostname(), config) {
		return nil, errors.New("Proxy Target Host Not Allowed!")
	}

	if !isConfiguredProxyTargetPort(targetURL.Port(), config) {
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

func isConfiguredProxyTargetHost(host string, config g.Config) bool {
	normalizedHost := normalizeProxyTargetHost(host)
	if normalizedHost == "" {
		return false
	}

	for key, member := range config.Network {
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

func isConfiguredProxyTargetPort(port string, config g.Config) bool {
	if port == "" {
		return false
	}

	targetPort, err := strconv.Atoi(port)
	if err != nil || targetPort <= 0 {
		return false
	}

	return targetPort == config.Port
}
