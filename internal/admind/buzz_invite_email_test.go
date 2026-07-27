package admind

import (
	"strings"
	"testing"
)

func TestBuildBuzzInviteMessageContainsLinkAndGreeting(t *testing.T) {
	message := string(buildBuzzInviteMessage("Admin <admin@dawn.kim>", "newcomer@dawn.kim", "Newcomer", "https://intern.kim/messenger"))
	if !strings.Contains(message, "To: newcomer@dawn.kim") {
		t.Fatal("expected the invitee in the To header")
	}
	if !strings.Contains(message, "From: Admin <admin@dawn.kim>") {
		t.Fatal("expected the staff member as the sender")
	}
	if !strings.Contains(message, "Hi Newcomer") {
		t.Fatal("expected a personalized greeting")
	}
	if !strings.Contains(message, "https://intern.kim/messenger") {
		t.Fatal("expected the messenger link")
	}
}

func TestBuildBuzzInviteMessageFallsBackToGenericGreeting(t *testing.T) {
	message := string(buildBuzzInviteMessage("admin@dawn.kim", "newcomer@dawn.kim", "", "https://intern.kim/messenger"))
	if !strings.Contains(message, "Hello,") {
		t.Fatal("expected a generic greeting when no name is given")
	}
}

func TestMessengerPublicURLPrefersFlowPublicURL(t *testing.T) {
	service := &Service{Configuration: Configuration{FlowPublicURL: "https://intern.kim/"}}
	if got := service.messengerPublicURL(); got != "https://intern.kim/messenger" {
		t.Fatalf("expected flow public URL based link, got %q", got)
	}
}

func TestMessengerPublicURLEmptyWhenUnconfigured(t *testing.T) {
	service := &Service{Configuration: Configuration{}}
	if got := service.messengerPublicURL(); got != "" {
		t.Fatalf("expected empty link when nothing is configured, got %q", got)
	}
}
