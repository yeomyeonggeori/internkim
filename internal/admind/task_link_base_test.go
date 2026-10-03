package admind

import (
	"os"
	"path/filepath"
	"testing"
)

func aServiceWithATaskURLFile(t *testing.T, taskPublicURL string) *Service {
	t.Helper()
	taskPath := filepath.Join(t.TempDir(), "flow-public-url")
	if errorValue := os.WriteFile(taskPath, []byte(taskPublicURL+"\n"), 0o600); errorValue != nil {
		t.Fatalf("write the flow url: %v", errorValue)
	}
	return &Service{Configuration: Configuration{TaskPublicURLPath: taskPath}}
}

func TestLinksPointAtTheCompanyAddress(t *testing.T) {
	service := aServiceWithATaskURLFile(t, "https://company.example.test")
	service.Configuration.CentralPlaneAppURL = "https://example.test"
	if base := service.taskLinkBaseURL(); base != "https://example.test" {
		t.Fatalf("expected the company address, got %q", base)
	}
}

func TestAWrittenTaskURLMovesTheLinks(t *testing.T) {
	service := aServiceWithATaskURLFile(t, "https://company.example.test")
	if base := service.taskLinkBaseURL(); base != "https://company.example.test" {
		t.Fatalf("expected the written url, got %q", base)
	}
}

func TestTheLinksPointNowhereWhenNothingNamesAnAddress(t *testing.T) {
	service := aServiceWithATaskURLFile(t, "https://company.example.test")
	if errorValue := os.Remove(service.Configuration.TaskPublicURLPath); errorValue != nil {
		t.Fatalf("remove the flow url file: %v", errorValue)
	}
	if base := service.taskLinkBaseURL(); base != "" {
		t.Fatalf("expected no address, got %q", base)
	}
}

func TestTheFlagStillWinsOverTheFile(t *testing.T) {
	service := aServiceWithATaskURLFile(t, "https://company.example.test")
	service.Configuration.TaskPublicURL = "https://asked.example.test"
	if base := service.taskLinkBaseURL(); base != "https://asked.example.test" {
		t.Fatalf("expected the flag to win, got %q", base)
	}
}
