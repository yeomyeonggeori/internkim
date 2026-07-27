package admind

import (
	"strings"
	"testing"
)

func TestBuildBuzzInviteMessageContainsLinkAndGreeting(t *testing.T) {
	message := string(buildBuzzInviteMessage("Admin <admin@example.com>", "newcomer@example.com", "Newcomer", "https://example.test/messenger"))
	if !strings.Contains(message, "To: newcomer@example.com") {
		t.Fatal("expected the invitee in the To header")
	}
	if !strings.Contains(message, "From: Admin <admin@example.com>") {
		t.Fatal("expected the staff member as the sender")
	}
	if !strings.Contains(message, "Hi Newcomer") {
		t.Fatal("expected a personalized greeting")
	}
	if !strings.Contains(message, "https://example.test/messenger") {
		t.Fatal("expected the messenger link")
	}
}

func TestBuildBuzzInviteMessageFallsBackToGenericGreeting(t *testing.T) {
	message := string(buildBuzzInviteMessage("admin@example.com", "newcomer@example.com", "", "https://example.test/messenger"))
	if !strings.Contains(message, "Hello,") {
		t.Fatal("expected a generic greeting when no name is given")
	}
}

func TestMessengerPublicURLPrefersFlowPublicURL(t *testing.T) {
	service := &Service{Configuration: Configuration{FlowPublicURL: "https://example.test/"}}
	if got := service.messengerPublicURL(); got != "https://example.test/messenger" {
		t.Fatalf("expected flow public URL based link, got %q", got)
	}
}

func TestMessengerPublicURLEmptyWhenUnconfigured(t *testing.T) {
	service := &Service{Configuration: Configuration{}}
	if got := service.messengerPublicURL(); got != "" {
		t.Fatalf("expected empty link when nothing is configured, got %q", got)
	}
}
