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

const earlierAndLaterPrinciples = `{"history":[{"version":1,"document":{"schemaVersion":1,"values":["처음"]}},{"version":2,"document":{"schemaVersion":1,"values":["나중"]}}]}`
const principlesRevisionAnswer = `{"version":3,"document":{"schemaVersion":1},"reason":"","createdAt":"2026-11-01T00:00:00Z","origin":"administrator"}`

func principlesAnswering(history string) *standingIn {
	return answeringPerCall(func(request *http.Request) (int, string) {
		if request.Method == http.MethodGet {
			return http.StatusOK, history
		}
		return http.StatusOK, principlesRevisionAnswer
	})
}

func principlesRequest(toolName string, input string) capabilities.ToolInvokeRequest {
	return capabilities.ToolInvokeRequest{ToolName: toolName, Input: json.RawMessage(input), Context: capabilityprotocol.ToolInvokeContext{RequesterEmail: "admin@example.com"}}
}

func TestPrinciplesUpdateMergesOntoTheCurrentRevisionAndCarriesItsVersion(t *testing.T) {
	admind := principlesAnswering(`{"version":4,"document":{"schemaVersion":1,"values":["성실하게 일한다"],"tone":{"register":"formal","traits":["차분한"]}},"reason":"","createdAt":"2026-11-01T00:00:00Z","origin":"administrator"}`)
	service := serviceReaching(t, map[gateBackend]*standingIn{admindOverTheSocket: admind})
	response, errorValue := service.invokePersonaPrinciplesUpdateTool(t.Context(), principlesRequest("persona_principles_update", `{"tone":{"register":"polite"},"reason":"sample reason"}`))
	if errorValue != nil || response.IsError {
		t.Fatalf("the update failed: %+v, %v", response, errorValue)
	}
	var written soulRevisionRequestBody
	if errorValue := json.Unmarshal([]byte(admind.asked[1].Body), &written); errorValue != nil {
		t.Fatal(errorValue)
	}
	if admind.asked[1].Path != soulAdmindPath || written.ExpectedVersion != 4 || written.Reason != "sample reason" {
		t.Fatalf("the revision did not carry the version it was read at: %+v %s", admind.asked[1], admind.asked[1].Body)
	}
	if !strings.Contains(string(written.Document), `"traits":["차분한"]`) || !strings.Contains(string(written.Document), `"register":"polite"`) || strings.Contains(string(written.Document), "sample reason") {
		t.Fatalf("the document was not the current one with only the named fields changed: %s", written.Document)
	}
}

func TestPrinciplesRevertWritesTheEarlierDocumentAsANewRevision(t *testing.T) {
	admind := principlesAnswering(earlierAndLaterPrinciples)
	service := serviceReaching(t, map[gateBackend]*standingIn{admindOverTheSocket: admind})
	response, errorValue := service.invokePersonaPrinciplesRevertTool(t.Context(), principlesRequest("persona_principles_revert", `{"version":1,"reason":"sample"}`))
	if errorValue != nil || response.IsError {
		t.Fatalf("the revert failed: %+v, %v", response, errorValue)
	}
	var written soulRevisionRequestBody
	if errorValue := json.Unmarshal([]byte(admind.asked[1].Body), &written); errorValue != nil {
		t.Fatal(errorValue)
	}
	if written.ExpectedVersion != 2 || !strings.Contains(string(written.Document), "처음") {
		t.Fatalf("the revert did not write version 1 over version 2: %+v", written)
	}
}

func TestPrinciplesRevertRefusesAVersionThatDoesNotExist(t *testing.T) {
	admind := principlesAnswering(`{"history":[{"version":1,"document":{"schemaVersion":1}}]}`)
	service := serviceReaching(t, map[gateBackend]*standingIn{admindOverTheSocket: admind})
	response, errorValue := service.invokePersonaPrinciplesRevertTool(t.Context(), principlesRequest("persona_principles_revert", `{"version":9}`))
	if errorValue != nil || !response.IsError || !strings.Contains(response.Message, "no version 9") {
		t.Fatalf("an unknown version was not refused plainly: %+v, %v", response, errorValue)
	}
	if len(admind.asked) != 1 {
		t.Fatalf("a refused revert still wrote: %+v", admind.asked)
	}
}

func TestLearnedSkillUpdateCallsEachRouteItsFieldsNameInOrder(t *testing.T) {
	admind := answeringPerCall(func(request *http.Request) (int, string) {
		return http.StatusOK, `{"id":"sample-skill","action":"done","ok":true}`
	})
	service := serviceReaching(t, map[gateBackend]*standingIn{admindOverTheSocket: admind})
	response, errorValue := service.invokePersonaLearnedSkillUpdateTool(t.Context(), principlesRequest("persona_learned_skill_update", `{"id":"sample-skill","protected":true,"retired":false}`))
	if errorValue != nil || response.IsError {
		t.Fatalf("the update failed: %+v, %v", response, errorValue)
	}
	paths := []string{}
	for _, asked := range admind.asked {
		paths = append(paths, asked.Path)
	}
	if strings.Join(paths, " ") != learnedSkillActionAdmindPrefix+"protect "+learnedSkillActionAdmindPrefix+"restore" {
		t.Fatalf("the routes called were %v", paths)
	}
}

func TestLearnedSkillUpdateWithNothingToChangeIsRefused(t *testing.T) {
	service := serviceReaching(t, map[gateBackend]*standingIn{admindOverTheSocket: answering(`{}`)})
	response, errorValue := service.invokePersonaLearnedSkillUpdateTool(t.Context(), principlesRequest("persona_learned_skill_update", `{"id":"sample-skill"}`))
	if errorValue != nil || !response.IsError {
		t.Fatalf("an update that names nothing was not refused: %+v, %v", response, errorValue)
	}
}
