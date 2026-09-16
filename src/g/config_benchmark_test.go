package g

import (
	"strconv"
	"testing"
)

func BenchmarkGetBaseInt(b *testing.B) {
	for _, nodes := range []int{1, 100, 1000} {
		b.Run(strconv.Itoa(nodes)+"Nodes", func(b *testing.B) {
			previous := ConfigSnapshot()
			b.Cleanup(func() { SetConfig(previous) })
			config := Config{Base: map[string]int{"Timeout": 10}, Network: make(map[string]NetworkMember)}
			for i := 0; i < nodes; i++ {
				address := "10.0." + strconv.Itoa(i/256) + "." + strconv.Itoa(i%256)
				config.Network[address] = NetworkMember{
					Addr:     address,
					Ping:     []string{"127.0.0.1"},
					Topology: []map[string]string{{"Addr": "127.0.0.1", "Thdloss": "10"}},
				}
			}
			SetConfig(config)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if got := GetBaseInt("Timeout", 20); got != 10 {
					b.Fatalf("timeout = %d, want 10", got)
				}
			}
		})
	}
}
