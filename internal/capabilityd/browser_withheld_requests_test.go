package capabilityd

import (
	"context"
	"strings"
	"testing"
	"time"

	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func TestTheApprovalShowsEachWithheldRequestAsTheBrowserWouldSendIt(t *testing.T) {
	target := approvalTargetOf(withheldBrowserCall{ID: "withheld-1", Requests: []browserruntime.WithheldRequest{
		{Method: "POST", URL: "https://bank.example/transfer?from=main", Headers: map[string]string{"content-type": "application/x-www-form-urlencoded"}, Body: "to=%EB%B0%95%EC%98%88%EC%8B%9C&amount=1000&note=rent+due"},
		{Method: "DELETE", URL: "https://api.example/v1/items/7"},
	}})

	expected := strings.Join([]string{
		"POST https://bank.example/transfer?from=main",
		"to: 박예시",
		"amount: 1000",
		"note: rent due",
		"",
		"DELETE https://api.example/v1/items/7",
	}, "\n")
	if target.Preview != expected {
		t.Fatalf("preview:\n%s\nexpected:\n%s", target.Preview, expected)
	}
	if target.Title != "POST https://bank.example, DELETE https://api.example" || target.InputField != withheldRequestInputField || target.ID != "withheld-1" {
		t.Fatalf("target = %+v", target)
	}
}

func TestAnApprovalNeverHidesThatABodyWasCutShort(t *testing.T) {
	body := strings.Repeat("x", shownBodyLimit+12)
	preview := approvalTargetOf(withheldBrowserCall{ID: "withheld-1", Requests: []browserruntime.WithheldRequest{{Method: "PUT", URL: "https://files.example/upload", Body: body}}}).Preview

	if !strings.HasSuffix(preview, "(12 more bytes not shown)") {
		t.Fatalf("preview does not say what it left out: %q", preview[len(preview)-40:])
	}
}

func TestAWithheldRequestIsSentAtMostOnceAndOnlyForItsRequester(t *testing.T) {
	store := &withheldBrowserRequests{}
	store.keep(withheldBrowserCall{ID: "withheld-1", requester: "sample@example.com", withheldAt: time.Now()})

	if _, isTaken := store.take("withheld-1", "other@example.com"); isTaken {
		t.Fatal("another person took the requester's withheld request")
	}
	if _, isTaken := store.take("withheld-1", "sample@example.com"); !isTaken {
		t.Fatal("the requester could not take their withheld request")
	}
	if _, isTaken := store.take("withheld-1", "sample@example.com"); isTaken {
		t.Fatal("a withheld request was taken twice")
	}
}

func TestTheModelCannotSendAWithheldRequestByNamingIt(t *testing.T) {
	commandsRun := 0
	service := Service{
		Configuration:           Configuration{AgentBrowserPath: "agent-browser-test"},
		DeviceBrowsers:          deviceBrowsersWithNoDevtools(t),
		withheldBrowserRequests: &withheldBrowserRequests{},
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			commandsRun++
			return nil, nil
		},
	}
	service.withheldBrowserRequests.keep(withheldBrowserCall{ID: "withheld-1", requester: "sample@example.com", withheldAt: time.Now()})

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser_click", strings.NewReader(`{"input":{"ref":"@e1","withheldRequest":"withheld-1"},"context":{"requesterEmail":"sample@example.com"}}`))

	if errorValue != nil || response.Outcome == capabilities.ToolOutcomeSucceeded || commandsRun != 0 {
		t.Fatalf("response=%+v error=%v commands=%d", response, errorValue, commandsRun)
	}
	if _, isStillHeld := service.withheldBrowserRequests.take("withheld-1", "sample@example.com"); !isStillHeld {
		t.Fatal("refusing the call spent the withheld request")
	}
}

func TestABrowserThatCannotHoldRequestsDoesNotAct(t *testing.T) {
	commandsRun := 0
	service := Service{
		Configuration:           Configuration{AgentBrowserPath: "agent-browser-test"},
		DeviceBrowsers:          deviceBrowsersWithNoDevtools(t),
		withheldBrowserRequests: &withheldBrowserRequests{},
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			commandsRun++
			return nil, nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser_click", strings.NewReader(`{"input":{"ref":"@e1"},"context":{"requesterEmail":"sample@example.com"}}`))

	if errorValue != nil || response.Outcome != capabilities.ToolOutcomeFailed || commandsRun != 0 {
		t.Fatalf("response=%+v error=%v commands=%d", response, errorValue, commandsRun)
	}
	if !strings.Contains(response.Content, "could not hold this action's requests") {
		t.Fatalf("the refusal does not say why: %q", response.Content)
	}
}

func deviceBrowsersWithNoDevtools(t *testing.T) *browserruntime.DeviceBrowsers {
	return browserruntime.NewDeviceBrowsers(browserruntime.DeviceBrowserSettings{
		StateDirectory: t.TempDir(),
		FirstPort:      freeLocalPort(t),
		Capacity:       1,
		Launch: func(context.Context, browserruntime.DeviceBrowserLaunch) (browserruntime.RunningDeviceBrowser, error) {
			return &fakeRunningBrowser{exited: make(chan struct{})}, nil
		},
	})
}
