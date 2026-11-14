package capabilityd

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/desktop"
)

func desktopResultIn(t *testing.T, document []byte) desktopToolResult {
	t.Helper()
	var result desktopToolResult
	if errorValue := json.Unmarshal(document, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	return result
}

func snapshotOf(length int) string {
	return `{"version":"2.6","ok":true,"command":"snapshot","data":{"app":"Slack","snapshot_id":"sdense","tree":{"name":"` + strings.Repeat("가", length) + `"},"window":{"id":"w-5"}}}`
}

func depthAsked(posted postedDesktopWork) string {
	arguments := posted.Work.Arguments
	if index := slices.Index(arguments, "--max-depth"); index >= 0 {
		return arguments[index+1]
	}
	return ""
}

func TestASnapshotLongerThanOneAnswerCarriesComesBackShallowerWithWhatNarrowsIt(t *testing.T) {
	lengthAtDepth := map[string]int{"30": 40000, "3": 20000, "2": 900}
	service, relay := serviceWithARelay(t, func(posted postedDesktopWork) (int, string) {
		return answeredByTheDesktop(desktop.Answer{Output: snapshotOf(lengthAtDepth[depthAsked(posted)])})(posted)
	})

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["snapshot","--app","Slack","--max-depth","30"]}`))

	if errorValue != nil || response.IsError {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if len(relay.posted) != 3 || !slices.Equal(relay.posted[2].Work.Arguments, []string{"snapshot", "--app", "Slack", "--skeleton", "-i", "--compact", "--max-depth", "2"}) {
		t.Fatalf("the desktop was asked %+v", relay.posted)
	}
	result := desktopResultIn(t, response.Result)
	note, isNoted := strings.CutPrefix(response.Content, snapshotOf(900)+"\n\n")
	if !isNoted || result.Output != response.Content || desktop.IsTooLong(response.Content) {
		t.Fatalf("the model reads %d characters", len(response.Content))
	}
	for _, stated := range []string{"is 40", "one answer carries " + strconv.Itoa(desktop.LongestObservation), "stops 2 levels down", `["snapshot", "--root", ref_id]`, `["find", "--role", role`} {
		if !strings.Contains(note, stated) {
			t.Errorf("the note does not say %q: %s", stated, note)
		}
	}
	descriptor, _ := capabilityToolDescriptorFor(desktopReadToolName)
	if errorValue := validateContractedCapabilityResponse(descriptor, response, "", ""); errorValue != nil {
		t.Fatalf("the answer breaks its contract: %v", errorValue)
	}
}

func TestASnapshotTooLongAtEveryDepthFailsWithItsSizeAndWhatNarrowsIt(t *testing.T) {
	service, relay := serviceWithARelay(t, answeredByTheDesktop(desktop.Answer{Output: snapshotOf(40000)}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["snapshot","--app","Slack"]}`))

	if errorValue != nil || !response.IsError || len(relay.posted) != 1+len(desktop.ShallowerDepths) {
		t.Fatalf("answered %+v, %v after %d calls", response, errorValue, len(relay.posted))
	}
	if desktop.IsTooLong(response.Content) || !strings.HasPrefix(response.Content, "snapshot_too_long: ") || !strings.Contains(response.Content, `["find", "--role", role`) {
		t.Fatalf("the model reads %q", response.Content)
	}
}
