package http

import (
	"bytes"
	"context"
	"errors"
	"io"
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
	return readProxyResponseBodyWithLength(reader, -1)
}

func readProxyResponseBodyWithLength(reader io.Reader, contentLength int64) ([]byte, error) {
	if contentLength > maxProxyResponseBytes {
		return nil, errors.New("Proxy Response Too Large!")
	}
	return readLimitedProxyResponseBody(io.LimitReader(reader, maxProxyResponseBytes+1), contentLength)
}

// Both callers validate the length hint and supply a reader with the same
// maximum-byte sentinel. Buffer growth and actual-length checks stay shared.
func readLimitedProxyResponseBody(limited io.Reader, contentLength int64) ([]byte, error) {
	var body []byte
	var err error
	if contentLength >= bytes.MinRead {
		// Reserve EOF-read space plus the size-limit sentinel, so a response
		// declaring the maximum length needs no second large buffer to check it.
		buffer := bytes.NewBuffer(make([]byte, 0, int(contentLength)+bytes.MinRead+1))
		_, err = buffer.ReadFrom(limited)
		body = buffer.Bytes()
	} else {
		body, err = io.ReadAll(limited)
	}
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
	return readProxyResponseBodyWithLength(response.Body, response.ContentLength)
}

func readProxyHTTPResponseBodyContext(ctx context.Context, response *http.Response) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if response == nil || response.Body == nil {
		return nil, errors.New("Proxy Response Body Missing!")
	}
	if response.ContentLength > maxProxyResponseBytes {
		return nil, errors.New("Proxy Response Too Large!")
	}
	var limited io.Reader
	if ctx.Done() != nil {
		// Keep cancellation and the byte budget in one request-local object.
		limited = &proxyContextReader{ctx: ctx,
			limited: io.LimitedReader{R: response.Body, N: maxProxyResponseBytes + 1}}
	} else {
		limited = io.LimitReader(response.Body, maxProxyResponseBytes+1)
	}
	body, err := readLimitedProxyResponseBody(limited, response.ContentLength)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return body, nil
}

type proxyContextReader struct {
	ctx     context.Context
	limited io.LimitedReader
}

func (reader *proxyContextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	// Bound reads so even a buffered response checks cancellation between
	// chunks. The HTTP transport still handles cancellation of a blocked read.
	const maxReadBytes = 32 << 10
	if len(buffer) > maxReadBytes {
		buffer = buffer[:maxReadBytes]
	}
	count, err := reader.limited.Read(buffer)
	if err == nil || err == io.EOF {
		if canceled := reader.ctx.Err(); canceled != nil {
			return count, canceled
		}
	}
	return count, err
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

	port, _ := strconv.Atoi(targetURL.Port())
	hostAllowed, portAllowed := g.ConfiguredNodeEndpoint(targetURL.Hostname(), port)
	if !hostAllowed {
		return nil, errors.New("Proxy Target Host Not Allowed!")
	}

	if !portAllowed {
		return nil, errors.New("Proxy Target Port Not Allowed!")
	}

	rule, ok := proxyTargetRules[targetURL.Path]
	if !ok {
		return nil, errors.New("Proxy Target API Not Allowed!")
	}

	queryValues, err := url.ParseQuery(targetURL.RawQuery)
	if err != nil {
		return nil, errors.New("Proxy Target Query Not Allowed!")
	}
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
