package g

import "testing"

func TestPingLossPreservesIntegerPercentageBoundaries(t *testing.T) {
	for _, test := range []struct {
		name                 string
		sent, received, want int
	}{
		{"healthy", 100, 100, 0},
		{"complete loss", 100, 0, 100},
		{"29 percent", 100, 71, 29},
		{"57 percent", 100, 43, 57},
		{"58 percent", 50, 21, 58},
		{"fraction rounds down", 3, 2, 33},
		{"fraction below threshold", 7, 5, 28},
	} {
		t.Run(test.name, func(t *testing.T) {
			stat := PingSt{SendPk: test.sent, RevcPk: test.received}
			stat.UpdateLoss()
			if stat.LossPk != test.want {
				t.Fatalf("loss = %d%%, want %d%%", stat.LossPk, test.want)
			}
		})
	}
}
