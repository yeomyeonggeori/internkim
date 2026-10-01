package companyhost

import (
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestInstallWaitsForTheMessengerBridgeBeforeCallingTheServerReady(t *testing.T) {
	for _, probe := range companyHostProbes(blueclaw.CompanyHostLayout{}) {
		if probe.SupervisedName != blueclaw.ChatdServiceName {
			continue
		}
		if !strings.HasSuffix(strings.Join(probe.Command, " "), blueclaw.CompanyHostChatdEndpoint+blueclaw.ChatdHealthPath) {
			t.Fatalf("the messenger bridge probe asks %v, not chatd's readiness route", probe.Command)
		}
		return
	}
	t.Fatal("install calls the server ready without waiting for chatd, so a person's first message can arrive while nothing hands it to the agent")
}
