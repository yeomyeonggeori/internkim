package capabilityd

import (
	"net/http"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

// The gate over the tools the catalog offers. A case says what a valid call
// looks like and what the answer has to be.
//
// What a case can prove depends on where the tool is implemented, and saying so
// is the difference between a gate and a number. A tool whose rows live in the
// record is implemented on the plane, so a case here fakes the record's answer
// and proves the carrying: the route, the requester, the input, the shape that
// comes back. What the record itself does is proved by the plane's own suite —
// a 403 like internkim#1136 would never have failed a carrying case. A tool the
// device implements has nowhere else to be proved, so its case is the whole of
// it.
type gateCaseKind string

const (
	provesCarrying  gateCaseKind = "carrying"
	provesBehaviour gateCaseKind = "behaviour"
)

type catalogGateCase struct {
	kind gateCaseKind
	// the addresses this call reaches, and what each of them answers
	reaches map[gateBackend]*standingIn
	input   string
	expect  func(*testing.T, capabilities.ToolInvokeResponse)
}

func gateCases() map[string]catalogGateCase {
	return map[string]catalogGateCase{
		"web_search": {
			kind: provesBehaviour,
			reaches: map[gateBackend]*standingIn{openRouterOverHTTP: answeringPerCall(func(*http.Request) (int, string) {
				return http.StatusOK, `{"id":"gate","model":"gate-model","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"{\"provider\": \"openrouter\", \"query\": \"새 세법 개정안\", \"answer\": \"세법 개정안은 이렇게 바뀌었습니다.\", \"results\": [{\"title\": \"세법 개정\", \"url\": \"https://example.test/tax\", \"snippet\": \"바뀐 것\"}]}"}}]}`
			})},
			input: `{"query":"새 세법 개정안"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"query":"새 세법 개정안"`)
				expectResultHolds(t, answered, "example.test/tax")
			},
		},
		"site_list": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{admindOverHTTP: answering(`{"sites":[{"siteID":"s1","slug":"q3-report","status":"published","title":"3분기 보고"}]}`)},
			input:   `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"q3-report"`)
			},
		},
		"leave_list": {
			kind:   provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_list","result":{"count":1,"scope":"person","leave":[{"leaveID":"l1","kind":"연차","days":2,"status":"approved"}]}}`)},
			input:  `{"status":"approved"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"count":1`)
			},
		},
		"leave_balance": {
			kind:   provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_balance","result":{"personName":"이샘플","year":2026,"grantedDays":15,"remainingDays":13,"usedDays":2,"tracking":"managed"}}`)},
			input:  `{"year":2026}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"remainingDays":13`)
			},
		},
		"leave_request": {
			kind:   provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_request","result":{"leaveID":"l2","kind":"연차","days":1,"status":"requested"}}`)},
			input:  `{"kind":"연차","startsAt":"2026-09-01","endsAt":"2026-09-01","days":1}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"requested"`)
			},
		},
		"leave_decide": {
			kind:   provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_decide","result":{"leaveID":"l2","status":"approved"}}`)},
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
	"site_serve",
	"site_unserve",
	"task_add",
	"task_delete",
	"task_list",
	"task_update",
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

// A case that proves only carrying must say so, and the tool it carries for
// must be one the plane implements. Otherwise the gate would count a faked
// answer as if the tool had been tested.
func TestACarryingCaseNamesAToolThePlaneImplements(t *testing.T) {
	runsOnThePlane := map[string]bool{
		"task_add": true, "task_update": true, "task_list": true, "task_delete": true,
		"event_add": true, "event_update": true, "event_list": true, "event_delete": true,
		"person_list":  true,
		"leave_list":   true, "leave_balance": true, "leave_request": true, "leave_decide": true,
	}
	for name, gateCase := range gateCases() {
		if gateCase.kind == "" {
			t.Errorf("%s does not say what its case proves", name)
			continue
		}
		if gateCase.kind == provesCarrying && !runsOnThePlane[name] {
			t.Errorf("%s is implemented on the device, so a carrying case proves nothing about it", name)
		}
		if gateCase.kind == provesBehaviour && runsOnThePlane[name] {
			t.Errorf("%s is implemented on the plane, so this case cannot prove its behaviour", name)
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
			service := serviceReaching(t, gateCase.reaches)

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
