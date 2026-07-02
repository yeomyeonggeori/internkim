package browser

import "testing"

func TestScaleToAbsoluteAxisClampsAndScales(t *testing.T) {
	testCases := []struct {
		name     string
		value    int
		origin   int
		extent   int
		expected uint16
	}{
		{name: "at origin maps to zero", value: 0, origin: 0, extent: 1920, expected: 0},
		{name: "before origin clamps to zero", value: -50, origin: 0, extent: 1920, expected: 0},
		{name: "at far edge maps to max", value: 1919, origin: 0, extent: 1920, expected: 65535},
		{name: "past far edge clamps to max", value: 5000, origin: 0, extent: 1920, expected: 65535},
		{name: "midpoint scales proportionally", value: 960, origin: 0, extent: 1920, expected: 32784},
		{name: "negative origin offsets correctly", value: 0, origin: -1920, extent: 1920, expected: 65535},
		{name: "degenerate extent returns zero", value: 500, origin: 0, extent: 1, expected: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := scaleToAbsoluteAxis(testCase.value, testCase.origin, testCase.extent)
			if actual != testCase.expected {
				t.Fatalf("scaleToAbsoluteAxis(%d, %d, %d) = %d, want %d", testCase.value, testCase.origin, testCase.extent, actual, testCase.expected)
			}
		})
	}
}
