package companyhost

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const startedCompanyID = "00000000-0000-4000-8000-0000000000c1"

func aMachineWhoseServicesAnswer() *recordedMachine {
	return &recordedMachine{answers: map[string]string{blueclaw.LinuxCompanyHostLayout().DataServicePath(): "PONG\n", "systemctl": "active"}}
}

func TestATakenOverBoxIsMarkedServingOnlyAfterItsServicesAnswered(t *testing.T) {
	machine := aMachineWhoseServicesAnswer()
	askedBeforeTheMark := 0

	errorValue := startTheServicesThenConfirm(linuxPlatform{}, machine, startedCompanyID, func(companyID string) error {
		for _, run := range machine.runs {
			if run[0] == "curl" {
				askedBeforeTheMark++
			}
		}
		return nil
	}, io.Discard)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if askedBeforeTheMark == 0 {
		t.Fatal("the box was marked as serving before any of its services was asked whether it answers")
	}
}

func TestAFailureWhileTheServicesStartLeavesTheBoxUnmarked(t *testing.T) {
	machine := aMachineWhoseServicesAnswer()
	machine.failures = map[string]error{"systemctl": errors.New("internkim-prepare.service failed")}
	marked := false

	errorValue := startTheServicesThenConfirm(linuxPlatform{}, machine, startedCompanyID, func(string) error {
		marked = true
		return nil
	}, io.Discard)

	if errorValue == nil || !strings.Contains(errorValue.Error(), "internkim-prepare.service failed") {
		t.Fatalf("a service that did not start was not reported: %v", errorValue)
	}
	if marked {
		t.Fatal("a box whose services did not start was marked as serving, so it never tries the take-over again")
	}
}
