package centralplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recordedToolCall struct {
	tool      string
	requester string
	input     map[string]any
}

type directoryStub struct {
	server  *httptest.Server
	teams   []map[string]any
	seated  map[string]string
	calls   []recordedToolCall
	nextIDs int
}

func newDirectoryStub(t *testing.T) *directoryStub {
	t.Helper()
	stub := &directoryStub{seated: map[string]string{"one@example.com": "member-one"}}
	stub.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/api/agent/session":
			var asked struct{ ExternalID string }
			_ = json.NewDecoder(request.Body).Decode(&asked)
			writeJSON(writer, map[string]any{
				"memberID":    "member-for-" + asked.ExternalID,
				"accessToken": "token-for-" + asked.ExternalID,
				"expiresAt":   4102444800,
			})
		case request.URL.Path == "/api/agent/member":
			stub.answerTheDirectory(writer, request)
		case strings.HasPrefix(request.URL.Path, "/api/v1/tools/"):
			stub.answerTheTool(writer, request)
		default:
			http.Error(writer, "the stub was asked for "+request.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

// Only the one-person lookup: a directory write runs as the administrator who
// claimed the device, so anything reading the whole roster to decide who to
// write as is a mistake this stub should name rather than answer.
func (stub *directoryStub) answerTheDirectory(writer http.ResponseWriter, request *http.Request) {
	email := request.URL.Query().Get("email")
	if email == "" {
		http.Error(writer, "these writes do not read the roster", http.StatusNotFound)
		return
	}
	memberID, isSeated := stub.seated[email]
	if !isSeated {
		writeJSON(writer, map[string]any{"member": nil})
		return
	}
	writeJSON(writer, map[string]any{"member": map[string]any{"memberID": memberID, "email": email, "name": "이샘플"}})
}

func (stub *directoryStub) answerTheTool(writer http.ResponseWriter, request *http.Request) {
	toolName := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/api/v1/tools/"), "/invoke")
	var payload struct {
		Input map[string]any `json:"input"`
	}
	_ = json.NewDecoder(request.Body).Decode(&payload)
	stub.calls = append(stub.calls, recordedToolCall{
		tool:      toolName,
		requester: strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer token-for-"),
		input:     payload.Input,
	})

	switch toolName {
	case "team_list":
		writeJSON(writer, map[string]any{"tool": toolName, "result": map[string]any{"teams": stub.teams}})
	case "team_add":
		stub.nextIDs++
		made := map[string]any{
			"teamID":       "team-made-" + string(rune('a'+stub.nextIDs-1)),
			"name":         payload.Input["name"],
			"parentTeamID": stringOr(payload.Input["parentHint"], ""),
			"position":     numberOr(payload.Input["position"], 0),
		}
		stub.teams = append(stub.teams, made)
		writeJSON(writer, map[string]any{"tool": toolName, "result": made})
	case "team_update":
		written := stub.applyTeamUpdate(payload.Input)
		writeJSON(writer, map[string]any{"tool": toolName, "result": written})
	case "team_delete":
		stub.removeTeam(stringOr(payload.Input["teamHint"], ""))
		writeJSON(writer, map[string]any{"tool": toolName, "result": map[string]any{"deleted": true}})
	case "person_invite":
		writeJSON(writer, map[string]any{"tool": toolName, "result": map[string]any{
			"personID":          "member-new",
			"email":             payload.Input["email"],
			"name":              payload.Input["name"],
			"employmentStatus":  "invited",
			"temporaryPassword": "abcd-efgh-ijkl",
		}})
	case "person_update":
		writeJSON(writer, map[string]any{"tool": toolName, "result": map[string]any{
			"personID": "member-one",
			"name":     stringOr(payload.Input["name"], "이샘플"),
		}})
	default:
		http.Error(writer, "the stub answers no tool called "+toolName, http.StatusNotFound)
	}
}

func (stub *directoryStub) applyTeamUpdate(input map[string]any) map[string]any {
	teamID := stringOr(input["teamHint"], "")
	for _, team := range stub.teams {
		if team["teamID"] != teamID {
			continue
		}
		team["name"] = input["name"]
		team["parentTeamID"] = stringOr(input["parentHint"], "")
		team["position"] = numberOr(input["position"], 0)
		return team
	}
	return map[string]any{"teamID": teamID}
}

func (stub *directoryStub) removeTeam(teamID string) {
	kept := []map[string]any{}
	for _, team := range stub.teams {
		if team["teamID"] != teamID {
			kept = append(kept, team)
		}
	}
	stub.teams = kept
}

func (stub *directoryStub) toolsCalled() []string {
	names := []string{}
	for _, call := range stub.calls {
		names = append(names, call.tool)
	}
	return names
}

func (stub *directoryStub) client() *Client {
	return stub.clientClaimedBy("boss@example.com")
}

func (stub *directoryStub) clientClaimedBy(administratorEmail string) *Client {
	return New(Settings{
		AppURL:                    stub.server.URL,
		HostCredential:            func() string { return "agent-key" },
		ProjectURL:                stub.server.URL,
		PublishableKey:            "publishable",
		ClaimedAdministratorEmail: func() string { return administratorEmail },
	})
}

func stringOr(value any, fallback string) string {
	if held, isText := value.(string); isText {
		return held
	}
	return fallback
}

func numberOr(value any, fallback int) int {
	if held, isNumber := value.(float64); isNumber {
		return int(held)
	}
	return fallback
}

func TestADirectoryWriteRunsAsTheAdministratorWhoClaimedTheDevice(t *testing.T) {
	stub := newDirectoryStub(t)

	if _, errorValue := stub.clientClaimedBy("Early@Example.com").SettleTeams(context.Background(), nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(stub.calls) == 0 {
		t.Fatal("settling teams read nothing from the record")
	}
	for _, call := range stub.calls {
		if call.requester != "early@example.com" {
			t.Fatalf("%s ran as %q; a directory write runs as the administrator who claimed this device", call.tool, call.requester)
		}
	}
}

func TestADirectoryWriteRefusesWhenNobodyHasClaimedTheDevice(t *testing.T) {
	stub := newDirectoryStub(t)

	_, errorValue := stub.clientClaimedBy("  ").SettleTeams(context.Background(), nil)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "no administrator has claimed this device") {
		t.Fatalf("error = %v; a write with nobody to run as says why", errorValue)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("called %v; a write it cannot attribute reaches the record for nothing", stub.toolsCalled())
	}
}

func TestADirectoryWriteRefusesWhenTheDeviceWasBuiltWithNoClaimAtAll(t *testing.T) {
	stub := newDirectoryStub(t)
	client := New(Settings{
		AppURL:         stub.server.URL,
		HostCredential: func() string { return "agent-key" },
		ProjectURL:     stub.server.URL,
		PublishableKey: "publishable",
	})

	_, errorValue := client.SettleTeams(context.Background(), nil)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "names no claimed administrator") {
		t.Fatalf("error = %v; a client with no claim at all says so rather than writing as nobody", errorValue)
	}
}

func TestSettlingTeamsAddsWhatIsNewAndLeavesWhatAlreadyAgrees(t *testing.T) {
	stub := newDirectoryStub(t)
	stub.teams = []map[string]any{
		{"teamID": "team-product", "name": "제품", "parentTeamID": "", "position": 0},
	}

	settled, errorValue := stub.client().SettleTeams(context.Background(), []OfferedTeam{
		{TeamID: "team-product", Name: "제품"},
		{Name: "디자인", ParentName: "제품"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(settled) != 2 || settled[0].TeamID != "team-product" {
		t.Fatalf("settled = %#v; an organization already held keeps its id", settled)
	}
	if settled[1].ParentTeamID != "team-product" {
		t.Fatalf("settled = %#v; a new organization takes the parent it was offered under", settled)
	}
	called := strings.Join(stub.toolsCalled(), ",")
	if strings.Count(called, "team_add") != 1 {
		t.Fatalf("called %s; only the organization nobody held is added", called)
	}
	if strings.Contains(called, "team_delete") {
		t.Fatalf("called %s; nothing was left out of the offer", called)
	}
}

func TestSettlingTeamsRemovesWhatNobodyOfferedFromTheLeavesUp(t *testing.T) {
	stub := newDirectoryStub(t)
	stub.teams = []map[string]any{
		{"teamID": "team-product", "name": "제품", "parentTeamID": "", "position": 0},
		{"teamID": "team-design", "name": "디자인", "parentTeamID": "team-product", "position": 1},
		{"teamID": "team-kept", "name": "영업", "parentTeamID": "", "position": 2},
	}

	if _, errorValue := stub.client().SettleTeams(context.Background(), []OfferedTeam{{TeamID: "team-kept", Name: "영업"}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	removed := []string{}
	for _, call := range stub.calls {
		if call.tool == "team_delete" {
			removed = append(removed, stringOr(call.input["teamHint"], ""))
		}
	}
	if len(removed) != 2 || removed[0] != "team-design" || removed[1] != "team-product" {
		t.Fatalf("removed %v; a parent cannot go before the organization under it", removed)
	}
}

func TestSeatingSomebodyTheDirectoryDoesNotHoldGoesThroughTheInvitePath(t *testing.T) {
	stub := newDirectoryStub(t)

	seated, errorValue := stub.client().EnsureMember(context.Background(), "New@Example.com", "새사람")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if seated.MemberID != "member-new" || seated.Status != "invited" {
		t.Fatalf("seated = %#v; person_invite answers with the seat it made", seated)
	}
	called := stub.toolsCalled()
	if len(called) != 1 || called[0] != "person_invite" {
		t.Fatalf("called %v; seating somebody new is one invite", called)
	}
	if stub.calls[0].input["email"] != "new@example.com" {
		t.Fatalf("invited %#v; the address is written the way the record holds it", stub.calls[0].input)
	}
}

func TestSeatingSomebodyAlreadyHeldRenamesThemRatherThanInvitingThemAgain(t *testing.T) {
	stub := newDirectoryStub(t)

	seated, errorValue := stub.client().EnsureMember(context.Background(), "one@example.com", "박예시")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if seated.MemberID != "member-one" {
		t.Fatalf("seated = %#v; somebody already seated keeps the seat they have", seated)
	}
	called := stub.toolsCalled()
	if len(called) != 1 || called[0] != "person_update" {
		t.Fatalf("called %v; a name is a person_update, never a second invitation", called)
	}
}

func TestSeatingSomebodyAlreadyHeldUnderTheSameNameWritesNothing(t *testing.T) {
	stub := newDirectoryStub(t)

	if _, errorValue := stub.client().EnsureMember(context.Background(), "one@example.com", "이샘플"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if called := stub.toolsCalled(); len(called) != 0 {
		t.Fatalf("called %v; a name that already reads that way is not a write", called)
	}
}

func TestAProfileIsWrittenAgainstTheTeamIDRatherThanTheNameItNamed(t *testing.T) {
	stub := newDirectoryStub(t)
	stub.teams = []map[string]any{{"teamID": "team-product", "name": "제품", "parentTeamID": "", "position": 0}}

	written, errorValue := stub.client().WriteOrganizationProfiles(context.Background(), []OrganizationProfile{
		{Email: "One@Example.com", JobTitle: "편집장", TeamName: "제품", SupervisorEmail: "Boss@Example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(written) != 1 || written[0] != "one@example.com" {
		t.Fatalf("written = %v; the answer names who was written", written)
	}
	update := stub.calls[len(stub.calls)-1]
	if update.tool != "person_update" {
		t.Fatalf("last call was %s; a profile is a person_update", update.tool)
	}
	if update.input["teamHint"] != "team-product" {
		t.Fatalf("teamHint = %v; a name that resolved is sent as the id it resolved to", update.input["teamHint"])
	}
	if update.input["supervisorHint"] != "boss@example.com" {
		t.Fatalf("supervisorHint = %v; a supervisor is named by their address", update.input["supervisorHint"])
	}
}

func TestAProfileNamingATeamNobodyHoldsAddsThatTeamFirst(t *testing.T) {
	stub := newDirectoryStub(t)

	if _, errorValue := stub.client().WriteOrganizationProfiles(context.Background(), []OrganizationProfile{
		{Email: "one@example.com", TeamName: "새로운팀"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	called := stub.toolsCalled()
	if len(called) != 3 || called[0] != "team_list" || called[1] != "team_add" || called[2] != "person_update" {
		t.Fatalf("called %v; the team the profile names is settled before the profile is written", called)
	}
}
