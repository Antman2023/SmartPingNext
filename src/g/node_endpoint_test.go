package g

import (
	"sync"
	"testing"
)

func TestConfiguredNodeEndpoint(t *testing.T) {
	withGlobalConfigState(t, func() {
		SetConfig(Config{Port: 8899, Network: map[string]NetworkMember{
			"192.0.2.1": {Addr: "192.0.2.1"},
			"Alias":     {Addr: " NODE.Example "},
			"ipv6":      {Addr: "[2001:0db8:0:0:0:0:0:1]"},
		}})
		for _, test := range []struct {
			host               string
			port               int
			wantHost, wantPort bool
		}{
			{"192.0.2.1", 8899, true, true},
			{"::ffff:192.0.2.1", 8899, true, true},
			{"node.example", 8899, true, true},
			{"alias", 8899, true, true},
			{"2001:db8::1", 8899, true, true},
			{"192.0.2.2", 8899, false, true},
			{"", 8899, false, true},
			{"192.0.2.1", 8080, true, false},
			{"192.0.2.1", 0, true, false},
		} {
			host, port := ConfiguredNodeEndpoint(test.host, test.port)
			if host != test.wantHost || port != test.wantPort {
				t.Errorf("endpoint %s:%d = (%v, %v), want (%v, %v)", test.host, test.port, host, port, test.wantHost, test.wantPort)
			}
		}
		SetConfig(Config{})
		if host, port := ConfiguredNodeEndpoint("192.0.2.1", 8899); host || port {
			t.Fatal("removed endpoint remained allowed")
		}
	})
}

func TestConfiguredNodeEndpointUsesOneConfigVersion(t *testing.T) {
	withGlobalConfigState(t, func() {
		allowedHost := Config{Port: 8899, Network: map[string]NetworkMember{"192.0.2.1": {Addr: "192.0.2.1"}}}
		allowedPort := Config{Port: 8080, Network: map[string]NetworkMember{"192.0.2.2": {Addr: "192.0.2.2"}}}
		SetConfig(allowedHost)
		var workers sync.WaitGroup
		workers.Add(2)
		go func() {
			defer workers.Done()
			for i := 0; i < 1000; i++ {
				SetConfig(allowedHost)
				SetConfig(allowedPort)
			}
		}()
		go func() {
			defer workers.Done()
			for i := 0; i < 1000; i++ {
				if host, port := ConfiguredNodeEndpoint("192.0.2.1", 8080); host && port {
					t.Error("accepted host and port from different config versions")
					return
				}
			}
		}()
		workers.Wait()
	})
}
