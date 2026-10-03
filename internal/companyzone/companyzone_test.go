package companyzone

import "testing"

func TestZoneDropsTheAPILabel(t *testing.T) {
	for apiBaseURL, expected := range map[string]string{
		"https://api.example.test":  "example.test",
		"https://api.example.test/": "example.test",
		"api.example.test":          "example.test",
		"https://example.test":      "example.test",
		"":                          Default(),
	} {
		if zone := Zone(apiBaseURL); zone != expected {
			t.Fatalf("Zone(%q) = %q, want %q", apiBaseURL, zone, expected)
		}
	}
}

func TestConfigurationWinsOverTheDefaultZone(t *testing.T) {
	if Default() == "" {
		t.Fatal("a build with no self-hosting settings still needs a zone to serve")
	}
	if Zone("https://api.selfhosted.example") != "selfhosted.example" {
		t.Fatal("a configured API address must win over the default")
	}
}

func TestCoversAcceptsTheZoneAndItsLabelsOnly(t *testing.T) {
	if !Covers("example.test", "company.example.test") {
		t.Fatal("a host under the zone must be covered")
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
