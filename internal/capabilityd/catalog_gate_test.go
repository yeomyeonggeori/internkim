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
	// the call as it arrives, for a tool that needs more of it than an input:
	// the conversation it was asked in, the files it was handed
	arrives func(capabilities.ToolInvokeRequest) capabilities.ToolInvokeRequest
	expect  func(*testing.T, capabilities.ToolInvokeResponse)
}

func gateCases() map[string]catalogGateCase {
	return map[string]catalogGateCase{
		"message_context": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{
				mattermostOverHTTP: answering(`{"id":"bot-1","username":"internkim","is_bot":true}`),
				blueclawOverHTTP:   answering(memberPolicyFor("person-1", "member@example.com")),
			},
			input:   `{}`,
			arrives:   inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"channelName":"전사-공지"`)
				expectResultHolds(t, answered, `"platform":"mattermost"`)
			},
		},
		"browser_open": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{browserAsACommand: runningTheBrowser()},
			input:   `{"url":"https://example.test/"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "example.test")
			},
		},
		"browser_snapshot": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{browserAsACommand: runningTheBrowser()},
			input:   `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "@e1")
			},
		},
		"artifact_review": {
			kind: provesBehaviour,
			reaches: map[gateBackend]*standingIn{
				workspaceOnDisk: holdingFiles(map[string]string{"shared/reports/shot.png": "a rendered page"}),
				openRouterOverHTTP: answeringPerCall(func(*http.Request) (int, string) {
				return http.StatusOK, `{"id":"gate","model":"gate-model","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"{\"passed\": true, \"issues\": [], \"acceptedWarnings\": [\"여백이 조금 넓습니다\"], \"summary\": \"의도대로 보입니다.\"}"}}]}`
			})},
			input: `{"artifactKind":"deck","intent":"3분기 실적을 한 장으로","rubric":"숫자가 읽히는가","evidence":[{"role":"rendered","path":"/workspace/shared/reports/shot.png","mimeType":"image/png","label":"1쪽"}]}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"passed": true`)
			},
		},
		"browser_click": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{browserAsACommand: runningTheBrowser()},
			input:   `{"ref":"@e1"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"target":"@e1"`)
			},
		},
		// The device browser cannot take a picture; the companion's can. Saying so
		// is the whole of this tool on a device, and a case that expected a
		// screenshot would be asserting a thing the product does not do.
		"browser_screenshot": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{browserAsACommand: runningTheBrowser()},
			input:   `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				if answered.Outcome == capabilities.ToolOutcomeSucceeded {
					t.Fatalf("the device browser answered a screenshot: %s", answered.Result)
				}
				expectResultHolds(t, answered, capabilities.CapabilityNotConnected)
			},
		},
		"channel_update": {
			kind: provesBehaviour,
			reaches: map[gateBackend]*standingIn{
				mattermostOverHTTP: answeringPerCall(func(request *http.Request) (int, string) {
					if strings.Contains(request.URL.Path, "/users/me") {
						return http.StatusOK, `{"id":"bot-1","username":"internkim","is_bot":true}`
					}
					return http.StatusOK, `{"id":"channel-1","display_name":"전사 공지","header":"9월 공지"}`
				}),
				blueclawOverHTTP: answering(adminPolicyFor("person-1", "member@example.com")),
			},
			input:     `{"channelID":"channel-1","header":"9월 공지"}`,
			arrives:   inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "channel-1")
			},
		},
		"site_serve": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{admindOverHTTP: answering(`{"siteID":"s1","slug":"q3-report","status":"published","publishedURL":"https://example.test/q3-report","previewURL":"https://example.test/preview/q3-report"}`)},
			input:   `{"title":"3분기 보고","sourceWorkspacePath":"shared/reports/q3","mode":"publish"}`,
			arrives: func(arriving capabilities.ToolInvokeRequest) capabilities.ToolInvokeRequest {
				arriving.Transport.SiteSourceBundle = &capabilities.SiteSourceBundle{
					WorkspacePath: "shared/reports/q3",
					Format:        "tar.gz",
					ContentBase64: "YSBzaXRlJ3Mgc291cmNlLCBhcyBhIHRhcmJhbGwgd291bGQgYmU=",
					SHA256:        "66a58643803176b2bd2b1e62efcbe1ddbee046c955051d5dd7999178e243274a",
				}
				return arriving
			},
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"slug":"q3-report"`)
			},
		},
		"site_unserve": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{admindOverHTTP: answeringPerCall(func(request *http.Request) (int, string) {
				// Taking a site down finds it first, so the list has to hold it.
				if request.Method == http.MethodDelete {
					return http.StatusOK, `{"siteID":"s1","slug":"q3-report","status":"deleted"}`
				}
				return http.StatusOK, `{"sites":[{"siteID":"s1","slug":"q3-report","status":"published","title":"3분기 보고"}]}`
			})},
			input: `{"siteReference":"q3-report"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"unserved":true`)
			},
		},
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
		"message_search": {
			kind: provesBehaviour,
			reaches: reachingTheMessenger(answeringPerCall(func(request *http.Request) (int, string) {
				if strings.Contains(request.URL.Path, "/users/me") {
					return http.StatusOK, `{"id":"bot-1","username":"internkim","is_bot":true}`
				}
				return http.StatusOK, `{"order":["post-1"],"posts":{"post-1":{"id":"post-1","message":"분기 보고 올립니다","channel_id":"channel-1","user_id":"person-1","create_at":1788000000000}}}`
			})),
			input:     `{"queries":["분기"]}`,
			arrives:   inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "post-1")
			},
		},
		"message_send": {
			kind: provesBehaviour,
			reaches: reachingTheMessenger(answeringPerCall(func(request *http.Request) (int, string) {
				if strings.Contains(request.URL.Path, "/users/me") {
					return http.StatusOK, `{"id":"bot-1","username":"internkim","is_bot":true}`
				}
				return http.StatusOK, `{"id":"post-2","channel_id":"channel-1","message":"덱 다 됐습니다","create_at":1788000001000}`
			})),
			input:     `{"targetType":"currentChannel","message":"덱 다 됐습니다"}`,
			arrives:   inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "post-2")
			},
		},
		"message_update": {
			kind: provesBehaviour,
			reaches: reachingTheMessenger(answeringPerCall(func(request *http.Request) (int, string) {
				switch {
				case strings.Contains(request.URL.Path, "/users/me"):
					return http.StatusOK, `{"id":"bot-1","username":"internkim","is_bot":true}`
				case request.Method == http.MethodPut:
					return http.StatusOK, `{"id":"post-1","channel_id":"channel-1","message":"9월 2일로 옮깁니다","user_id":"bot-1","create_at":1788000000000}`
				default:
					return http.StatusOK, `{"id":"post-1","channel_id":"channel-1","message":"9월 1일로 옮깁니다","user_id":"bot-1","create_at":1788000000000}`
				}
			})),
			input:     `{"messageID":"post-1","oldText":"9월 1일","newText":"9월 2일"}`,
			arrives:   inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "post-1")
			},
		},
		"message_delete": {
			kind: provesBehaviour,
			reaches: reachingTheMessenger(answeringPerCall(func(request *http.Request) (int, string) {
				switch {
				case strings.Contains(request.URL.Path, "/users/me"):
					return http.StatusOK, `{"id":"bot-1","username":"internkim","is_bot":true}`
				case request.Method == http.MethodDelete:
					return http.StatusOK, `{"status":"OK"}`
				default:
					return http.StatusOK, `{"id":"post-1","channel_id":"channel-1","message":"지울 것","user_id":"bot-1","create_at":1788000000000}`
				}
			})),
			input:     `{"messageIDs":["post-1"]}`,
			arrives:   inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "post-1")
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
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_list","result":{"count":1,"scope":"person","personID":"p1","personName":"이샘플","statusFilter":"approved","registeredKinds":["연차"],"leave":[{"leaveID":"l1","person":"이샘플","kind":"연차","days":2,"status":"approved","isPaid":true,"isDeducted":true,"startDate":"2026-09-01","endDate":"2026-09-02","note":null}]}}`)},
			input:  `{"status":"approved"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"count":1`)
			},
		},
		"document_read": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{converterAsACommand: answering(`{"content":"# 3분기 보고\n\n지출은 이렇게 되었습니다."}`)},
			input:   `{"path":"shared/reports/q3.md"}`,
			arrives: func(arriving capabilities.ToolInvokeRequest) capabilities.ToolInvokeRequest {
				arriving.Transport.WorkspaceFile = &capabilities.WorkspaceFile{
					WorkspacePath: "shared/reports/q3.md",
					Filename:      "q3.md",
					ContentBase64: "IyAz67aE6riwIOuztOqzoAoK7KeA7Lac7J2AIOydtOugh+qyjCDrkJjsl4jsirXri4jri6QuCg==",
					SHA256:        "d7c030e5d5e46e721000a0fbbd93944deadfae6572ae013387544dec30864c39",
				}
				return arriving
			},
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "3분기 보고")
			},
		},
		"image_read": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{},
			input:   `{"path":"shared/reports/q3.png"}`,
			arrives: func(arriving capabilities.ToolInvokeRequest) capabilities.ToolInvokeRequest {
				arriving.Transport.WorkspaceFile = &capabilities.WorkspaceFile{
					WorkspacePath: "shared/reports/q3.png",
					Filename:      "q3.png",
					ContentBase64: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAACklEQVR4nGNgAAACAAEA",
					SHA256:        "58ac0dfd8909f6d06eec8b91cc318c4761b315db23aff59f439c27d29afd2df5",
				}
				return arriving
			},
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "q3.png")
			},
		},
		"leave_balance": {
			kind:   provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_balance","result":{"personID":"p1","personName":"이샘플","year":2026,"grantedDays":15,"remainingDays":13,"usedDays":2,"tracking":"managed"}}`)},
			input:  `{"year":2026}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"remainingDays":13`)
			},
		},
		"leave_request": {
			kind:   provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_request","result":{"leaveID":"l2","person":"이샘플","kind":"연차","days":1,"status":"approved","isPaid":true,"isDeducted":true,"startDate":"2026-09-04","endDate":"2026-09-04","note":null}}`)},
			input:  `{"kind":"연차","startsAt":"2026-09-04","endsAt":"2026-09-04","days":1}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"approved"`)
			},
		},
		"leave_decide": {
			kind:   provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_decide","result":{"leaveID":"l2","person":"이샘플","kind":"연차","days":1,"status":"approved","isPaid":true,"isDeducted":true,"startDate":"2026-09-04","endDate":"2026-09-04","note":null}}`)},
			input:  `{"leaveHint":"이샘플 · 연차 · 2026-09-04","decision":"approved"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"approved"`)
			},
		},
		"attendance_list": {
			kind: provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_list","result":{"scope":"person","personID":"p1","personName":"이샘플","from":"2026-08-02","to":"2026-09-01","count":1,"attendance":[{"eventID":"a1","person":"이샘플","kind":"clock_in","date":"2026-09-01","time":"09:02","location":"본사","wasCorrected":false,"reason":null}]}}`)},
			input: `{"scope":"self"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"time":"09:02"`)
			},
		},
		"attendance_add": {
			kind: provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_add","result":{"status":"added","eventID":"a2","backdated":false}}`)},
			input: `{"kind":"clock_in","date":"2026-09-01","time":"09:02","reason":"출근 기록을 잊었습니다"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"added"`)
			},
		},
		"attendance_update": {
			kind: provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_update","result":{"status":"corrected","eventID":null,"backdated":false}}`)},
			input: `{"eventHint":"이샘플 · clock_in · 2026-09-01 09:02","time":"08:52","reason":"10분 일찍 왔습니다"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"corrected"`)
			},
		},
		"attendance_delete": {
			kind: provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_delete","result":{"status":"asked","eventID":null,"backdated":true}}`)},
			input: `{"eventHint":"이샘플 · clock_in · 2026-08-04 09:02","reason":"두 번 찍혔습니다"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"asked"`)
			},
		},
	}
}

