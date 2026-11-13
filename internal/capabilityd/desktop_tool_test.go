package capabilityd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/desktop"
)

type postedDesktopWork struct {
	RequestID    string       `json:"requestID"`
	DesktopOwner string       `json:"desktopOwner"`
	Work         desktop.Call `json:"work"`
}

type standInRelay struct {
	posted []postedDesktopWork
	fields [][]string
}

func serviceWithARelay(t *testing.T, answer func(postedDesktopWork) (int, string)) (Service, *standInRelay) {
	t.Helper()
	relay := &standInRelay{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/desktop-work" || request.Header.Get("Authorization") != "Bearer host-service-token" {
			t.Errorf("the relay was asked %s with %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(request.Body)
		var posted postedDesktopWork
		var fields map[string]json.RawMessage
		if json.Unmarshal(body, &posted) != nil || json.Unmarshal(body, &fields) != nil {
			t.Errorf("the relay was sent %s", body)
		}
		relay.posted = append(relay.posted, posted)
		relay.fields = append(relay.fields, slices.Sorted(maps.Keys(fields)))
		status, document := answer(posted)
		writer.WriteHeader(status)
		writer.Write([]byte(document))
	}))
	t.Cleanup(server.Close)
	tokenPath := filepath.Join(t.TempDir(), "service-token")
	if errorValue := os.WriteFile(tokenPath, []byte("host-service-token\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return Service{Configuration: Configuration{RelayBaseURL: server.URL, ServiceTokenPath: tokenPath}}, relay
}

func answeredByTheGateway(status int, body string) func(postedDesktopWork) (int, string) {
	return func(posted postedDesktopWork) (int, string) {
		return http.StatusOK, `{"requestID":"` + posted.RequestID + `","status":` + strconv.Itoa(status) + `,"body":` + body + `}`
	}
}

func answeredByTheDesktop(answer desktop.Answer) func(postedDesktopWork) (int, string) {
	body, _ := json.Marshal(answer)
	return answeredByTheGateway(http.StatusOK, string(body))
}

func desktopCall(toolName string, requesterEmail string, input string) capabilities.ToolInvokeRequest {
	return capabilities.ToolInvokeRequest{
		ToolName: toolName,
		Input:    []byte(input),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: requesterEmail},
	}
}

const snapshotEnvelope = `{"version":"2.6","ok":true,"command":"snapshot","data":{"snapshot_id":"s8f3k2p9","tree":{"role":"window","name":"Finder","ref":"@s8f3k2p9:e1"}}}`

