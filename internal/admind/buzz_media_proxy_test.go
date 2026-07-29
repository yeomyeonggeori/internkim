package admind

import "testing"

func TestRewriteBuzzMedia(t *testing.T) {
	service := &Service{}
	service.Configuration.BuzzRelayURL = "ws://127.0.0.1:3000"

	cases := map[string]string{
		"![photo](http://127.0.0.1:3000/media/abc.png)": "![photo](/buzz-media/media/abc.png)",
		"http://127.0.0.1:3000/media/avatar.png":         "/buzz-media/media/avatar.png",
		"":                                               "",
		"https://external.example/x.png":                 "https://external.example/x.png",
	}
	for input, expected := range cases {
		if actual := service.rewriteBuzzMedia(input); actual != expected {
			t.Errorf("rewriteBuzzMedia(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestBuzzMediaOriginScheme(t *testing.T) {
	service := &Service{}
	service.Configuration.BuzzRelayURL = "wss://relay.example"
	if origin := service.buzzMediaOrigin(); origin != "https://relay.example" {
		t.Errorf("wss origin = %q, want https://relay.example", origin)
	}
}
