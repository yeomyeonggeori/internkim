package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
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
		"dataroom_member_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"dataroom_member_update","result":{"saved":true}}`)},
			input:   `{"memberID":"62000000-0000-4000-8000-000000000001","roleCodes":["finance"]}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"saved":true`)
			},
		},
		"dataroom_links_get": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"dataroom_links_get","result":{"links":[],"shareableRoleCodes":["finance"],"downloadableRoleCodes":[]}}`)},
			input:   `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"shareableRoleCodes":["finance"]`)
			},
		},
		"dataroom_link_add": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"dataroom_link_add","result":{"linkID":"62000000-0000-4000-8000-000000000001","accessCode":"123456"}}`)},
			input:   `{"roleCode":"finance","label":"Sample financial review"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"accessCode":"123456"`)
			},
		},
		"dataroom_link_delete": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"dataroom_link_delete","result":{"saved":true}}`)},
			input:   `{"linkID":"62000000-0000-4000-8000-000000000001"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"saved":true`)
			},
		},
		"dataroom_get": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"dataroom_get","result":{"categories":[],"roles":[],"shares":[],"canManage":false}}`)},
			input:   `{}`,
			expect:  func(t *testing.T, answered capabilities.ToolInvokeResponse) { expectSucceeded(t, answered) },
		},
		"dataroom_category_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"dataroom_category_update","result":{"saved":true}}`)},
			input:   `{"code":"FZ","parent":"F","slug":"custom","name":"Custom","nameKO":"추가","description":"Sample finance records."}`,
			expect:  func(t *testing.T, answered capabilities.ToolInvokeResponse) { expectSucceeded(t, answered) },
		},
		"dataroom_role_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"dataroom_role_update","result":{"saved":true}}`)},
			input:   `{"code":"custom","name":"Custom","nameKO":"","readableCategories":["FS"]}`,
			expect:  func(t *testing.T, answered capabilities.ToolInvokeResponse) { expectSucceeded(t, answered) },
		},
		"dataroom_share_add": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"dataroom_share_add","result":{"shareID":"62000000-0000-4000-8000-000000000001"}}`)},
			input:   `{"roleCode":"investor","audience":"email","email":"sample@example.com"}`,
			expect:  func(t *testing.T, answered capabilities.ToolInvokeResponse) { expectSucceeded(t, answered) },
		},
		"dataroom_share_delete": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"dataroom_share_delete","result":{"saved":true}}`)},
			input:   `{"shareID":"62000000-0000-4000-8000-000000000001"}`,
			expect:  func(t *testing.T, answered capabilities.ToolInvokeResponse) { expectSucceeded(t, answered) },
		},
		"company_document_classify": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"categoryCode":"FS"}`)},
			input:   `{"title":"Statement","text":"Annual financial statements."}`,
			expect:  func(t *testing.T, answered capabilities.ToolInvokeResponse) { expectSucceeded(t, answered) },
		},
		"schedule_list": {
			kind: provesBehaviour,
			reaches: map[gateBackend]*standingIn{
				admindOverTheSocket: answering(`{"schedules":[{"scheduleID":"schedule-1","taskInstruction":"prepare the daily report","cadence":"cron","status":"failed"}]}`),
			},
			input: `{"status":"failed","limit":1}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"scheduleID":"schedule-1"`)
				expectResultHolds(t, answered, `"status":"failed"`)
			},
		},
		"schedule_create": {
			kind: provesBehaviour,
			reaches: map[gateBackend]*standingIn{
				admindOverTheSocket: answering(aWrittenSchedule),
			},
			input:   `{"taskInstruction":"주간 보고서를 정리해 올린다","kind":"cron","cronExpression":"0 9 * * 1","repeatPolicy":"unbounded"}`,
			arrives: inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"scheduleID":"schedule-1"`)
			},
		},
		"schedule_update": {
			kind: provesBehaviour,
			reaches: map[gateBackend]*standingIn{
				admindOverTheSocket: answering(aWrittenSchedule),
			},
			input: `{"scheduleHint":"주간 보고","intervalSecond":3600}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"nextRunAt":"2026-09-21T00:00:00Z"`)
			},
		},
		"schedule_cancel": {
			kind: provesBehaviour,
			reaches: map[gateBackend]*standingIn{
				admindOverTheSocket: answering(`{"scheduleIDs":["schedule-1"],"cancelled":[{"scheduleID":"schedule-1","description":"주간 보고"}]}`),
			},
			input: `{"scheduleHints":["주간 보고"]}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"scheduleID":"schedule-1"`)
			},
		},
		"message_context": {
			kind:    provesBehaviour,
			reaches: reachingTheMessenger(answering(`{"pubkeyHex":"bot-1","name":"internkim"}`)),
			input:   `{}`,
			arrives: inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"channelName":"전사-공지"`)
				expectResultHolds(t, answered, `"platform":"buzz"`)
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
		"image_generate": {
			kind: provesBehaviour,
			reaches: map[gateBackend]*standingIn{
				workspaceOnDisk:    holdingFiles(map[string]string{}),
				openRouterOverHTTP: answering(`{"choices":[{"message":{"content":"","images":[{"image_url":{"url":"data:image/png;base64,iVBORw0KGgo="}}]}}]}`),
			},
			input: `{"prompt":"파란 배경 위의 흰 로고","path":"/workspace/shared/logo.png"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"devicePath":"/workspace/shared/logo.png"`)
				expectResultHolds(t, answered, `"contentBase64":"iVBORw0KGgo="`)
			},
		},
		"browser_fill": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{browserAsACommand: runningTheBrowser()},
			input:   `{"ref":"@e1","text":"분기 보고서"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"action":"fill"`)
				expectResultHolds(t, answered, `"target":"@e1"`)
			},
		},
		"browser_screenshot": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{browserAsACommand: runningTheBrowser()},
			input:   `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"action":"screenshot"`)
				expectResultHolds(t, answered, `"contentType":"image/png"`)
			},
		},
		"browser_select": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{browserAsACommand: runningTheBrowser()},
			input:   `{"selector":"#city","value":"서울"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"action":"select"`)
				expectResultHolds(t, answered, `"target":"#city"`)
			},
		},
		"browser_press": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{browserAsACommand: runningTheBrowser()},
			input:   `{"key":"Enter"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"action":"press"`)
			},
		},
		"browser_wait": {
			kind:    provesBehaviour,
			reaches: map[gateBackend]*standingIn{browserAsACommand: runningTheBrowser()},
			input:   `{"milliseconds":500}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"action":"wait"`)
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
			reaches: reachingTheMessenger(answering(
				`{"channelID":"channel-1","candidates":[{"messageID":"post-1","channelID":"channel-1","text":"분기 보고 올립니다","authorPubkeyHex":"person-1","createdAt":1788000000000,"deletable":true}]}`)),
			input:   `{"queries":["분기"]}`,
			arrives: inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "post-1")
			},
		},
		"message_send": {
			kind:    provesBehaviour,
			reaches: reachingTheMessenger(answering(`{"messageID":"post-2","channelID":"channel-1"}`)),
			input:   `{"targetType":"currentChannel","message":"덱 다 됐습니다"}`,
			arrives: inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "post-2")
			},
		},
		"message_update": {
			kind: provesBehaviour,
			reaches: reachingTheMessenger(answeringPerCall(func(request *http.Request) (int, string) {
				// An edit reads the message's current text back before it
				// applies the quoted span, so the search answers first.
				if strings.HasSuffix(request.URL.Path, "/message.search") {
					return http.StatusOK, `{"channelID":"channel-1","candidates":[{"messageID":"post-1","channelID":"channel-1","text":"9월 1일로 옮깁니다","editable":true}]}`
				}
				return http.StatusOK, `{"messageID":"post-1"}`
			})),
			input:   `{"messageID":"post-1","oldText":"9월 1일","newText":"9월 2일"}`,
			arrives: inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "post-1")
			},
		},
		"message_delete": {
			kind:    provesBehaviour,
			reaches: reachingTheMessenger(answering(`{"deleted":true}`)),
			input:   `{"messageIDs":["post-1"]}`,
			arrives: inAChannel,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, "post-1")
			},
		},
		"leave_list": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_list","result":{"count":1,"scope":"person","personID":"p1","personName":"이샘플","statusFilter":"approved","registeredKinds":["연차"],"leave":[{"leaveID":"l1","personID":"p1","person":"이샘플","kindID":"annual","kind":"연차","days":2,"status":"approved","isPaid":true,"isDeducted":true,"startDate":"2026-09-01","endDate":"2026-09-02","startsAt":"2026-08-31T15:00:00Z","endsAt":"2026-09-02T15:00:00Z","note":null}]}}`)},
			input:   `{"status":"approved"}`,
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
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_balance","result":{"scope":"person","year":2026,"count":1,"balances":[{"personID":"p1","personName":"이샘플","grantedDays":15,"remainingDays":13,"usedDays":2,"tracking":"managed"}]}}`)},
			input:   `{"year":2026}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"remainingDays":13`)
			},
		},
		"leave_grant_set": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_grant_set","result":{"personID":"p1","personName":"이샘플","grantedDays":18,"remainingDays":16,"usedDays":2,"tracking":"managed"}}`)},
			input:   `{"personHint":"이샘플","days":18}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"grantedDays":18`)
			},
		},
		"leave_return_early": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_return_early","result":{"shortened":true,"leaveID":"l1","endsAt":"2026-09-01T04:00:00Z","days":0.5}}`)},
			input:   `{"location":"사무실"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"shortened":true`)
			},
		},
		"leave_request": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_request","result":{"leaveID":"l2","personID":"p1","person":"이샘플","kindID":"annual","kind":"연차","days":1,"status":"approved","isPaid":true,"isDeducted":true,"startDate":"2026-09-04","endDate":"2026-09-04","startsAt":"2026-09-03T15:00:00Z","endsAt":"2026-09-04T15:00:00Z","note":null}}`)},
			input:   `{"kind":"연차","startsAt":"2026-09-04","endsAt":"2026-09-04","days":1}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"approved"`)
			},
		},
		"leave_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_update","result":{"leaveID":"l2","personID":"p1","person":"이샘플","kindID":"annual","kind":"연차","days":1,"status":"requested","isPaid":true,"isDeducted":true,"startDate":"2026-09-04","endDate":"2026-09-04","startsAt":"2026-09-03T15:00:00Z","endsAt":"2026-09-04T15:00:00Z","note":null}}`)},
			input:   `{"leaveHint":"l2","startsAt":"2026-09-04","endsAt":"2026-09-04"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"startDate":"2026-09-04"`)
			},
		},
		"leave_delete": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_delete","result":{"leaveID":"l2","personID":"p1","person":"이샘플","kindID":"annual","kind":"연차","days":1,"status":"requested","isPaid":true,"isDeducted":true,"startDate":"2026-09-04","endDate":"2026-09-04","startsAt":"2026-09-03T15:00:00Z","endsAt":"2026-09-04T15:00:00Z","note":null}}`)},
			input:   `{"leaveHint":"l2"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"leaveID":"l2"`)
			},
		},
		"leave_decide": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"leave_decide","result":{"leaveID":"l2","personID":"p1","person":"이샘플","kindID":"annual","kind":"연차","days":1,"status":"approved","isPaid":true,"isDeducted":true,"startDate":"2026-09-04","endDate":"2026-09-04","startsAt":"2026-09-03T15:00:00Z","endsAt":"2026-09-04T15:00:00Z","note":null}}`)},
			input:   `{"leaveHint":"이샘플 · 연차 · 2026-09-04","decision":"approved"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"approved"`)
			},
		},
		"attendance_list": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_list","result":{"scope":"person","personID":"p1","personName":"이샘플","from":"2026-08-02","to":"2026-09-01","serverTime":"2026-09-01T00:02:00Z","backdatedAfterMinutes":4320,"count":1,"attendance":[{"eventID":"a1","personID":"p1","person":"이샘플","kind":"clock_in","date":"2026-09-01","time":"09:02","occurredAt":"2026-09-01T00:02:00Z","location":"본사","wasCorrected":false,"originalDate":null,"originalTime":null,"originalOccurredAt":null,"reason":null}]}}`)},
			input:   `{"scope":"self","handWrittenOnly":true}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"time":"09:02"`)
			},
		},
		"attendance_add": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_add","result":{"status":"added","eventID":"a2","backdated":false,"event":{"id":"a2","personID":"p1","kind":"clock_in","occurredAt":"2026-09-01T00:02:00Z","location":"본사"}}}`)},
			input:   `{"kind":"clock_in","location":"본사"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"added"`)
				expectResultHolds(t, answered, `"event":{"id":"a2","personID":"p1","kind":"clock_in","occurredAt":"2026-09-01T00:02:00Z","location":"본사"}`)
			},
		},
		"attendance_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_update","result":{"status":"corrected","eventID":"event-1","backdated":false}}`)},
			input:   `{"eventHint":"이샘플 · clock_in · 2026-09-01 09:02","time":"08:52","reason":"10분 일찍 왔습니다"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"corrected"`)
			},
		},
		"company_settings_get": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_settings_get","result":{"name":"\uc5ec\uba85\uac70\ub9ac","country":"KR","locale":"ko","timeZone":"Asia/Seoul","currencyCode":"KRW","workLocations":[{"name":"\uc0ac\ubb34\uc2e4","color":null}],"leaveDays":15,"teamViewVisibleToAll":true,"profileImageURL":null}}`)},
			input:   `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"timeZone":"Asia/Seoul"`)
			},
		},
		"company_settings_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_settings_update","result":{"name":"\uc5ec\uba85\uac70\ub9ac","country":"KR","locale":"ko","timeZone":"Asia/Tokyo","currencyCode":"KRW","workLocations":[],"leaveDays":15,"teamViewVisibleToAll":true,"profileImageURL":null}}`)},
			input:   `{"timeZone":"Asia/Tokyo"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"timeZone":"Asia/Tokyo"`)
			},
		},
		"company_info_get": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_info_get","result":{"language":"ko","name":"\uc8fc\uc2dd\ud68c\uc0ac \uc608\uc2dc","brandName":"","slogan":"","description":"","representative":"\uc774\uc0d8\ud50c","representativeTitle":"\ub300\ud45c\uc774\uc0ac","address":"\uc11c\uc6b8","officeAddress":"","jurisdiction":"","bankAccount":"","legalAttributes":[],"foundedDate":"","capital":"","fiscalYearEnd":"","employeeCount":0,"phone":"","fax":"","email":"","website":"","missingFields":["bankAccount","phone","email"],"updatedAt":""}}`)},
			input:   `{"language":"ko"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"missingFields"`)
			},
		},
		"company_info_set": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_info_set","result":{"language":"ko","name":"\uc8fc\uc2dd\ud68c\uc0ac \uc608\uc2dc","brandName":"","slogan":"","description":"","representative":"\uc774\uc0d8\ud50c","representativeTitle":"\ub300\ud45c\uc774\uc0ac","address":"\uc11c\uc6b8","officeAddress":"","jurisdiction":"","bankAccount":"","legalAttributes":[{"label":"\uc0ac\uc5c5\uc790\ub4f1\ub85d\ubc88\ud638","value":"123-45-67890"}],"foundedDate":"","capital":"","fiscalYearEnd":"","employeeCount":0,"phone":"","fax":"","email":"","website":"","missingFields":[],"updatedAt":"2026-09-03T00:00:00Z"}}`)},
			input:   `{"language":"ko","representative":"\uc774\uc0d8\ud50c","legalAttributes":"{\"\uc0ac\uc5c5\uc790\ub4f1\ub85d\ubc88\ud638\": \"123-45-67890\"}"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `123-45-67890`)
			},
		},
		"company_holiday_list": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_holiday_list","result":{"count":1,"year":2026,"holidays":[{"holidayID":"company-holiday-1","name":"\uac1c\ucc9c\uc808","date":"2026-10-03","recursAnnually":true,"createdAt":null,"updatedAt":null}]}}`)},
			input:   `{"year":2026}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"date":"2026-10-03"`)
			},
		},
		"company_holiday_add": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_holiday_add","result":{"holidayID":"company-holiday-2","name":"\ucc3d\ub9bd\uae30\ub150\uc77c","date":"2026-11-02","recursAnnually":false,"createdAt":"2026-09-03T00:00:00Z","updatedAt":"2026-09-03T00:00:00Z"}}`)},
			input:   `{"date":"2026-11-02","name":"\ucc3d\ub9bd\uae30\ub150\uc77c"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"holidayID":"company-holiday-2"`)
			},
		},
		"company_holiday_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_holiday_update","result":{"holidayID":"company-holiday-2","name":"\ucc3d\ub9bd\uae30\ub150\uc77c","date":"2026-11-03","recursAnnually":true,"createdAt":"2026-09-03T00:00:00Z","updatedAt":"2026-09-03T01:00:00Z"}}`)},
			input:   `{"holidayHint":"\ucc3d\ub9bd\uae30\ub150\uc77c","date":"2026-11-03","recursAnnually":true}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"recursAnnually":true`)
			},
		},
		"company_holiday_delete": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_holiday_delete","result":{"holidayID":"company-holiday-2","name":"\ucc3d\ub9bd\uae30\ub150\uc77c","date":"2026-11-03","recursAnnually":true,"createdAt":null,"updatedAt":null}}`)},
			input:   `{"holidayHint":"2026-11-03"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"holidayID":"company-holiday-2"`)
			},
		},
		"notification_settings_get": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"notification_settings_get","result":{"categories":[{"category":"message","isOn":true,"isChoosable":true},{"category":"leave","isOn":true,"isChoosable":false},{"category":"mail","isOn":false,"isChoosable":true}],"mutedConversationIDs":["conversation-1"]}}`)},
			input:   `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"category":"leave","isOn":true,"isChoosable":false`)
			},
		},
		"notification_settings_set": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"notification_settings_set","result":{"categories":[{"category":"message","isOn":true,"isChoosable":true},{"category":"leave","isOn":true,"isChoosable":false},{"category":"mail","isOn":true,"isChoosable":true}],"mutedConversationIDs":["conversation-1"]}}`)},
			input:   `{"turnOn":["mail"]}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"category":"mail","isOn":true`)
			},
		},
		"conversation_mute": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"conversation_mute","result":{"conversationID":"conversation-1","isMuted":true,"mutedConversationIDs":["conversation-1"]}}`)},
			input:   `{"conversationID":"conversation-1"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"isMuted":true`)
			},
		},
		"conversation_unmute": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"conversation_unmute","result":{"conversationID":"conversation-1","isMuted":false,"mutedConversationIDs":[]}}`)},
			input:   `{"conversationID":"conversation-1"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"isMuted":false`)
			},
		},
		"attendance_work_policy_get": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_work_policy_get","result":{"timeZone":"Asia/Seoul","workMode":"flexible","policy":{"version":1,"revisions":[{"effectiveDate":"1970-01-01","workMode":"flexible","workingWeekdays":[1,2,3,4,5],"dailyTargetMinutes":480,"weeklyTargetMinutes":2400,"referenceStartTime":"09:00","fixedStartTime":"","fixedEndTime":"","coreTimeEnabled":false,"coreStartTime":"","coreEndTime":"","breakPeriods":[],"nightStartTime":"22:00","nightEndTime":"06:00"}]},"people":[]}}`)},
			input:   `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"effectiveDate":"1970-01-01"`)
			},
		},
		"attendance_work_policy_set": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_work_policy_set","result":{"timeZone":"Asia/Seoul","effectiveDate":"2026-09-03","policy":{"version":1,"revisions":[{"effectiveDate":"2026-09-03","workMode":"fixed","workingWeekdays":[1,2,3,4],"dailyTargetMinutes":480,"weeklyTargetMinutes":1920,"referenceStartTime":"09:00","fixedStartTime":"09:00","fixedEndTime":"18:00","coreTimeEnabled":false,"coreStartTime":"","coreEndTime":"","breakPeriods":[{"startTime":"12:00","endTime":"13:00"}],"nightStartTime":"22:00","nightEndTime":"06:00"}]}}}`)},
			input:   `{"workMode":"fixed","workingWeekdays":[1,2,3,4],"dailyTargetMinutes":480,"weeklyTargetMinutes":1920,"referenceStartTime":"09:00","fixedStartTime":"09:00","fixedEndTime":"18:00","coreTimeEnabled":false,"coreStartTime":"","coreEndTime":"","breakPeriods":[{"startTime":"12:00","endTime":"13:00"}],"nightStartTime":"22:00","nightEndTime":"06:00"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"effectiveDate":"2026-09-03"`)
			},
		},
		"attendance_leave_policy_get": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_leave_policy_get","result":{"version":2,"balanceTrackingMode":"managed","fiscalYearStartMonth":1,"fiscalYearStartDay":1,"leaveTypes":[{"id":"annual","systemKind":"annual","name":"\uc5f0\ucc28","paid":true,"balanceMode":"annual","grantCadence":"annual","grantAmountMilliDays":15000,"expiryMode":"fiscalYearEnd","carryoverEnabled":false,"allowedUnits":["fullDay","halfDay","quarterDay"],"includeInSummary":true,"isActive":true,"isSystem":true,"sortOrder":0}],"updatedAt":""}}`)},
			input:   `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"balanceTrackingMode":"managed"`)
			},
		},
		"attendance_leave_policy_set": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_leave_policy_set","result":{"version":2,"balanceTrackingMode":"unlimited","fiscalYearStartMonth":3,"fiscalYearStartDay":1,"leaveTypes":[],"updatedAt":"2026-09-03T00:00:00Z"}}`)},
			input:   `{"balanceTrackingMode":"unlimited","fiscalYearStartMonth":3,"fiscalYearStartDay":1,"leaveTypes":[]}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"balanceTrackingMode":"unlimited"`)
			},
		},
		"company_metric_record": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_metric_record","result":{"metricID":"metric-1","metric":"annualRevenue","year":2025,"quarter":0,"month":0,"value":1200000000,"currency":"KRW","valueUSD":870000,"unit":null,"note":null,"updatedAt":"2026-09-04T00:00:00Z"}}`)},
			input:   `{"metric":"annualRevenue","year":2025,"value":1200000000,"currency":"KRW","valueUSD":870000}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"valueUSD":870000`)
			},
		},
		"company_metric_list": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_metric_list","result":{"count":1,"metrics":[{"metricID":"metric-1","metric":"annualRevenue","year":2025,"quarter":0,"month":0,"value":1200000000,"currency":"KRW","valueUSD":870000,"unit":null,"note":null,"updatedAt":"2026-09-04T00:00:00Z"}]}}`)},
			input:   `{"metric":"annualRevenue","fromYear":2024}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"metric":"annualRevenue"`)
			},
		},
		"company_record_add": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_record_add","result":{"recordID":"record-1","category":"funding","date":"2025-12-01","title":"Seed round closed","detail":null,"attributes":[{"label":"round","value":"Seed"}],"updatedAt":"2026-09-04T00:00:00Z"}}`)},
			input:   `{"category":"funding","date":"2025-12-01","title":"Seed round closed","attributes":"{\"round\": \"Seed\"}"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"recordID":"record-1"`)
			},
		},
		"company_record_list": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_record_list","result":{"count":1,"records":[{"recordID":"record-1","category":"funding","date":"2025-12-01","title":"Seed round closed","detail":null,"attributes":[],"updatedAt":"2026-09-04T00:00:00Z"}]}}`)},
			input:   `{"category":"funding"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"category":"funding"`)
			},
		},
		"company_record_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_record_update","result":{"recordID":"record-1","category":"funding","date":"2025-12-01","title":"Pre-seed round closed","detail":null,"attributes":[],"updatedAt":"2026-09-04T01:00:00Z"}}`)},
			input:   `{"recordHint":"Seed round closed","title":"Pre-seed round closed"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"title":"Pre-seed round closed"`)
			},
		},
		"company_record_delete": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_record_delete","result":{"recordID":"record-1","category":"funding","date":"2025-12-01","title":"Pre-seed round closed","detail":null,"attributes":[],"updatedAt":"2026-09-04T01:00:00Z"}}`)},
			input:   `{"recordHint":"record-1"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"recordID":"record-1"`)
			},
		},
		"company_document_register": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_document_register","result":{"documentID":"document-1","documentNumber":"Q-2026-001","kind":"issued","documentType":"quote","title":"ABC Trading onboarding quote","counterpart":"ABC Trading","language":"ko","filePath":null,"summary":"A quote for the onboarding consulting.","requesterID":"member-1","issuedAt":"2026-09-04T00:00:00Z","date":null,"period":null,"status":null,"supersedes":null,"sha256":null,"tags":[],"storagePath":null,"published":null,"storageDirectory":"/workspace/circles/member/documents/quote"}}`)},
			input:   `{"documentType":"quote","title":"ABC Trading onboarding quote","counterpart":"ABC Trading","language":"ko","summary":"A quote for the onboarding consulting."}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"documentNumber":"Q-2026-001"`)
			},
		},
		"company_document_list": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_document_list","result":{"count":1,"documents":[{"documentID":"document-1","documentNumber":"Q-2026-001","kind":"issued","documentType":"quote","title":"ABC Trading onboarding quote","counterpart":"ABC Trading","language":"ko","filePath":null,"summary":"A quote for the onboarding consulting.","requesterID":"member-1","issuedAt":"2026-09-04T00:00:00Z","date":null,"period":null,"status":null,"supersedes":null,"sha256":null,"tags":[],"storagePath":null,"published":null}]}}`)},
			input:   `{"type":"quote"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"documentType":"quote"`)
			},
		},
		"company_document_search": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_document_search","result":{"count":1,"documents":[{"documentID":"document-1","documentNumber":"Q-2026-001","kind":"issued","documentType":"quote","title":"ABC Trading onboarding quote","counterpart":"ABC Trading","language":"ko","filePath":null,"summary":"A quote for the onboarding consulting.","requesterID":"member-1","issuedAt":"2026-09-04T00:00:00Z","date":null,"period":null,"status":null,"supersedes":null,"sha256":null,"tags":[],"storagePath":null,"published":null}]}}`)},
			input:   `{"query":"what did we quote ABC Trading","limit":3}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"documentID":"document-1"`)
			},
		},
		"company_document_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_document_update","result":{"documentID":"document-1","documentNumber":"Q-2026-001","kind":"issued","documentType":"quote","title":"ABC Trading onboarding quote","counterpart":"ABC Trading","language":"ko","filePath":"shared/documents/quote/abc.md","summary":"A quote for the onboarding consulting.","requesterID":"member-1","issuedAt":"2026-09-04T00:00:00Z","date":null,"period":null,"status":null,"supersedes":null,"sha256":null,"tags":[],"storagePath":null,"published":null}}`)},
			input:   `{"documentHint":"Q-2026-001","filePath":"shared/documents/quote/abc.md"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"filePath":"shared/documents/quote/abc.md"`)
			},
		},
		"company_document_upload": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_document_upload","result":{"storagePath":"company-1/dataroom/Q/QQ/abc-quote.document-1.pdf","uploadURL":"https://example.test/storage/v1/object/upload/sign/asset/company-1/dataroom/Q/QQ/abc-quote.document-1.pdf?token=signed"}}`)},
			input:   `{"documentHint":"Q-2026-001","originalFileName":"abc quote.pdf"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"storagePath":"company-1/dataroom/Q/QQ/`)
			},
		},
		"company_document_download": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"company_document_download","result":{"storagePath":"company-1/dataroom/Q/QQ/abc-quote.document-1.pdf","downloadURL":"https://example.test/storage/v1/object/sign/asset/company-1/dataroom/Q/QQ/abc-quote.document-1.pdf?token=signed"}}`)},
			input:   `{"documentHint":"Q-2026-001"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"downloadURL":"https://example.test/`)
			},
		},
		"attendance_delete": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"attendance_delete","result":{"status":"asked","eventID":null,"backdated":true}}`)},
			input:   `{"eventHint":"이샘플 · clock_in · 2026-08-04 09:02","reason":"두 번 찍혔습니다"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"asked"`)
			},
		},
		"crm_activity_list": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_activity_list","result":{"count":1,"activities":[{"activityID":"t1","organizationID":"o1","opportunityID":"p1","contactID":"c1","business":"\uc601\uc5c5","kind":"meeting","title":"\ud0a5\uc624\ud504 \ubbf8\ud305","occurredAt":"2026-09-02T00:00:00Z","content":"\uc694\uad6c\uc0ac\ud56d\uc744 \ub4e4\uc5c8\ub2e4","taskStatus":"completed","ownerPersonID":"m1","requesterPersonID":"m1","isEvent":false,"isWholeDay":false,"startsAt":"","endsAt":"","notifyMinutesBefore":null,"location":"","createdAt":"2026-09-02T00:00:00Z","updatedAt":"2026-09-03T00:00:00Z"}],"registeredLabels":{"businesses":[{"name":"\uc601\uc5c5","color":"#2563eb"}],"types":[{"name":"meeting"}],"sizes":["XS","S","M","L","XL","XXL"],"statuses":["planned","in_progress","completed","requested","paused","rejected","stopped"]}}}`)},
			input:   `{"opportunityHint":"ABC\uc0c1\uc0ac \ub3c4\uc785"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"color":"#2563eb"`)
			},
		},
		"crm_activity_save": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_activity_save","result":{"activityID":"t1","organizationID":"o1","opportunityID":"p1","contactID":"c1","business":"\uc601\uc5c5","kind":"meeting","title":"\ud0a5\uc624\ud504 \ubbf8\ud305","occurredAt":"2026-09-02T00:00:00Z","content":"\uc694\uad6c\uc0ac\ud56d\uc744 \ub4e4\uc5c8\ub2e4","taskStatus":"completed","ownerPersonID":"m1","requesterPersonID":"m1","isEvent":false,"isWholeDay":false,"startsAt":"","endsAt":"","notifyMinutesBefore":null,"location":"","createdAt":"2026-09-02T00:00:00Z","updatedAt":"2026-09-03T00:00:00Z"}}`)},
			input:   `{"organizationHint":"ABC\uc0c1\uc0ac","title":"\ud0a5\uc624\ud504 \ubbf8\ud305","kind":"meeting","note":"\uc694\uad6c\uc0ac\ud56d\uc744 \ub4e4\uc5c8\ub2e4","occurredAt":"2026-09-02T00:00:00Z"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"activityID":"t1"`)
			},
		},
		"crm_organization_list": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_organization_list","result":{"count":1,"organizations":[{"organizationID":"o1","name":"ABC\uc0c1\uc0ac","status":"active","types":["customer"],"tags":["\uc11c\uc6b8"],"importance":"high","ownerPersonID":"m1","address":"\uc11c\uc6b8","description":"","audit":{"createdAt":"2026-09-01T00:00:00Z","createdByPersonID":"m1","updatedAt":"2026-09-04T00:00:00Z","updatedByPersonID":"m1","archivedAt":null,"archivedByPersonID":""}}]}}`)},
			input:   `{"query":"ABC"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"organizationID":"o1"`)
			},
		},
		"crm_organization_add": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_organization_add","result":{"organizationID":"o1","name":"ABC\uc0c1\uc0ac","status":"active","types":["customer"],"tags":["\uc11c\uc6b8"],"importance":"high","ownerPersonID":"m1","address":"\uc11c\uc6b8","description":"","audit":{"createdAt":"2026-09-01T00:00:00Z","createdByPersonID":"m1","updatedAt":"2026-09-04T00:00:00Z","updatedByPersonID":"m1","archivedAt":null,"archivedByPersonID":""}}}`)},
			input:   `{"name":"ABC\uc0c1\uc0ac","types":["customer"],"importance":"high"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"name":"ABC\uc0c1\uc0ac"`)
			},
		},
		"crm_organization_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_organization_update","result":{"organizationID":"o1","name":"ABC\uc0c1\uc0ac","status":"active","types":["customer"],"tags":["\uc11c\uc6b8"],"importance":"high","ownerPersonID":"m1","address":"\uc11c\uc6b8","description":"","audit":{"createdAt":"2026-09-01T00:00:00Z","createdByPersonID":"m1","updatedAt":"2026-09-04T00:00:00Z","updatedByPersonID":"m1","archivedAt":null,"archivedByPersonID":""}}}`)},
			input:   `{"organizationHint":"ABC\uc0c1\uc0ac","importance":"high"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"importance":"high"`)
			},
		},
		"crm_organization_archive": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_organization_archive","result":{"recordID":"o1","name":"ABC\uc0c1\uc0ac","archivedAt":"2026-09-04T00:00:00Z"}}`)},
			input:   `{"organizationHint":"ABC\uc0c1\uc0ac"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"recordID":"o1"`)
			},
		},
		"crm_contact_list": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_contact_list","result":{"count":1,"contacts":[{"contactID":"c1","organizationID":"o1","name":"\uc774\uc0d8\ud50c","email":"sample@example.com","phoneNumber":"","role":"\uad6c\ub9e4\ud300\uc7a5","department":"","description":"","audit":{"createdAt":"2026-09-01T00:00:00Z","createdByPersonID":"m1","updatedAt":"2026-09-04T00:00:00Z","updatedByPersonID":"m1","archivedAt":null,"archivedByPersonID":""}}]}}`)},
			input:   `{"organizationHint":"ABC\uc0c1\uc0ac"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"contactID":"c1"`)
			},
		},
		"crm_contact_add": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_contact_add","result":{"contactID":"c1","organizationID":"o1","name":"\uc774\uc0d8\ud50c","email":"sample@example.com","phoneNumber":"","role":"\uad6c\ub9e4\ud300\uc7a5","department":"","description":"","audit":{"createdAt":"2026-09-01T00:00:00Z","createdByPersonID":"m1","updatedAt":"2026-09-04T00:00:00Z","updatedByPersonID":"m1","archivedAt":null,"archivedByPersonID":""}}}`)},
			input:   `{"organizationHint":"ABC\uc0c1\uc0ac","name":"\uc774\uc0d8\ud50c","email":"sample@example.com"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"email":"sample@example.com"`)
			},
		},
		"crm_contact_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_contact_update","result":{"contactID":"c1","organizationID":"o1","name":"\uc774\uc0d8\ud50c","email":"sample@example.com","phoneNumber":"","role":"\uad6c\ub9e4\ud300\uc7a5","department":"","description":"","audit":{"createdAt":"2026-09-01T00:00:00Z","createdByPersonID":"m1","updatedAt":"2026-09-04T00:00:00Z","updatedByPersonID":"m1","archivedAt":null,"archivedByPersonID":""}}}`)},
			input:   `{"contactHint":"sample@example.com","role":"\uad6c\ub9e4\ud300\uc7a5"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"role":"\uad6c\ub9e4\ud300\uc7a5"`)
			},
		},
		"crm_contact_archive": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_contact_archive","result":{"recordID":"c1","name":"\uc774\uc0d8\ud50c","archivedAt":"2026-09-04T00:00:00Z"}}`)},
			input:   `{"contactHint":"sample@example.com"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"recordID":"c1"`)
			},
		},
		"crm_opportunity_list": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_opportunity_list","result":{"count":1,"opportunities":[{"opportunityID":"p1","organizationID":"o1","contactID":"c1","title":"ABC\uc0c1\uc0ac \ub3c4\uc785","business":"\uc601\uc5c5","pipeline":"partnership","stage":"review","stagePosition":0,"stageChangedAt":"2026-09-04T00:00:00Z","ownerPersonID":"m1","amountMinor":18000000,"currencyCode":"KRW","baseAmountMinor":null,"baseCurrencyCode":"","importance":"high","expectedCloseAt":"2026-09-30T14:59:59.999Z","expectedCloseTimeZone":"Asia/Seoul","lostReason":"","description":"","activityCount":2,"audit":{"createdAt":"2026-09-01T00:00:00Z","createdByPersonID":"m1","updatedAt":"2026-09-04T00:00:00Z","updatedByPersonID":"m1","archivedAt":null,"archivedByPersonID":""}}]}}`)},
			input:   `{"organizationHint":"ABC\uc0c1\uc0ac","stage":"review"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"stage":"review"`)
			},
		},
		"crm_opportunity_add": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_opportunity_add","result":{"opportunityID":"p1","organizationID":"o1","contactID":"c1","title":"ABC\uc0c1\uc0ac \ub3c4\uc785","business":"\uc601\uc5c5","pipeline":"partnership","stage":"waiting","stagePosition":0,"stageChangedAt":"2026-09-04T00:00:00Z","ownerPersonID":"m1","amountMinor":18000000,"currencyCode":"KRW","baseAmountMinor":null,"baseCurrencyCode":"","importance":"high","expectedCloseAt":"2026-09-30T14:59:59.999Z","expectedCloseTimeZone":"Asia/Seoul","lostReason":"","description":"","activityCount":2,"audit":{"createdAt":"2026-09-01T00:00:00Z","createdByPersonID":"m1","updatedAt":"2026-09-04T00:00:00Z","updatedByPersonID":"m1","archivedAt":null,"archivedByPersonID":""}}}`)},
			input:   `{"organizationHint":"ABC\uc0c1\uc0ac","title":"ABC\uc0c1\uc0ac \ub3c4\uc785","amountMinor":18000000,"currencyCode":"KRW"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"stage":"waiting"`)
			},
		},
		"crm_opportunity_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_opportunity_update","result":{"opportunityID":"p1","organizationID":"o1","contactID":"c1","title":"ABC\uc0c1\uc0ac \ub3c4\uc785","business":"\uc601\uc5c5","pipeline":"partnership","stage":"review","stagePosition":0,"stageChangedAt":"2026-09-04T00:00:00Z","ownerPersonID":"m1","amountMinor":18000000,"currencyCode":"KRW","baseAmountMinor":null,"baseCurrencyCode":"","importance":"high","expectedCloseAt":"2026-09-30T14:59:59.999Z","expectedCloseTimeZone":"Asia/Seoul","lostReason":"","description":"","activityCount":2,"audit":{"createdAt":"2026-09-01T00:00:00Z","createdByPersonID":"m1","updatedAt":"2026-09-04T00:00:00Z","updatedByPersonID":"m1","archivedAt":null,"archivedByPersonID":""}}}`)},
			input:   `{"opportunityHint":"ABC\uc0c1\uc0ac \ub3c4\uc785","amountMinor":18000000}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"amountMinor":18000000`)
			},
		},
		"crm_opportunity_move": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_opportunity_move","result":{"opportunityID":"p1","organizationID":"o1","contactID":"c1","title":"ABC\uc0c1\uc0ac \ub3c4\uc785","business":"\uc601\uc5c5","pipeline":"partnership","stage":"done","stagePosition":1,"stageChangedAt":"2026-09-04T00:00:00Z","ownerPersonID":"m1","amountMinor":18000000,"currencyCode":"KRW","baseAmountMinor":18000000,"baseCurrencyCode":"KRW","importance":"high","expectedCloseAt":"2026-09-30T14:59:59.999Z","expectedCloseTimeZone":"Asia/Seoul","lostReason":"","description":"","activityCount":2,"audit":{"createdAt":"2026-09-01T00:00:00Z","createdByPersonID":"m1","updatedAt":"2026-09-04T00:00:00Z","updatedByPersonID":"m1","archivedAt":null,"archivedByPersonID":""}}}`)},
			input:   `{"opportunityHint":"ABC\uc0c1\uc0ac \ub3c4\uc785","stage":"done","position":1}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"baseAmountMinor":18000000`)
			},
		},
		"crm_opportunity_archive": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_opportunity_archive","result":{"recordID":"p1","name":"ABC\uc0c1\uc0ac \ub3c4\uc785","archivedAt":"2026-09-04T00:00:00Z"}}`)},
			input:   `{"opportunityHint":"ABC\uc0c1\uc0ac \ub3c4\uc785"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"recordID":"p1"`)
			},
		},
		"crm_vocabulary_get": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_vocabulary_get","result":{"organizationTypes":[{"id":"customer","name":"\uace0\uac1d"}],"pipelines":[{"id":"partnership","name":"\ud30c\ud2b8\ub108\uc2ed"}],"stages":[{"stage":"waiting","outcome":"open"},{"stage":"in_progress","outcome":"open"},{"stage":"review","outcome":"open"},{"stage":"done","outcome":"won"},{"stage":"on_hold","outcome":"on_hold"},{"stage":"lost","outcome":"lost"}]}}`)},
			input:   `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"stage":"on_hold"`)
			},
		},
		"crm_vocabulary_set": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"crm_vocabulary_set","result":{"organizationTypes":[{"id":"customer","name":"\uace0\uac1d"}],"pipelines":[{"id":"partnership","name":"\ud30c\ud2b8\ub108\uc2ed"}],"stages":[{"stage":"waiting","outcome":"open"},{"stage":"in_progress","outcome":"open"},{"stage":"review","outcome":"open"},{"stage":"done","outcome":"won"},{"stage":"on_hold","outcome":"on_hold"},{"stage":"lost","outcome":"lost"}]}}`)},
			input:   `{"organizationTypes":[{"id":"customer","name":"\uace0\uac1d"}],"pipelines":[{"id":"partnership","name":"\ud30c\ud2b8\ub108\uc2ed"}]}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"id":"partnership"`)
			},
		},
		"task_vocabulary_set": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"task_vocabulary_set","result":{"businesses":[{"name":"\uc0ac\uc5c5\ud558\ub098","color":"#2563eb"}],"types":[{"name":"\uac1c\uc120"}],"sizes":["XS","S","M","L","XL","XXL"],"statuses":["planned","in_progress","completed","requested","paused","rejected","stopped"],"etcBusinessColor":"#94a3b8"}}`)},
			input:   `{"businesses":[{"name":"\uc0ac\uc5c5\ud558\ub098","color":"#2563eb"}],"types":[{"name":"\uac1c\uc120"}],"etcBusinessColor":"#94a3b8"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"etcBusinessColor":"#94a3b8"`)
			},
		},
		"host_version_get": {
			kind: provesBehaviour,
			reaches: map[gateBackend]*standingIn{
				admindOverTheSocket: answering(`{"installedVersion":"v2026.10.01.000000","channel":"stable","updateMethod":"apt","latestStable":{"version":"v2026.10.02.090000","publishedAt":"2026-10-02T09:00:00Z","notes":"Faster replies."},"isUpdateAvailable":true}`),
			},
			input: `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"isUpdateAvailable":true`)
			},
		},
		"host_update": {
			kind: provesBehaviour,
			reaches: map[gateBackend]*standingIn{
				admindOverTheSocket: answering(`{"status":"started","fromVersion":"v2026.10.01.000000","toVersion":"v2026.10.02.090000","startedAt":"2026-10-02T14:00:00Z","expectedDowntimeSeconds":60}`),
			},
			input: `{"targetVersion":"v2026.10.02.090000"}`,
			arrives: func(arriving capabilities.ToolInvokeRequest) capabilities.ToolInvokeRequest {
				arriving.Context.IsApprovalContinuation = true
				return arriving
			},
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"status":"started"`)
			},
		},
		"task_label_get": {
			kind: provesBehaviour,
			reaches: map[gateBackend]*standingIn{
				admindOverTheSocket: answering(`{"business":"\uc601\uc5c5","type":"","size":"M"}`),
			},
			input: `{"title":"\uc81c\uc548\uc11c"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"size":"M"`)
			},
		},
		"person_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"person_update","result":{"personID":"person-1","name":"\uc774\uc0d8\ud50c","email":"member@example.com","handle":"sample","mention":"@\uc774\uc0d8\ud50c","jobTitle":"\ud3b8\uc9d1\uc7a5","teamID":"team-1","teamName":"\ud3b8\uc9d1\ud300","supervisorID":"","supervisorName":"","phoneNumber":"","hireDate":"2026-03-02","isAdmin":false,"employmentStatus":"active"}}`)},
			input:   `{"personHint":"member@example.com","jobTitle":"\ud3b8\uc9d1\uc7a5"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"personID":"person-1"`)
			},
		},
		"person_invite": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"person_invite","result":{"personID":"person-2","email":"newcomer@example.com","name":"\ubc15\uc608\uc2dc","employmentStatus":"active","temporaryPassword":"one-time-password"}}`)},
			input:   `{"email":"newcomer@example.com","name":"\ubc15\uc608\uc2dc"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"temporaryPassword":"one-time-password"`)
			},
		},
		"team_list": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"team_list","result":{"count":2,"teams":[{"teamID":"team-1","name":"\ud3b8\uc9d1\ud300","parentTeamID":"","parentTeamName":"","position":0,"peopleCount":3},{"teamID":"team-2","name":"\uc601\uc5c5\ud300","parentTeamID":"","parentTeamName":"","position":1,"peopleCount":2}]}}`)},
			input:   `{}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"teamID":"team-2"`)
			},
		},
		"team_add": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"team_add","result":{"teamID":"team-3","name":"\ub514\uc790\uc778\ud300","parentTeamID":"team-1","parentTeamName":"\ud3b8\uc9d1\ud300","position":0,"peopleCount":0}}`)},
			input:   `{"name":"\ub514\uc790\uc778\ud300","parentHint":"\ud3b8\uc9d1\ud300"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"teamID":"team-3"`)
			},
		},
		"team_update": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"team_update","result":{"teamID":"team-3","name":"\ub514\uc790\uc778\uc2e4","parentTeamID":"","parentTeamName":"","position":2,"peopleCount":0}}`)},
			input:   `{"teamHint":"\ub514\uc790\uc778\ud300","name":"\ub514\uc790\uc778\uc2e4"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"position":2`)
			},
		},
		"team_delete": {
			kind:    provesCarrying,
			reaches: map[gateBackend]*standingIn{admindOverTheSocket: answering(`{"tool":"team_delete","result":{"teamID":"team-3","name":"\ub514\uc790\uc778\uc2e4","deleted":true,"peopleLeftWithNoOrganization":2}}`)},
			input:   `{"teamHint":"\ub514\uc790\uc778\uc2e4"}`,
			expect: func(t *testing.T, answered capabilities.ToolInvokeResponse) {
				expectSucceeded(t, answered)
				expectResultHolds(t, answered, `"peopleLeftWithNoOrganization":2`)
			},
		},
	}
}

