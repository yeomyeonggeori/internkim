package companyhost

import (
	"errors"
	"io"
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

type machineWhoseRosterNeverLands struct {
	recordedMachine
}

func (machine *machineWhoseRosterNeverLands) Output(name string, arguments []string) (string, error) {
	if name == "curl" && strings.HasSuffix(arguments[len(arguments)-1], blueclaw.AdmindRosterReadinessPath) {
		return "", errors.New("exit status 22: The requested URL returned error: 503")
	}
	return machine.recordedMachine.Output(name, arguments)
}

func TestTheServerIsNotReadyUntilTheAgentHoldsTheRoster(t *testing.T) {
	previousBudget := waitForTheServerBudget
	waitForTheServerBudget = 0
	t.Cleanup(func() { waitForTheServerBudget = previousBudget })
	machine := &machineWhoseRosterNeverLands{recordedMachine{
		answers: map[string]string{blueclaw.LinuxCompanyHostLayout().DataServicePath(): "PONG\n", "systemctl": "active"},
	}}

	errorValue := waitUntilTheServerAnswers(linuxPlatform{}, machine, io.Discard)

	if errorValue == nil {
		t.Fatal("install called the server ready while the agent did not know anybody, so a member's first message has nobody to run as")
	}
	for _, named := range []string{"has not handed the agent the company's roster", blueclaw.AdmindRosterReadinessPath, "journalctl -u internkim-admind.service"} {
		if !strings.Contains(errorValue.Error(), named) {
			t.Fatalf("the refusal does not name %q:\n%s", named, errorValue)
		}
	}
}
