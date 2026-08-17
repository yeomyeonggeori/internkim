package blueclaw

import "testing"

func TestRelayPublicHost(t *testing.T) {
	cases := map[string]string{
		"relay.example.test":         "relay.example.test",
		"https://relay.example.test": "relay.example.test",
		"wss://relay.example.test":   "relay.example.test",
		"wss://relay.example.test/":  "relay.example.test",
		"  relay.example.test  ":     "relay.example.test",
		"":                           "",
		"   ":                        "",
	}
	for input, expected := range cases {
		if actual := RelayPublicHost(input); actual != expected {
			t.Errorf("RelayPublicHost(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestRelayPublicURL(t *testing.T) {
	if actual := RelayPublicURL("relay.example.test"); actual != "wss://relay.example.test" {
		t.Errorf("RelayPublicURL host = %q", actual)
	}
	if actual := RelayPublicURL("wss://relay.example.test"); actual != "wss://relay.example.test" {
		t.Errorf("RelayPublicURL url = %q", actual)
	}
	if actual := RelayPublicURL(""); actual != "" {
		t.Errorf("RelayPublicURL without a domain = %q, want empty", actual)
	}
}
