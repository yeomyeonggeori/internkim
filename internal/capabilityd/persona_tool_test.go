package capabilityd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

func echoingWritesOver(current string) *standingIn {
	standIn := &standingIn{}
	standIn.answers = func(request *http.Request) (int, string) {
		if request.Method == http.MethodGet {
			return http.StatusOK, current
		}
		standIn.mutex.Lock()
		defer standIn.mutex.Unlock()
		return http.StatusOK, standIn.asked[len(standIn.asked)-1].Body
	}
	return standIn
}

func TestPersonaUpdateWritesTheMergedDocumentToTheRouteTheScreenUses(t *testing.T) {
	admind := echoingWritesOver(`{"schemaVersion":1,"callMe":"샘플 님","morningBriefing":{"enabled":true,"time":"07:30"}}`)
	service := serviceReaching(t, map[gateBackend]*standingIn{admindOverTheSocket: admind})
	request := capabilities.ToolInvokeRequest{ToolName: "persona_profile_update", Input: json.RawMessage(`{"morningBriefing":{"enabled":false}}`), Context: capabilityprotocol.ToolInvokeContext{RequesterEmail: "member@example.com"}}
	response, errorValue := service.invokePersonaUpdateTool(t.Context(), request)
	if errorValue != nil || response.IsError {
		t.Fatalf("the update failed: %+v, %v", response, errorValue)
	}
	if len(admind.asked) != 2 || admind.asked[0].Method != http.MethodGet || admind.asked[1].Method != http.MethodPost || admind.asked[1].Path != personaProfileAdmindPath {
		t.Fatalf("the update did not read and then write the screen's route: %+v", admind.asked)
	}
	var written map[string]any
	if errorValue := json.Unmarshal([]byte(admind.asked[1].Body), &written); errorValue != nil {
		t.Fatal(errorValue)
	}
	briefing, _ := written["morningBriefing"].(map[string]any)
	if written["callMe"] != "샘플 님" || briefing["enabled"] != false || briefing["time"] != "07:30" {
		t.Fatalf("fields the call did not name were lost: %v", written)
	}
}

func TestPersonaUpdateKeepsTheAdministratorRefusal(t *testing.T) {
	admind := answeringPerCall(func(request *http.Request) (int, string) {
		if request.Method == http.MethodGet {
			return http.StatusOK, `{"schemaVersion":1,"names":["김인턴"]}`
		}
		return http.StatusForbidden, "only an administrator changes the agent identity"
	})
	service := serviceReaching(t, map[gateBackend]*standingIn{admindOverTheSocket: admind})
	request := capabilities.ToolInvokeRequest{ToolName: "persona_identity_update", Input: json.RawMessage(`{"names":["이샘플"]}`), Context: capabilityprotocol.ToolInvokeContext{RequesterEmail: "member@example.com"}}
	response, errorValue := service.invokePersonaUpdateTool(t.Context(), request)
	if errorValue != nil || !response.IsError || !strings.Contains(response.Content+string(response.Result), "only an administrator") {
		t.Fatalf("the administrator refusal was lost: %+v, %v", response, errorValue)
	}
}

func TestMergePatchFollowsRFC7396(t *testing.T) {
	var target, patch any
	_ = json.Unmarshal([]byte(`{"a":"b","c":{"d":"e","f":"g"},"names":["x","y"]}`), &target)
	_ = json.Unmarshal([]byte(`{"a":"z","c":{"f":null},"names":["w"]}`), &patch)
	merged, _ := json.Marshal(mergePatch(target, patch))
	if string(merged) != `{"a":"z","c":{"d":"e"},"names":["w"]}` {
		t.Fatalf("merged = %s", merged)
	}
}
