package funcs

import (
	"smartping/src/g"
	"testing"
)

func TestAggregateMappingDelayIgnoresQuarterOfFailedProbes(t *testing.T) {
	stats := []g.PingSt{
		{AvgDelay: 2000, LossPk: 100},
		{AvgDelay: 20, RevcPk: 1},
		{AvgDelay: 30, RevcPk: 1},
	}

	if got := aggregateMappingDelay(stats); got != 25 {
		t.Fatalf("aggregateMappingDelay() = %.2f, want 25.00", got)
	}
}

func TestAggregateMappingDelayPenalizesFailuresBeyondAllowance(t *testing.T) {
	stats := []g.PingSt{
		{AvgDelay: 2000, LossPk: 100},
		{AvgDelay: 20, RevcPk: 1},
		{AvgDelay: 2000, LossPk: 100},
		{AvgDelay: 40, RevcPk: 1},
	}

	if got := aggregateMappingDelay(stats); got != 686.67 {
		t.Fatalf("aggregateMappingDelay() = %.2f, want 686.67", got)
	}
}

func TestAggregateMappingDelayUsesFailureSentinelWhenAllProbesFail(t *testing.T) {
	stats := []g.PingSt{
		{AvgDelay: 2000, LossPk: 100},
		{AvgDelay: 2000, LossPk: 100},
		{AvgDelay: 2000, LossPk: 100},
	}

	if got := aggregateMappingDelay(stats); got != 2000 {
		t.Fatalf("aggregateMappingDelay() = %.2f, want 2000.00", got)
	}
}
