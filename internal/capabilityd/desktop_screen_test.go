package capabilityd

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/desktop"
)

const (
	windowSnapshotEnvelope = `{"version":"2.6","ok":true,"command":"snapshot","data":{"app":"TextEdit","complete":true,"ref_count":1,"snapshot_id":"s8f3k2p9","tree":{"role":"window","children":[{"ref_id":"@s8f3k2p9:e4","role":"button","name":"New Document"}]},"window":{"id":"w-14363","title":"Open"}}}`
	laterSnapshotEnvelope  = `{"version":"2.6","ok":true,"command":"snapshot","data":{"app":"TextEdit","complete":true,"ref_count":1,"snapshot_id":"s3ev5ejd","tree":{"role":"window","children":[{"ref_id":"@s3ev5ejd:e1","role":"textfield"}]},"window":{"id":"w-14401","title":"Untitled"}}}`
	clickedEnvelope        = `{"version":"2.6","ok":true,"command":"click","data":{"action":"click","disposition":{"delivery":"delivered_unverified","retry":"unsafe"},"steps":[{"label":"AXPress","mechanism":"semantic_api","outcome":"succeeded"}]}}`
	launchedEnvelope       = `{"version":"2.6","ok":true,"command":"launch","data":{"app":"TextEdit","pid":53908,"window":{"accessible":true,"app_name":"TextEdit","id":"w-14363","title":"Open","visible":true}}}`
)

func answeredPerCommand(answers map[string]desktop.Answer) func(postedDesktopWork) (int, string) {
	return func(posted postedDesktopWork) (int, string) {
		return answeredByTheDesktop(answers[posted.Work.Arguments[0]])(posted)
	}
}

func TestALaunchAnswersWithTheWindowItOpened(t *testing.T) {
	service, relay := serviceWithARelay(t, answeredPerCommand(map[string]desktop.Answer{
		"launch":   {Output: launchedEnvelope},
		"snapshot": {Output: windowSnapshotEnvelope},
	}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "sample@example.com", `{"arguments":["launch","TextEdit"]}`))

	if errorValue != nil || response.IsError {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	asked := []string{"snapshot", "--window-id", "w-14363", "-i", "--compact"}
	if len(relay.posted) != 2 || !slices.Equal(relay.posted[1].Work.Arguments, asked) || relay.posted[1].DesktopOwner != "sample@example.com" {
		t.Fatalf("the desktop was asked %+v", relay.posted)
	}
	want := launchedEnvelope + "\n\n" + `the desktop after this act, asked with ["snapshot","--window-id","w-14363","-i","--compact"]:` + "\n" + windowSnapshotEnvelope
	if response.Content != want {
		t.Fatalf("the model reads %q", response.Content)
	}
	if result := desktopResultIn(t, response.Result); result.Output != want {
		t.Fatalf("the result is %+v", result)
	}
	descriptor, _ := capabilityToolDescriptorFor(desktopActToolName)
	if errorValue := validateContractedCapabilityResponse(descriptor, response, "", ""); errorValue != nil {
		t.Fatalf("the answer breaks its contract: %v", errorValue)
	}
}

func TestAnActOnARefAnswersWithTheWindowThatRefWasReadFromAndItsNewRefsWorkTheSameWay(t *testing.T) {
	snapshots := []string{windowSnapshotEnvelope, laterSnapshotEnvelope, laterSnapshotEnvelope}
	service, relay := serviceWithARelay(t, func(posted postedDesktopWork) (int, string) {
		answer := desktop.Answer{Output: clickedEnvelope}
		if posted.Work.Arguments[0] == "snapshot" {
			answer, snapshots = desktop.Answer{Output: snapshots[0]}, snapshots[1:]
		}
		return answeredByTheDesktop(answer)(posted)
	})
	service.observedWindows = &observedWindows{}

	for _, input := range []string{
		`{"arguments":["snapshot","--app","TextEdit"]}`,
		`{"arguments":["click","@s8f3k2p9:e4"]}`,
		`{"arguments":["click","@s3ev5ejd:e1"]}`,
	} {
		if response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "sample@example.com", input)); errorValue != nil || response.IsError {
			t.Fatalf("%s was answered %+v, %v", input, response, errorValue)
		}
	}

	asked := [][]string{
		{"snapshot", "--app", "TextEdit", "-i", "--compact"},
		{"click", "@s8f3k2p9:e4"},
		{"snapshot", "--window-id", "w-14363", "-i", "--compact"},
		{"click", "@s3ev5ejd:e1"},
		{"snapshot", "--window-id", "w-14401", "-i", "--compact"},
	}
	if len(relay.posted) != len(asked) {
		t.Fatalf("the desktop was asked %+v", relay.posted)
	}
	for index, want := range asked {
		if !slices.Equal(relay.posted[index].Work.Arguments, want) {
			t.Errorf("call %d asked %v", index, relay.posted[index].Work.Arguments)
		}
	}
}

