package blueclaw

import "testing"

func TestDeriveRelayPublicHost(t *testing.T) {
	cases := map[string]string{
		"zd2df6qt6jmc.intern.kim":          "zd2df6qt6jmc-relay.intern.kim",
		"https://zd2df6qt6jmc.intern.kim":  "zd2df6qt6jmc-relay.intern.kim",
		"https://zd2df6qt6jmc.intern.kim/": "zd2df6qt6jmc-relay.intern.kim",
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
	if actual := DeriveRelayPublicURL("zd2df6qt6jmc.intern.kim"); actual != "wss://zd2df6qt6jmc-relay.intern.kim" {
		t.Errorf("DeriveRelayPublicURL host = %q", actual)
	}
	if actual := DeriveRelayPublicURL("wss://zd2df6qt6jmc.intern.kim"); actual != "wss://zd2df6qt6jmc-relay.intern.kim" {
		t.Errorf("DeriveRelayPublicURL url = %q", actual)
	}
	if actual := DeriveRelayPublicURL(""); actual != "" {
		t.Errorf("DeriveRelayPublicURL empty = %q, want empty", actual)
	}
}