// The tools this gate does not cover yet. It exists so that a tool added
// tomorrow cannot quietly join them: a catalog tool in neither this list nor
// gateCases fails the gate. It shrinks to empty as cases land — internkim#1144.
// The task, calendar and person tools are implemented twice: once in Go here,
// once on the plane. Step 4 of the SQLite retirement replaces the Go handler
// bodies with a call to /api/v1, at which point their cases are as thin as
// leave's. A case written now would encode the contract of code scheduled for
// deletion and make deleting it harder, so they wait for that step rather than
// for somebody's attention.
var toolsWithNoGateCaseYet = []string{
	"event_add",
	"event_delete",
	"event_list",
	"event_update",
	"person_list",
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
		"attendance_list": true, "attendance_add": true, "attendance_update": true, "attendance_delete": true,
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
			answered, errorValue := route.Handler(service, context.Background(), arrivingCall(name, gateCase))
			if errorValue != nil {
				t.Fatalf("%s: %v", name, errorValue)
			}
			expectAnswerKeepsItsContract(t, name, answered)
			gateCase.expect(t, answered)
		})
	}
}

// agent-browser prints the current URL for one command and a JSON snapshot for
// another, so the stand-in answers by what it was asked to do.
func runningTheBrowser() *standingIn {
	return answeringPerCall(func(request *http.Request) (int, string) {
		asked := request.URL.Path
		switch {
		case strings.Contains(asked, "snapshot"):
			return 0, `{"url":"https://example.test/","title":"예시","elements":[{"ref":"@e1","role":"button","name":"보내기"}]}`
		case strings.Contains(asked, "screenshot"):
			return 0, `{"path":"/tmp/shot.png"}`
		default:
			return 0, "https://example.test/"
		}
	})
}

