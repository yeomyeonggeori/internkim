package admind

import "testing"

func TestRelayOriginForDeviceURL(t *testing.T) {
	cases := map[string]string{
		"https://zd2df6qt6jmc.intern.kim":  "https://zd2df6qt6jmc-relay.intern.kim",
		"https://foo.intern.kim":           "https://foo-relay.intern.kim",
		"http://box.local":                 "http://box-relay.local",
		"https://single":                   "https://single-relay",
		"":                                 "",
	}
	for input, want := range cases {
		if got := relayOriginForDeviceURL(input); got != want {
			t.Errorf("relayOriginForDeviceURL(%q) = %q, want %q", input, got, want)
		}
	}
}