func TestAnActThatReplacedItsWindowAnswersWithTheWindowItsAppShowsNow(t *testing.T) {
	windowGone := `{"version":"2.6","ok":false,"command":"snapshot","error":{"code":"WINDOW_NOT_FOUND","message":"no matching window"}}`
	service, relay := serviceWithARelay(t, func(posted postedDesktopWork) (int, string) {
		answer := desktop.Answer{Output: launchedEnvelope}
		if posted.Work.Arguments[0] == "snapshot" {
			answer = desktop.Answer{Output: windowGone}
		}
		if slices.Contains(posted.Work.Arguments, "--app") {
			answer = desktop.Answer{Output: laterSnapshotEnvelope}
		}
		return answeredByTheDesktop(answer)(posted)
	})

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "sample@example.com", `{"arguments":["launch","TextEdit"]}`))

	want := launchedEnvelope + "\n\n" + `the desktop after this act, asked with ["snapshot","--app","TextEdit","-i","--compact"]:` + "\n" + laterSnapshotEnvelope
	if errorValue != nil || response.IsError || response.Content != want {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if len(relay.posted) != 3 || !slices.Equal(relay.posted[1].Work.Arguments, []string{"snapshot", "--window-id", "w-14363", "-i", "--compact"}) {
		t.Fatalf("the desktop was asked %+v", relay.posted)
	}
}

