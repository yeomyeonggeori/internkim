package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/openroutertest"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

// The gate over the tools the catalog offers. A case says what a valid call
// looks like and what the answer has to be, so passing it means the tool works
// rather than that it answered.
type catalogGateCase struct {
	// what the record or the device answers when the tool is called
	answer string
	input  string
	expect func(*testing.T, capabilities.ToolInvokeResponse)
}

func gateCases() map[string]catalogGateCase {
	return map[string]catalogGateCase{
		"leave_list": {
			answer: `{"tool":"leave_list","result":{"count":1,"scope":"person","leave":[{"leaveID":"l1","kind":"연차","days":2,"status":"approved"}]}}`,
			input:  `{"status":"approved"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"count":1`)
			},
		},
		"leave_balance": {
			answer: `{"tool":"leave_balance","result":{"personName":"이샘플","year":2026,"grantedDays":15,"remainingDays":13,"usedDays":2,"tracking":"managed"}}`,
			input:  `{"year":2026}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"remainingDays":13`)
			},
		},
		"leave_request": {
			answer: `{"tool":"leave_request","result":{"leaveID":"l2","kind":"연차","days":1,"status":"requested"}}`,
			input:  `{"kind":"연차","startsAt":"2026-09-01","endsAt":"2026-09-01","days":1}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"requested"`)
			},
		},
		"leave_decide": {
			answer: `{"tool":"leave_decide","result":{"leaveID":"l2","status":"approved"}}`,
			input:  `{"leaveHint":"이샘플 · 연차 · 2026-09-01","decision":"approved"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"approved"`)
			},
		},
	}
}

// The tools this gate does not cover yet. It exists so that a tool added
// tomorrow cannot quietly join them: a catalog tool in neither this list nor
// gateCases fails the gate. It shrinks to empty as cases land — internkim#1144.
var toolsWithNoGateCaseYet = []string{
	"artifact_review",
	"browser_click",
	"browser_open",
	"browser_screenshot",
	"browser_snapshot",
	"channel_update",
	"document_read",
	"event_add",
	"event_delete",
	"event_list",
	"event_update",
	"image_read",
	"message_context",
	"message_delete",
	"message_search",
	"message_send",
	"message_update",
	"person_list",
	"site_list",
	"site_serve",
	"site_unserve",
	"task_add",
	"task_delete",
	"task_list",
	"task_update",
	"web_search",
}

func TestNoCatalogToolEscapesTheGateUnnoticed(t *testing.T) {
	covered := gateCases()
	uncovered := map[string]bool{}
	for _, name := range toolsWithNoGateCaseYet {
		uncovered[name] = true
	}

	var unaccounted []string
	for _, descriptor := range capabilityprotocol.GeneratedToolDescriptorSet() {
		_, hasCase := covered[descriptor.Name]
		if !hasCase && !uncovered[descriptor.Name] {
			unaccounted = append(unaccounted, descriptor.Name)
		}
	}
	sort.Strings(unaccounted)
	if len(unaccounted) > 0 {
		t.Fatalf("%v joined the catalog with no gate case; write one, or name it in toolsWithNoGateCaseYet and say why in internkim#1144", unaccounted)
	}
}

func TestTheUncoveredListHoldsNothingThatIsCovered(t *testing.T) {
	covered := gateCases()
	known := map[string]bool{}
	for _, descriptor := range capabilityprotocol.GeneratedToolDescriptorSet() {
		known[descriptor.Name] = true
	}
	for _, name := range toolsWithNoGateCaseYet {
		if _, hasCase := covered[name]; hasCase {
			t.Errorf("%s has a gate case and is still listed as uncovered", name)
		}
		if !known[name] {
			t.Errorf("%s is listed as uncovered but the catalog does not offer it", name)
		}
	}
}

func TestEveryGateCaseNamesAToolTheCatalogCarries(t *testing.T) {
	known := map[string]bool{}
	for _, descriptor := range capabilityprotocol.GeneratedToolDescriptorSet() {
		known[descriptor.Name] = true
	}
	for name := range gateCases() {
		if !known[name] {
			t.Errorf("%s has a gate case but the catalog does not offer it", name)
		}
	}
}

func TestTheCoveredCatalogToolsAnswerTheirCalls(t *testing.T) {
	for name, gateCase := range gateCases() {
		t.Run(name, func(t *testing.T) {
			socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.Header.Get(admindRequesterEmailHeader) == "" {
					http.Error(responseWriter, "this call named nobody", http.StatusForbidden)
					return
				}
				responseWriter.Header().Set("Content-Type", "application/json")
				_, _ = responseWriter.Write([]byte(gateCase.answer))
			}))
			fake := openroutertest.Start()
			defer fake.Close()

			service := Service{Configuration: Configuration{
				AdmindBaseURL:     admindOnLoopbackThatFailsTheTest(t),
				AdmindSocketPath:  socketPath,
				OpenRouterBaseURL: fake.BaseURL(),
			}}

			route, hasRoute := capabilityToolRouteFor(name)
			if !hasRoute {
				t.Fatalf("%s is in the catalog and nothing serves it", name)
			}
			answered, errorValue := route.Handler(service, context.Background(), capabilities.ToolInvokeRequest{
				ToolName: name,
				Input:    json.RawMessage(gateCase.input),
				Context:  capabilityprotocol.ToolInvokeContext{RequesterEmail: "staff@example.com"},
			})
			if errorValue != nil {
				t.Fatalf("%s: %v", name, errorValue)
			}
			gateCase.expect(t, answered)
		})
	}
}

func expectSucceeded(t *testing.T, answered capabilities.ToolInvokeResponse) {
	t.Helper()
	if answered.Outcome != capabilities.ToolOutcomeSucceeded {
		t.Fatalf("answered %s: %s", answered.Outcome, answered.Message)
	}
}

func expectResultHolds(t *testing.T, answered capabilities.ToolInvokeResponse, fragment string) {
	t.Helper()
	if !strings.Contains(string(answered.Result), fragment) {
		t.Fatalf("the result does not carry %s: %s", fragment, answered.Result)
	}
}
