package nettools

import (
	"strings"
	"testing"
	"time"
)

func TestRunMtrInvalidHost(t *testing.T) {
	res, err := RunMtr("invalid host !@", time.Second, 8, 3)
	if err == nil {
		t.Fatalf("RunMtr should fail for invalid host")
	}
	if !strings.Contains(err.Error(), "Unable to resolve destination host") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("result should be empty on resolve failure")
	}
}

func TestRunMtrZeroTTL(t *testing.T) {
	res, err := RunMtr("127.0.0.1", time.Second, 0, 3)
	if err != nil {
		t.Fatalf("RunMtr with maxttl=0 should not error, got: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("RunMtr with maxttl=0 should return empty result")
	}
}
