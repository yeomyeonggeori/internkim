package admind

import (
	"context"
	"strings"
	"testing"
)

func TestGoogleCalendarProviderName(t *testing.T) {
	provider := googleCalendarProvider{}
	if provider.Name() != remoteCalendarProviderGoogle {
		t.Errorf("Name(): got %q", provider.Name())
	}
}

func TestGoogleCalendarProviderEndpointUsesLiteralEmail(t *testing.T) {
	provider := googleCalendarProvider{}
	endpoint := provider.Endpoint(remoteCalendarAccount{AccountEmail: "user@example.com"})
	expected := googleCalDAVBaseURL + "/user@example.com/user/"
	if endpoint != expected {
		t.Errorf("Endpoint: got %q, want %q", endpoint, expected)
	}
}

func TestGoogleCalendarProviderDiscoverPopulatesURLs(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedRemoteCalendarAccountForPull(t, service, "user@example.com")
	provider := googleCalendarProvider{}
	discovered, errorValue := provider.Discover(ctx, service, account)
	if errorValue != nil {
		t.Fatalf("Discover: %v", errorValue)
	}
	if !strings.HasSuffix(discovered.PrincipalURL, "/user") {
		t.Errorf("PrincipalURL: got %q", discovered.PrincipalURL)
	}
	if !strings.HasSuffix(discovered.HomeSetURL, "/") {
		t.Errorf("HomeSetURL: got %q", discovered.HomeSetURL)
	}
	if !strings.HasSuffix(discovered.DefaultCalendarURL, "/events/") {
		t.Errorf("DefaultCalendarURL: got %q", discovered.DefaultCalendarURL)
	}
	if !strings.Contains(discovered.DefaultCalendarURL, "user@example.com") {
		t.Errorf("DefaultCalendarURL missing email: %q", discovered.DefaultCalendarURL)
	}
}

func TestGoogleCalendarProviderDiscoverRequiresEmail(t *testing.T) {
	service := newCalendarTestService(t)
	provider := googleCalendarProvider{}
	account := remoteCalendarAccount{ID: "x", Provider: remoteCalendarProviderGoogle}
	if _, errorValue := provider.Discover(context.Background(), service, account); errorValue == nil {
		t.Fatal("expected error when email missing")
	}
}