// Why each tool this gate does not cover yet is uncovered. It exists so that a
// tool added tomorrow cannot quietly join them: a catalog tool in neither this
// map nor gateCases fails the gate. It shrinks to empty as cases land —
// internkim#1144.
const (
	implementedOnThePlaneToo = "implemented twice, once in Go here and once on the plane; step 4 of internkim#1254 deletes the Go handler, and the case written then is as thin as leave's"
	overIMAPAndSMTP          = "answered over IMAP and SMTP, which no stand-in here speaks yet"
	reachesLivePublicURLs    = "fetches live public URLs"
)

var toolsWithNoGateCaseYet = map[string]string{
	"event_add":              implementedOnThePlaneToo,
	"event_delete":           implementedOnThePlaneToo,
	"event_list":             implementedOnThePlaneToo,
	"event_update":           implementedOnThePlaneToo,
	"person_list":            implementedOnThePlaneToo,
	"task_add":               implementedOnThePlaneToo,
	"task_delete":            implementedOnThePlaneToo,
	"task_list":              implementedOnThePlaneToo,
	"task_update":            implementedOnThePlaneToo,
	"mail_connection_start":  overIMAPAndSMTP,
	"mail_connection_status": overIMAPAndSMTP,
	"mail_mailbox_list":      overIMAPAndSMTP,
	"mail_message_list":      overIMAPAndSMTP,
	"mail_message_mark":      overIMAPAndSMTP,
	"mail_message_move":      overIMAPAndSMTP,
	"mail_message_read":      overIMAPAndSMTP,
	"mail_message_search":    overIMAPAndSMTP,
	"mail_message_send":      overIMAPAndSMTP,
	"web_fetch":              reachesLivePublicURLs,
}