// Mattermost answers the bot lookup and then whatever the tool asks of it, and
// blueclaw answers who the requester is. Every message tool needs both.
func reachingTheMessenger(mattermost *standingIn) map[gateBackend]*standingIn {
	return map[gateBackend]*standingIn{
		mattermostOverHTTP: mattermost,
		blueclawOverHTTP:   answering(memberPolicyFor("person-1", "member@example.com")),
	}
}

func inAChannel(arriving capabilities.ToolInvokeRequest) capabilities.ToolInvokeRequest {
	arriving.Context.Platform = "mattermost"
	arriving.Context.ConversationID = "channel-1"
	arriving.Context.ConversationType = "channel"
	arriving.Context.ChannelID = "channel-1"
	arriving.Context.ChannelName = "전사-공지"
	arriving.Context.RequesterPersonID = "person-1"
	return arriving
}

// The message tools ask blueclaw who the requester is before they answer, so a
// case for one of them says the requester works here.
func adminPolicyFor(personID string, email string) string {
	return `{"people":[{"personID":"` + personID + `","displayName":"이샘플","emails":["` + email + `"],"circles":["` + mattermostToolMemberCircle + `"],"isAdmin":true}]}`
}

func memberPolicyFor(personID string, email string) string {
	// The circle the message tools require is the code's to name, so the case
	// asks for it rather than spelling it — internkim#507 renames it.
	return `{"people":[{"personID":"` + personID + `","displayName":"이샘플","emails":["` + email + `"],"circles":["` + mattermostToolMemberCircle + `"]}]}`
}

