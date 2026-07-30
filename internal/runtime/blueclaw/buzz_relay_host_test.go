package blueclaw

import "testing"

func TestDeriveRelayPublicHost(t *testing.T) {
	cases := map[string]string{
		"example-device.example.test":          "example-device-relay.example.test",
		"https://example-device.example.test":  "example-device-relay.example.test",
		"https://example-device.example.test/": "example-device-relay.example.test",
		"localhost":                        "localhost-relay",
		"":                                 "",
		"   ":                              "",
	}
	for input, expected := range cases {
		if actual := DeriveRelayPublicHost(input); actual != expected {
			t.Errorf("DeriveRelayPublicHost(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestDeriveRelayPublicURL(t *testing.T) {
	if actual := DeriveRelayPublicURL("example-device.example.test"); actual != "wss://example-device-relay.example.test" {
		t.Errorf("DeriveRelayPublicURL host = %q", actual)
	}
	if actual := DeriveRelayPublicURL("wss://example-device.example.test"); actual != "wss://example-device-relay.example.test" {
		t.Errorf("DeriveRelayPublicURL url = %q", actual)
	}
	if actual := DeriveRelayPublicURL(""); actual != "" {
		t.Errorf("DeriveRelayPublicURL empty = %q, want empty", actual)
	}
}
