package fleetdomain

import "testing"

func TestZoneDropsTheAPILabel(t *testing.T) {
	for apiBaseURL, expected := range map[string]string{
		"https://api.example.test":  "example.test",
		"https://api.example.test/": "example.test",
		"api.example.test":          "example.test",
		"https://example.test":      "example.test",
		"":                          "",
	} {
		if zone := Zone(apiBaseURL); zone != expected {
			t.Fatalf("Zone(%q) = %q, want %q", apiBaseURL, zone, expected)
		}
	}
}

func TestHostAndSubdomainStayEmptyWithoutAZone(t *testing.T) {
	if host := Host("FLEET01", "example.test"); host != "fleet01.example.test" {
		t.Fatalf("host = %q", host)
	}
	if host := Host("fleet01", ""); host != "" {
		t.Fatalf("host without a zone = %q", host)
	}
	if address := Subdomain("updates", ""); address != "" {
		t.Fatalf("subdomain without a zone = %q", address)
	}
}

func TestCoversAcceptsTheZoneAndItsLabelsOnly(t *testing.T) {
	if !Covers("example.test", "device.example.test") {
		t.Fatal("a device under the zone must be covered")
	}
	if !Covers("example.test", "EXAMPLE.TEST") {
		t.Fatal("the zone itself must be covered whatever its case")
	}
	if Covers("example.test", "example.test.evil.example") {
		t.Fatal("a zone appearing as a prefix must not be covered")
	}
	if Covers("", "anything.example.test") {
		t.Fatal("an unconfigured zone must cover nothing")
	}
}