func TestNoCatalogToolEscapesTheGateUnnoticed(t *testing.T) {
	covered := gateCases()

	var unaccounted []string
	for _, descriptor := range capabilityprotocol.GeneratedToolDescriptorSet() {
		_, hasCase := covered[descriptor.Name]
		_, isUncovered := toolsWithNoGateCaseYet[descriptor.Name]
		if !hasCase && !isUncovered {
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
	for name, reason := range toolsWithNoGateCaseYet {
		if _, hasCase := covered[name]; hasCase {
			t.Errorf("%s has a gate case and is still listed as uncovered", name)
		}
		if !known[name] {
			t.Errorf("%s is listed as uncovered but the catalog does not offer it", name)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s is listed as uncovered and says no reason why", name)
		}
	}
}

// A case that proves only carrying must say so, and the tool it carries for
// must be one the plane implements. Otherwise the gate would count a faked
// answer as if the tool had been tested.
func TestACarryingCaseNamesAToolThePlaneImplements(t *testing.T) {
	runsOnThePlane := map[string]bool{}
	for _, name := range toolNamesAnsweredBy(capabilityprotocol.AnsweredByRecord) {
		runsOnThePlane[name] = true
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
func savedAsPicture(path string) string {
	var encoded bytes.Buffer
	if errorValue := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); errorValue != nil {
		return errorValue.Error()
	}
	if errorValue := os.WriteFile(path, encoded.Bytes(), 0o600); errorValue != nil {
		return errorValue.Error()
	}
	return "saved"
}

func runningTheBrowser() *standingIn {
	return answeringPerCall(func(request *http.Request) (int, string) {
		asked := request.URL.Path
		switch {
		case strings.Contains(asked, "snapshot"):
			return 0, `{"url":"https://example.test/","title":"예시","elements":[{"ref":"@e1","role":"button","name":"보내기"}]}`
		case strings.Contains(asked, "screenshot"):
			arguments := strings.Fields(asked)
			return 0, savedAsPicture(arguments[len(arguments)-1])
		default:
			return 0, "https://example.test/"
		}
	})
}

// chatd carries whatever the tool asks of the company's messenger, and blueclaw
// answers who the requester is. Every message tool needs both.
func reachingTheMessenger(chatd *standingIn) map[gateBackend]*standingIn {
	return map[gateBackend]*standingIn{
		chatdOverHTTP:    chatd,
		blueclawOverHTTP: answering(memberPolicyFor("person-1", "member@example.com")),
	}
}

func inAChannel(arriving capabilities.ToolInvokeRequest) capabilities.ToolInvokeRequest {
	arriving.Context.Platform = "buzz"
	arriving.Context.ConversationID = "channel-1"
	arriving.Context.ConversationType = "channel"
	arriving.Context.ChannelID = "channel-1"
	arriving.Context.ChannelName = "전사-공지"
	arriving.Context.RequesterPersonID = "person-1"
	return arriving
}

func memberPolicyFor(personID string, email string) string {
	// The circle the message tools require is the code's to name, so the case
	// asks for it rather than spelling it — internkim#507 renames it.
	return `{"people":[{"personID":"` + personID + `","displayName":"이샘플","emails":["` + email + `"],"circles":["member"]}]}`
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
