package admind

import (
	"os"
	"path/filepath"
	"testing"
)

func aServiceWithLinkFiles(t *testing.T, deviceURL string, taskPublicURL string) *Service {
	t.Helper()
	directory := t.TempDir()
	devicePath := filepath.Join(directory, "device-url")
	taskPath := filepath.Join(directory, "flow-public-url")
	if deviceURL != "" {
		if errorValue := os.WriteFile(devicePath, []byte(deviceURL+"\n"), 0o600); errorValue != nil {
			t.Fatalf("write the device url: %v", errorValue)
		}
	}
	if taskPublicURL != "" {
		if errorValue := os.WriteFile(taskPath, []byte(taskPublicURL+"\n"), 0o600); errorValue != nil {
			t.Fatalf("write the flow url: %v", errorValue)
		}
	}
	return &Service{Configuration: Configuration{
		DeviceURLPath:     devicePath,
		TaskPublicURLPath: taskPath,
	}}
}

func TestLinksFollowTheDeviceWhenNothingSaysOtherwise(t *testing.T) {
	service := aServiceWithLinkFiles(t, "https://device.example.test", "")
	if base := service.taskLinkBaseURL(); base != "https://device.example.test" {
		t.Fatalf("expected the device url, got %q", base)
	}
}

func TestAWrittenTaskURLMovesTheLinks(t *testing.T) {
	service := aServiceWithLinkFiles(t, "https://device.example.test", "https://company.example.test")
	if base := service.taskLinkBaseURL(); base != "https://company.example.test" {
		t.Fatalf("expected the written url, got %q", base)
	}
}

func TestTheLinksMoveBackWhenTheFileGoesAway(t *testing.T) {
	service := aServiceWithLinkFiles(t, "https://device.example.test", "https://company.example.test")
	if errorValue := os.Remove(service.Configuration.TaskPublicURLPath); errorValue != nil {
		t.Fatalf("remove the flow url file: %v", errorValue)
	}
	if base := service.taskLinkBaseURL(); base != "https://device.example.test" {
		t.Fatalf("expected the device url again, got %q", base)
	}
}

func TestTheFlagStillWinsOverTheFile(t *testing.T) {
	service := aServiceWithLinkFiles(t, "https://device.example.test", "https://company.example.test")
	service.Configuration.TaskPublicURL = "https://asked.example.test"
	if base := service.taskLinkBaseURL(); base != "https://asked.example.test" {
		t.Fatalf("expected the flag to win, got %q", base)
	}
}

func TestEveryLinkKindFollowsTheSameBase(t *testing.T) {
	service := aServiceWithLinkFiles(t, "https://device.example.test", "https://company.example.test")
	links := map[string]string{
		"flow":      service.mattermostTaskPageURL(""),
		"calendar":  service.mattermostCalendarURL(""),
		"messenger": service.messengerPublicURL(),
	}
	for name, link := range links {
		if len(link) < len("https://company.example.test") || link[:len("https://company.example.test")] != "https://company.example.test" {
			t.Errorf("the %s link did not move: %q", name, link)
		}
	}
}