// The task, calendar and person tools all read the same Flow state and write
// through the same paths, so one stand-in answers for all nine of them.
func recordAnsweringTasks() *standingIn {
	state := `{"currentWeek":{"label":"2026-W36"},"members":[{"personID":"person-1","name":"이샘플","email":"member@example.com"}],"tasks":[{"id":"task-1","title":"분기 보고서 초안","status":"planned","size":"M","ownerID":"person-1","isEvent":false},{"id":"event-1","title":"주간 회의","status":"planned","ownerID":"person-1","isEvent":true,"startsAt":"2026-09-01T01:00:00Z","endsAt":"2026-09-01T02:00:00Z"}],"definitions":{"categories":["영업"],"types":["문서"],"sizes":[{"name":"M"}]}}`
	written := `{"id":"task-1","title":"분기 보고서 초안","status":"planned","size":"M","ownerID":"person-1","isEvent":false}`
	writtenEvent := `{"id":"event-1","title":"주간 회의","status":"planned","ownerID":"person-1","isEvent":true,"startsAt":"2026-09-01T01:00:00Z","endsAt":"2026-09-01T02:00:00Z"}`
	return answeringPerCall(func(request *http.Request) (int, string) {
		switch {
		case strings.Contains(request.URL.Path, "/state") || strings.Contains(request.URL.Path, "/summary"):
			return http.StatusOK, state
		case strings.Contains(request.URL.Path, "event"):
			return http.StatusOK, writtenEvent
		default:
			return http.StatusOK, written
		}
	})
}

func reachingTheRecord() map[gateBackend]*standingIn {
	return map[gateBackend]*standingIn{admindOverTheSocket: recordAnsweringTasks()}
}

func arrivingCall(name string, gateCase catalogGateCase) capabilities.ToolInvokeRequest {
	arriving := capabilities.ToolInvokeRequest{
		ToolName: name,
		Input:    json.RawMessage(gateCase.input),
		Context:  capabilityprotocol.ToolInvokeContext{RequesterEmail: "member@example.com"},
	}
	if gateCase.arrives == nil {
		return arriving
	}
	return gateCase.arrives(arriving)
}

// A handler answers; the caller that reaches the agent runs that answer past
// the tool's own descriptor first, and rejects it whole when the declared
// effects, schema or identity do not hold. Calling the handler alone proves
// the call was carried, never that the answer is usable, so the gate holds
// every covered tool to the same check its real caller applies.
func expectAnswerKeepsItsContract(t *testing.T, toolName string, answered capabilities.ToolInvokeResponse) {
	t.Helper()
	descriptor, isRegistered := capabilityToolDescriptorFor(toolName)
	if !isRegistered {
		t.Fatalf("%s is in the catalog and has no descriptor", toolName)
	}
	if errorValue := validateContractedCapabilityResponse(descriptor, answered, "", ""); errorValue != nil {
		t.Fatalf("%s answered something its caller refuses: %v", toolName, errorValue)
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
