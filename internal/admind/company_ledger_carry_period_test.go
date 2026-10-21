package admind

import "testing"

func TestLegacyMetricPeriodNamesTheMomentTheDeviceKept(t *testing.T) {
	for _, example := range []struct {
		year, quarter, month int
		period               string
	}{
		{2025, 0, 0, "2025"},
		{2025, 3, 0, "2025-Q3"},
		{2026, 0, 4, "2026-04"},
	} {
		if period := legacyMetricPeriod(example.year, example.quarter, example.month); period != example.period {
			t.Fatalf("legacyMetricPeriod(%d, %d, %d) = %q, want %q", example.year, example.quarter, example.month, period, example.period)
		}
	}
}
