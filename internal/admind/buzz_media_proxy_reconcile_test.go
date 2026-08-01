package admind

import "testing"

func TestRelayOriginForDeviceURL(t *testing.T) {
	cases := map[string]string{
		"https://example-device.example.test":  "https://example-device-relay.example.test",
		"https://foo.example.test":           "https://foo-relay.example.test",
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
