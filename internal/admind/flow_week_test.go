package admind

import "testing"

func TestCanonicalWeekCode(t *testing.T) {
	cases := map[string]string{
		"26W25":    "26W25",
		"26w25":    "26W25",
		"2026-25":  "26W25",
		"2026W25":  "26W25",
		"2026-W24": "26W24",
		"2026-W4":  "26W04",
		"  26W25 ": "26W25",
		"hello":    "",
		"2026-00":  "",
		"2026-54":  "",
	}
	for input, expected := range cases {
		if actual := canonicalWeekCode(input); actual != expected {
			t.Errorf("canonicalWeekCode(%q) = %q, want %q", input, actual, expected)
		}
	}
}
