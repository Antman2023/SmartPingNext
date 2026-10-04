package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"smartping/src/g"
	"testing"
)

func BenchmarkProxyResponseBody(b *testing.B) {
	for _, size := range []int{128, 1 << 10, 64 << 10, 1 << 20, maxProxyResponseBytes} {
		payload := bytes.Repeat([]byte("x"), size)
		for _, mode := range []string{"ReadAll", "KnownLength", "UnknownLength"} {
			b.Run(fmt.Sprintf("%dBytes/%s", size, mode), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(size))
				for i := 0; i < b.N; i++ {
					reader := bytes.NewReader(payload)
					var body []byte
					var err error
					if mode == "ReadAll" {
						body, err = io.ReadAll(io.LimitReader(io.NopCloser(reader), maxProxyResponseBytes+1))
					} else {
						length := int64(size)
						if mode == "UnknownLength" {
							length = -1
						}
						body, err = readProxyHTTPResponseBody(&http.Response{
							ContentLength: length, Body: io.NopCloser(reader),
						})
					}
					if err != nil || len(body) != size {
						b.Fatalf("body length = %d, error = %v", len(body), err)
					}
				}
			})
		}
	}
}

func BenchmarkValidateProxyTarget(b *testing.B) {
	for _, count := range []int{1, 100, 1000} {
		b.Run(fmt.Sprintf("%dNodes", count), func(b *testing.B) {
			previous := g.ConfigSnapshot()
			b.Cleanup(func() { g.SetConfig(previous) })
			config := g.Config{Port: 8899, Network: make(map[string]g.NetworkMember, count)}
			for i := 0; i < count; i++ {
				address := fmt.Sprintf("10.0.%d.%d", i/256, i%256)
				config.Network[address] = g.NetworkMember{
					Addr: address, Ping: []string{"127.0.0.1"},
					Topology: []map[string]string{{"Addr": "127.0.0.1", "Thdloss": "30"}},
				}
			}
			g.SetConfig(config)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := validateProxyTarget("http://10.0.0.0:8899/api/config.json"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
