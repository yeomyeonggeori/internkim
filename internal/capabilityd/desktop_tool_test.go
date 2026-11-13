package capabilityd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/desktop"
	"github.com/yeomyeonggeori/internkim/internal/workqueue"
)

type postedDesktopWork struct {
	RequestID    string       `json:"requestID"`
	AffinityKey  string       `json:"affinityKey"`
	DesktopOwner string       `json:"desktopOwner"`
	Work         desktop.Call `json:"work"`
}

type desktopGateway struct {
	posted []postedDesktopWork
}

func serviceWithACompanyQueue(t *testing.T, answer func(postedDesktopWork) string) (Service, *desktopGateway) {
	t.Helper()
	gateway := &desktopGateway{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/company/company-1/work" || request.Header.Get("Authorization") != "Bearer host.session.token" {
			t.Errorf("the queue was asked %s with %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(request.Body)
		var posted postedDesktopWork
		if errorValue := json.Unmarshal(body, &posted); errorValue != nil {
			t.Errorf("the queue was sent %s: %v", body, errorValue)
		}
		gateway.posted = append(gateway.posted, posted)
		writer.Write([]byte(answer(posted)))
	}))
	t.Cleanup(server.Close)
	directory := t.TempDir()
	queuePath := filepath.Join(directory, "work-queue.json")
	sessionPath := filepath.Join(directory, "agent-key")
	if errorValue := os.WriteFile(sessionPath, []byte("host.session.token\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := workqueue.Write(queuePath, workqueue.Address{GatewayURL: server.URL, CompanyID: "company-1"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	return Service{Configuration: Configuration{WorkQueuePath: queuePath, HostSessionPath: sessionPath}}, gateway
}

func answeredByTheDesktop(answer desktop.Answer) func(postedDesktopWork) string {
	return func(posted postedDesktopWork) string {
		body, _ := json.Marshal(answer)
		document, _ := json.Marshal(map[string]any{"requestID": posted.RequestID, "status": http.StatusOK, "body": json.RawMessage(body)})
		return string(document)
	}
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
	service, gateway := serviceWithACompanyQueue(t, answeredByTheDesktop(desktop.Answer{Output: snapshotEnvelope}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, " Sample@Example.com ", `{"arguments":["snapshot","--app","Finder","-i"]}`))

	if errorValue != nil || response.IsError {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if len(gateway.posted) != 1 {
		t.Fatalf("posted %d pieces of work", len(gateway.posted))
	}
	posted := gateway.posted[0]
	if posted.DesktopOwner != "sample@example.com" || posted.AffinityKey != "desktop:sample@example.com" || posted.RequestID == "" {
		t.Fatalf("the work was addressed %+v", posted)
	}
	if !slices.Equal(posted.Work.Arguments, []string{"snapshot", "--app", "Finder", "-i"}) || posted.Work.Text != "" {
		t.Fatalf("the desktop was asked %+v", posted.Work)
	}
	if response.Content != snapshotEnvelope {
		t.Fatalf("the model reads %q", response.Content)
	}
}

func TestADesktopCallWithNoRequesterIsRefusedBeforeAnythingIsPosted(t *testing.T) {
	service, gateway := serviceWithACompanyQueue(t, answeredByTheDesktop(desktop.Answer{Output: snapshotEnvelope}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "  ", `{"arguments":["snapshot"]}`))

	if errorValue != nil || !response.IsError || response.Outcome != capabilities.ToolOutcomeFailed {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if len(gateway.posted) != 0 {
		t.Fatalf("work with no requester was posted: %+v", gateway.posted)
	}
}

func TestACommandOfTheOtherKindIsRefusedBeforeAnythingIsPostedAndNamesTheToolThatRunsIt(t *testing.T) {
	service, gateway := serviceWithACompanyQueue(t, answeredByTheDesktop(desktop.Answer{Output: snapshotEnvelope}))

	for _, call := range []struct{ toolName, input, otherTool string }{
		{desktopReadToolName, `{"arguments":["click","@s8f3k2p9:e4"]}`, desktopActToolName},
		{desktopActToolName, `{"arguments":["snapshot","-i"]}`, desktopReadToolName},
	} {
		response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(call.toolName, "sample@example.com", call.input))
		if errorValue != nil || !response.IsError || !strings.Contains(response.Content, call.otherTool) {
			t.Errorf("%s %s was answered %+v, %v", call.toolName, call.input, response, errorValue)
		}
	}
	if len(gateway.posted) != 0 {
		t.Fatalf("a command of the wrong kind was posted: %+v", gateway.posted)
	}
}

func TestTextArrivesOnStandardInputBesideTheCommandItIsTypedWith(t *testing.T) {
	typed := `{"version":"2.6","ok":true,"command":"type","data":{"typed":true}}`
	service, gateway := serviceWithACompanyQueue(t, answeredByTheDesktop(desktop.Answer{Output: typed}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "sample@example.com", `{"arguments":["type","@s8f3k2p9:e4"],"text":"분기 보고서 --session other"}`))

	if errorValue != nil || response.IsError {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	work := gateway.posted[0].Work
	if !slices.Equal(work.Arguments, []string{"type", "@s8f3k2p9:e4", "--stdin"}) || work.Text != "분기 보고서 --session other" {
		t.Fatalf("the desktop was asked %+v", work)
	}
}

func TestAFailedEnvelopeComesBackAsAFailedCallCarryingAgentDesktopsOwnSuggestion(t *testing.T) {
	failed := `{"version":"2.6","ok":false,"command":"click","error":{"code":"SNAPSHOT_NOT_FOUND","message":"Snapshot 's0000' not found","suggestion":"Re-run a snapshot and retry with a ref from the returned snapshot"}}`
	service, _ := serviceWithACompanyQueue(t, answeredByTheDesktop(desktop.Answer{Output: failed + "\n"}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopActToolName, "sample@example.com", `{"arguments":["click","@s0000:e1"]}`))

	if errorValue != nil || !response.IsError || response.Outcome != capabilities.ToolOutcomeFailed {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if response.Content != failed {
		t.Fatalf("the model reads %q", response.Content)
	}
}

func TestARefusalFromTheDesktopComesBackAsAFailedCall(t *testing.T) {
	service, _ := serviceWithACompanyQueue(t, answeredByTheDesktop(desktop.Answer{Refusal: "the person turned desktop control off"}))

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["list-apps"]}`))

	if errorValue != nil || !response.IsError || !strings.Contains(response.Content, "the person turned desktop control off") {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
}

func TestADesktopNobodyHasOpenIsAnsweredUnreachable(t *testing.T) {
	service, _ := serviceWithACompanyQueue(t, func(posted postedDesktopWork) string {
		return `{"requestID":"` + posted.RequestID + `","status":503,"body":{"error":"this person has no desktop connected"}}`
	})

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["list-apps"]}`))

	if errorValue != nil || !response.IsError || !strings.HasPrefix(response.Content, "desktop_unreachable:") {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
}

func TestAHostThatServesNoCompanyQueueAnswersUnreachableWithoutAskingTheGateway(t *testing.T) {
	service, gateway := serviceWithACompanyQueue(t, answeredByTheDesktop(desktop.Answer{Output: snapshotEnvelope}))
	if errorValue := workqueue.Forget(service.Configuration.WorkQueuePath); errorValue != nil {
		t.Fatal(errorValue)
	}

	response, errorValue := service.invokeDesktopTool(context.Background(), desktopCall(desktopReadToolName, "sample@example.com", `{"arguments":["list-apps"]}`))

	if errorValue != nil || !response.IsError || !strings.HasPrefix(response.Content, "desktop_unreachable:") {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if len(gateway.posted) != 0 {
		t.Fatalf("the gateway was asked %+v", gateway.posted)
	}
}

func TestAScreenshotComesBackAsAnAttachmentAndTheEnvelopeWithoutThePicture(t *testing.T) {
	picture := base64.StdEncoding.EncodeToString([]byte("a png the desktop took"))
	screenshot := `{"version":"2.6","ok":true,"command":"screenshot","data":{"data":"` + picture + `","format":"png","height":2,"width":3}}`
	service, _ := serviceWithACompanyQueue(t, answeredByTheDesktop(desktop.Answer{Output: screenshot}))

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