func TestDesktopWorkIsAddressedToTheRequesterAloneWhateverCaseTheirAddressArrivesIn(t *testing.T) {
	service, relay := serviceWithARelay(t, answeredByTheDesktop(desktop.Answer{Output: snapshotEnvelope}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, " Sample@Example.com ", `{"arguments":["snapshot","--app","Finder","-i"]}`))

	if errorValue != nil || response.IsError {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if len(relay.posted) != 1 {
		t.Fatalf("posted %d pieces of work", len(relay.posted))
	}
	posted := relay.posted[0]
	if posted.DesktopOwner != "sample@example.com" || posted.RequestID == "" || !slices.Equal(relay.fields[0], []string{"desktopOwner", "requestID", "work"}) {
		t.Fatalf("the work was addressed %+v with %v", posted, relay.fields[0])
	}
	if !slices.Equal(posted.Work.Arguments, []string{"snapshot", "--app", "Finder", "-i"}) || posted.Work.Text != "" {
		t.Fatalf("the desktop was asked %+v", posted.Work)
	}
	if response.Content != snapshotEnvelope {
		t.Fatalf("the model reads %q", response.Content)
	}
}

func TestADesktopCallWithNoRequesterIsRefusedBeforeAnythingIsPosted(t *testing.T) {
	service, relay := serviceWithARelay(t, answeredByTheDesktop(desktop.Answer{Output: snapshotEnvelope}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "  ", `{"arguments":["snapshot"]}`))

	if errorValue != nil || !response.IsError || response.Outcome != capabilities.ToolOutcomeFailed {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if len(relay.posted) != 0 {
		t.Fatalf("work with no requester was posted: %+v", relay.posted)
	}
}

func TestACommandOfTheOtherKindIsRefusedBeforeAnythingIsPostedAndNamesTheToolThatRunsIt(t *testing.T) {
	service, relay := serviceWithARelay(t, answeredByTheDesktop(desktop.Answer{Output: snapshotEnvelope}))

	for _, call := range []struct{ toolName, input, otherTool string }{
		{desktopReadToolName, `{"arguments":["click","@s8f3k2p9:e4"]}`, desktopActToolName},
		{desktopActToolName, `{"arguments":["snapshot","-i"]}`, desktopReadToolName},
	} {
		response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(call.toolName, "sample@example.com", call.input))
		if errorValue != nil || !response.IsError || !strings.Contains(response.Content, call.otherTool) {
			t.Errorf("%s %s was answered %+v, %v", call.toolName, call.input, response, errorValue)
		}
	}
	if len(relay.posted) != 0 {
		t.Fatalf("a command of the wrong kind was posted: %+v", relay.posted)
	}
}

func TestTextArrivesOnStandardInputBesideTheCommandItIsTypedWith(t *testing.T) {
	typed := `{"version":"2.6","ok":true,"command":"type","data":{"typed":true}}`
	service, relay := serviceWithARelay(t, answeredByTheDesktop(desktop.Answer{Output: typed}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "sample@example.com", `{"arguments":["type","@s8f3k2p9:e4"],"text":"분기 보고서 --session other"}`))

	if errorValue != nil || response.IsError {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	work := relay.posted[0].Work
	if !slices.Equal(work.Arguments, []string{"type", "@s8f3k2p9:e4", "--stdin"}) || work.Text != "분기 보고서 --session other" {
		t.Fatalf("the desktop was asked %+v", work)
	}
}

func TestAFailedEnvelopeComesBackAsAFailedCallCarryingAgentDesktopsOwnSuggestion(t *testing.T) {
	failed := `{"version":"2.6","ok":false,"command":"click","error":{"code":"SNAPSHOT_NOT_FOUND","message":"Snapshot 's0000' not found","suggestion":"Re-run a snapshot and retry with a ref from the returned snapshot"}}`
	service, _ := serviceWithARelay(t, answeredByTheDesktop(desktop.Answer{Output: failed + "\n"}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "sample@example.com", `{"arguments":["click","@s0000:e1"]}`))

	if errorValue != nil || !response.IsError || response.Outcome != capabilities.ToolOutcomeFailed {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if response.Content != failed {
		t.Fatalf("the model reads %q", response.Content)
	}
}

func TestARefusalFromTheDesktopComesBackAsAFailedCall(t *testing.T) {
	service, _ := serviceWithARelay(t, answeredByTheDesktop(desktop.Answer{Refusal: "the person turned desktop control off"}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["list-apps"]}`))

	if errorValue != nil || !response.IsError || !strings.Contains(response.Content, "the person turned desktop control off") {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
}

func TestADesktopNobodyHasOpenIsAnsweredUnreachable(t *testing.T) {
	service, _ := serviceWithARelay(t, answeredByTheGateway(http.StatusServiceUnavailable, `{"error":"this person has no desktop connected"}`))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["list-apps"]}`))

	if errorValue != nil || !response.IsError || response.Content != desktopUnreachableMessage {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
}

func TestARelayThatDoesNotAnswerIsNamedAsTheCauseAndNotTheRequestersComputer(t *testing.T) {
	service, _ := serviceWithARelay(t, answeredByTheDesktop(desktop.Answer{Output: snapshotEnvelope}))
	closed := httptest.NewServer(http.NotFoundHandler())
	service.Configuration.RelayBaseURL = closed.URL
	closed.Close()

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["list-apps"]}`))

	if errorValue != nil || !response.IsError || !strings.HasPrefix(response.Content, "desktop_unavailable_on_this_host: the relay at "+closed.URL+"/desktop-work did not answer") {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
}

func TestARelayWithNoGatewayIsNamedAsTheCauseWithoutTheGatewayBeingAsked(t *testing.T) {
	service, relay := serviceWithARelay(t, func(postedDesktopWork) (int, string) {
		return relayHasNoGatewayStatus, `{"error":"this relay is given no GATEWAY_URL"}`
	})

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["list-apps"]}`))

	if errorValue != nil || !response.IsError || !strings.Contains(response.Content, "desktop_unavailable_on_this_host: the relay at ") || !strings.Contains(response.Content, "has no gateway") {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if len(relay.posted) != 1 {
		t.Fatalf("the relay was asked %d times", len(relay.posted))
	}
}

func TestAnyOtherAnswerIsAnErrorNamingItsStatus(t *testing.T) {
	for named, answer := range map[string]func(postedDesktopWork) (int, string){
		"answered 409": func(postedDesktopWork) (int, string) {
			return http.StatusConflict, `{"error":"this computer stands by for the company and does not serve it"}`
		},
		"the gateway answered 504": answeredByTheGateway(http.StatusGatewayTimeout, `{"error":"call_timed_out"}`),
	} {
		service, _ := serviceWithARelay(t, answer)

		_, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["list-apps"]}`))

		if errorValue == nil || !strings.Contains(errorValue.Error(), named) {
			t.Errorf("answered %v, wanted an error naming %q", errorValue, named)
		}
	}
}

func TestAScreenshotComesBackAsAnAttachmentAndTheEnvelopeWithoutThePicture(t *testing.T) {
	picture := base64.StdEncoding.EncodeToString([]byte("a png the desktop took"))
	screenshot := `{"version":"2.6","ok":true,"command":"screenshot","data":{"data":"` + picture + `","format":"png","height":2,"width":3}}`
	service, _ := serviceWithARelay(t, answeredByTheDesktop(desktop.Answer{Output: screenshot}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["screenshot","--app","Finder"]}`))

	if errorValue != nil || response.IsError {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	descriptor, _ := capabilityToolDescriptorFor(desktopReadToolName)
	if errorValue := validateContractedCapabilityResponse(descriptor, response, "", ""); errorValue != nil {
		t.Fatalf("the answer breaks its contract: %v", errorValue)
	}
	var result desktopToolResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(result.Attachments) != 1 || result.Attachments[0].ContentBase64 != picture || result.Attachments[0].ContentType != "image/png" || result.Attachments[0].SizeBytes != int64(len("a png the desktop took")) {
		t.Fatalf("attached %+v", result.Attachments)
	}
	if strings.Contains(result.Output, picture) || strings.Contains(response.Content, picture) || !strings.Contains(result.Output, `"width":3`) {
		t.Fatalf("the envelope still carries the picture: output %q, content %q", result.Output, response.Content)
	}
}