func TestAWindowOneRequesterReadIsNotTheScreenAfterAnothersAct(t *testing.T) {
	service, relay := serviceWithARelay(t, answeredPerCommand(map[string]desktop.Answer{
		"click":    {Output: clickedEnvelope},
		"snapshot": {Output: windowSnapshotEnvelope},
	}))
	service.observedWindows = &observedWindows{}

	service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["snapshot","--app","TextEdit"]}`))
	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "other@example.com", `{"arguments":["click","@s8f3k2p9:e4"]}`))

	if errorValue != nil || response.IsError || response.Content != clickedEnvelope || len(relay.posted) != 2 {
		t.Fatalf("answered %+v, %v after %d calls", response, errorValue, len(relay.posted))
	}
}

func TestNoScreenIsReadAfterAnActThatDeliveredNothingOrACommandThatOnlyReads(t *testing.T) {
	alreadyChecked := `{"version":"2.6","ok":true,"command":"check","data":{"action":"check","disposition":{"delivery":"not_delivered","retry":"safe"}}}`
	for input, answer := range map[string]string{
		`{"arguments":["check","@s8f3k2p9:e4","--app","TextEdit"]}`: alreadyChecked,
		`{"arguments":["list-windows","--app","TextEdit"]}`:         `{"version":"2.6","ok":true,"command":"list-windows","data":[{"id":"w-1","app_name":"TextEdit"}]}`,
		`{"arguments":["clipboard-set"],"text":"copied"}`:           `{"version":"2.6","ok":true,"command":"clipboard-set","data":{"written":true}}`,
	} {
		service, relay := serviceWithARelay(t, answeredByTheDesktop(desktop.Answer{Output: answer}))

		response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "sample@example.com", input))

		if errorValue != nil || response.IsError || response.Content != answer || len(relay.posted) != 1 {
			t.Errorf("%s was answered %+v, %v after %d calls", input, response, errorValue, len(relay.posted))
		}
	}
}

func TestAnActStandsWhenTheScreenAfterItCouldNotBeReadAndSaysWhy(t *testing.T) {
	asked := `the desktop after this act, asked with ["snapshot","--window-id","w-14363","-i","--compact"]:` + "\n"
	windowGone := `{"version":"2.6","ok":false,"command":"snapshot","error":{"code":"WINDOW_NOT_FOUND","message":"no matching window"}}`
	for name, example := range map[string]struct {
		answer func(postedDesktopWork) (int, string)
		want   string
	}{
		"the computer refused": {answeredPerCommand(map[string]desktop.Answer{"launch": {Output: launchedEnvelope}, "snapshot": {Refusal: "agent-desktop ran longer than 110 seconds and was stopped"}}), asked + "it was not read: agent-desktop ran longer than 110 seconds and was stopped"},
		"the program failed":   {answeredPerCommand(map[string]desktop.Answer{"launch": {Output: launchedEnvelope}, "snapshot": {Output: windowGone}}), `the desktop after this act, asked with ["snapshot","--app","TextEdit","-i","--compact"]:` + "\n" + windowGone},
		"no computer took it": {func(posted postedDesktopWork) (int, string) {
			if posted.Work.Arguments[0] == "snapshot" {
				return answeredByTheGateway(http.StatusServiceUnavailable, `{"error":"this person has no desktop connected"}`)(posted)
			}
			return answeredByTheDesktop(desktop.Answer{Output: launchedEnvelope})(posted)
		}, asked + "it was not read: " + errNoDesktopTookTheWork.Error()},
	} {
		service, _ := serviceWithARelay(t, example.answer)

		response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "sample@example.com", `{"arguments":["launch","TextEdit"]}`))

		if errorValue != nil || response.IsError || response.Content != launchedEnvelope+"\n\n"+example.want {
			t.Errorf("%s: answered %+v, %v", name, response, errorValue)
		}
	}
}

func TestAnActStandsWhenItsCallIsCancelledBeforeTheScreenIsRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	service, relay := serviceWithARelay(t, func(posted postedDesktopWork) (int, string) {
		if posted.Work.Arguments[0] == "snapshot" {
			cancel()
		}
		return answeredByTheDesktop(desktop.Answer{Output: launchedEnvelope})(posted)
	})

	response, errorValue := service.invokeDesktopTool(ctx, desktopCall(desktopActToolName, "sample@example.com", `{"arguments":["launch","TextEdit"]}`))

	if errorValue != nil || response.IsError || !strings.HasPrefix(response.Content, launchedEnvelope+"\n\n") || !strings.Contains(response.Content, "it was not read: ") || !strings.Contains(response.Content, context.Canceled.Error()) {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if len(relay.posted) != 2 {
		t.Fatalf("the relay was asked %d times", len(relay.posted))
	}
}

func TestABrowserActAnswersWithThePage(t *testing.T) {
	opened := `{"success":true,"data":{"title":"Example Domain","url":"https://example.com/"},"error":null}`
	page := `{"success":true,"data":{"refs":{"e1":{"name":"Learn more","role":"link"}},"snapshot":"- link \"Learn more\" [ref=e1]"},"error":null}`
	service, relay := serviceWithARelay(t, answeredPerCommand(map[string]desktop.Answer{"open": {Output: opened}, "snapshot": {Output: page}}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "sample@example.com", `{"program":"agent-browser","arguments":["open","https://example.com"]}`))

	if errorValue != nil || response.IsError || !strings.HasSuffix(response.Content, page) {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if len(relay.posted) != 2 || relay.posted[1].Work.Program != "agent-browser" || !slices.Equal(relay.posted[1].Work.Arguments, []string{"snapshot", "-i", "--json", "--headed"}) {
		t.Fatalf("the desktop was asked %+v", relay.posted)
	}
}

func TestTheScreenAfterAnActIsShallowerToo(t *testing.T) {
	lengthAtDepth := map[string]int{"": 40000, "3": 900}
	service, _ := serviceWithARelay(t, func(posted postedDesktopWork) (int, string) {
		if posted.Work.Arguments[0] == "launch" {
			return answeredByTheDesktop(desktop.Answer{Output: launchedEnvelope})(posted)
		}
		return answeredByTheDesktop(desktop.Answer{Output: snapshotOf(lengthAtDepth[depthAsked(posted)])})(posted)
	})

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "sample@example.com", `{"arguments":["launch","Slack"]}`))

	screen := launchedEnvelope + "\n\n" + `the desktop after this act, asked with ["snapshot","--window-id","w-14363","--skeleton","-i","--compact","--max-depth","3"]:` + "\n" + snapshotOf(900) + "\n\n"
	note, isNoted := strings.CutPrefix(response.Content, screen)
	if errorValue != nil || response.IsError || !isNoted || !strings.Contains(note, "stops 3 levels down") || desktop.IsTooLong(response.Content) {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
}
