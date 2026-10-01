package http

import (
	"fmt"
	"smartping/src/g"
	"testing"
)

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
