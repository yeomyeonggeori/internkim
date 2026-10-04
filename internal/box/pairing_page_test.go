package box

import "testing"

func TestTheMulticastNameIsReadFromBusctl(t *testing.T) {
	cases := map[string]string{
		"s \"kimmini-2\"\n":     "kimmini-2",
		"s \"Kimmini.local\"\n": "kimmini",
		"":                      "",
		"Failed to connect\n":   "",
		"u 4\n":                 "",
	}
	for output, want := range cases {
		if got := hostNameFromBusctl(output); got != want {
			t.Errorf("host name from %q = %q, want %q", output, got, want)
		}
	}
}
