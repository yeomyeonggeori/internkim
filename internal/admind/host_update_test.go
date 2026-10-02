package admind

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/hostupdate"
	capabilityschema "github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol/jsonschema"
)

const (
	hostUpdateAdminEmail  = "member1@example.com"
	hostUpdateMemberEmail = "member2@example.com"
	hostUpdateBlueclawURL = "https://blueclaw.example.test"
)

type hostUpdateRig struct {
	service       *Service
	notePath      string
	started       []string
	isRunning     bool
	noteAtStart   *hostupdate.Note
	channel       string
	method        string
	installed     string
	reportBodies  []string
	reportStatus  int
	releaseServer *httptest.Server
}

func newHostUpdateRig(t *testing.T) *hostUpdateRig {
	t.Helper()
	rig := &hostUpdateRig{channel: "stable", method: "apt-get", installed: "2026.10.01.000000", reportStatus: http.StatusCreated}
	rig.notePath = filepath.Join(t.TempDir(), "host-update.json")
	rig.releaseServer = servingStableReleases(t)
	keyPath := filepath.Join(t.TempDir(), "assertion-key")
	os.WriteFile(keyPath, []byte("a-test-assertion-key-of-enough-length"), 0o600)
	rig.service = NewService(Configuration{StateDirectory: t.TempDir(), BlueclawBaseURL: hostUpdateBlueclawURL, BlueclawAssertionKeyPath: keyPath})
	seatPeopleInACompanyDirectoryForTest(t, rig.service)
	directory := companyDirectoryHolding(
		memberForTest(hostUpdateAdminEmail, "이샘플", "admin"),
		memberForTest(hostUpdateMemberEmail, "박예시", "member"),
	)
	rig.service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case isCompanyDirectoryRequest(request):
			return directory.respond(t, request)
		case strings.HasPrefix(request.URL.String(), hostUpdateBlueclawURL+hostUpdateScheduleCreate):
			body, _ := io.ReadAll(request.Body)
			rig.reportBodies = append(rig.reportBodies, string(body))
			return jsonResponse(rig.reportStatus, `{"scheduleID":"schedule-1"}`, nil), nil
		default:
			return jsonResponse(http.StatusBadGateway, `{}`, nil), nil
		}
	})}
	rig.service.hostUpdateDependencies = rig.dependencies()
	return rig
}

func servingStableReleases(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		io.WriteString(responseWriter, `[{"tag_name":"v2026.10.03.000000","prerelease":true},{"tag_name":"v2026.10.02.090000","published_at":"2026-10-02T09:00:00Z","body":"Faster replies."},{"tag_name":"v2026.10.01.000000"},{"tag_name":"v2026.09.30.000000"}]`)
	}))
	t.Cleanup(server.Close)
	return server
}

func (rig *hostUpdateRig) dependencies() hostUpdateDependencies {
	return hostUpdateDependencies{
		Machine: hostupdate.Machine{
			LookPath: func(name string) (string, error) {
				if name == rig.method {
					return "/usr/bin/" + name, nil
				}
				return "", errors.New("not found")
			},
			ReadFile: func(path string) ([]byte, error) {
				if path == hostupdate.ChannelPath && rig.channel != "" {
					return []byte(rig.channel + "\n"), nil
				}
				return nil, os.ErrNotExist
			},
			InstalledVersion: hostupdate.TagOf(rig.installed),
		},
		Releases: hostupdate.ReleaseSource{APIURL: rig.releaseServer.URL},
		Supervisor: hostupdate.Supervisor{
			Run: func(name string, arguments ...string) ([]byte, error) {
				if name == "systemctl" {
					if rig.isRunning {
						return nil, nil
					}
					return nil, errors.New("inactive")
				}
				if note, isPending, _ := hostupdate.ReadNote(rig.notePath); isPending {
					rig.noteAtStart = &note
				}
				rig.started = append(rig.started, strings.Join(arguments, " "))
				return nil, nil
			},
		},
		NotePath:    rig.notePath,
		HostCommand: "/usr/bin/internkim",
		Now:         func() time.Time { return time.Date(2026, 10, 2, 14, 0, 0, 0, time.UTC) },
	}
}

func (rig *hostUpdateRig) ask(t *testing.T, path string, email string, body string) *httptest.ResponseRecorder {
	t.Helper()
	rig.service.hostUpdateDependencies = rig.dependencies()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set(requesterEmailHeader, email)
	response := httptest.NewRecorder()
	markRequestsAsAssertedByTheListener(rig.service.router()).ServeHTTP(response, request)
	return response
}

func errorCodeOf(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var refusal struct {
		ErrorCode string `json:"errorCode"`
	}
	json.Unmarshal(response.Body.Bytes(), &refusal)
	return refusal.ErrorCode
}

const approvedStart = `{"isApproved":true,"input":{},"requester":{"personID":"person-1","platform":"buzz","conversationID":"conversation-1","replyTargetID":"reply-1"}}`

func TestAMemberWhoIsNotAnAdministratorIsRefusedTheUpdate(t *testing.T) {
	rig := newHostUpdateRig(t)
	for path, body := range map[string]string{hostUpdatePlanPath: `{}`, hostUpdateStartPath: approvedStart} {
		response := rig.ask(t, path, hostUpdateMemberEmail, body)
		if response.Code != http.StatusForbidden || errorCodeOf(t, response) != "access_denied" {
			t.Fatalf("%s answered a member %d %s", path, response.Code, response.Body.String())
		}
	}
	if len(rig.started) != 0 {
		t.Fatalf("a member's request started %v", rig.started)
	}
	if _, isPending, _ := hostupdate.ReadNote(rig.notePath); isPending {
		t.Fatal("a member's request left a pending note")
	}
}

type hostVersionForTest struct {
	InstalledVersion  string              `json:"installedVersion"`
	Channel           string              `json:"channel"`
	UpdateMethod      string              `json:"updateMethod"`
	LatestStable      *hostupdate.Release `json:"latestStable"`
	PreviousStable    *hostupdate.Release `json:"previousStable"`
	IsUpdateAvailable bool                `json:"isUpdateAvailable"`
}

func TestAnyMemberCanReadTheHostVersion(t *testing.T) {
	rig := newHostUpdateRig(t)
	response := rig.ask(t, hostVersionPath, hostUpdateMemberEmail, `{}`)
	var answer hostVersionForTest
	json.Unmarshal(response.Body.Bytes(), &answer)
	if response.Code != http.StatusOK || answer.InstalledVersion != "v2026.10.01.000000" || answer.Channel != "stable" || answer.UpdateMethod != "apt" ||
		answer.LatestStable == nil || answer.LatestStable.Version != "v2026.10.02.090000" || answer.LatestStable.Notes != "Faster replies." ||
		!answer.IsUpdateAvailable || answer.PreviousStable == nil || answer.PreviousStable.Version != "v2026.09.30.000000" {
		t.Fatalf("the version answer is %d %s", response.Code, response.Body.String())
	}
}

type hostUpdateTargetForTest struct {
	capabilities.ApprovalTarget
	Choices []hostUpdateChoice `json:"choices"`
}

func TestTheConfirmationCarriesTheFactsAndOffersTheNightOrNow(t *testing.T) {
	rig := newHostUpdateRig(t)
	response := rig.ask(t, hostUpdatePlanPath, hostUpdateAdminEmail, `{"input":{}}`)
	var target hostUpdateTargetForTest
	json.Unmarshal(response.Body.Bytes(), &target)
	if response.Code != http.StatusOK || target.ID != "v2026.10.02.090000" || target.InputField != "targetVersion" {
		t.Fatalf("the plan is %d %s", response.Code, response.Body.String())
	}
	if len(target.Choices) != 2 || target.Choices[0] != (hostUpdateChoice{Key: "offHours", StartsAt: "2026-10-03T03:00:00+09:00"}) || target.Choices[1].Key != "now" {
		t.Fatalf("the choices are %+v", target.Choices)
	}
	var consequences map[string]any
	json.Unmarshal([]byte(target.Preview), &consequences)
	if consequences["fromVersion"] != "v2026.10.01.000000" || consequences["releaseNotes"] != "Faster replies." ||
		consequences["expectedDowntimeSeconds"] != float64(expectedHostUpdateDowntimeSeconds) || consequences["isRollback"] != false {
		t.Fatalf("the consequences are %+v", consequences)
	}
	asked := rig.ask(t, hostUpdatePlanPath, hostUpdateAdminEmail, `{"input":{"isRequestedNow":true}}`)
	json.Unmarshal(asked.Body.Bytes(), &target)
	if target.Choices[0].Key != "now" {
		t.Fatalf("an admin who asked for now is offered %+v first", target.Choices[0])
	}
}

func TestAHostOffTheStableChannelIsRefused(t *testing.T) {
	for _, channel := range []string{"testing", ""} {
		rig := newHostUpdateRig(t)
		rig.channel = channel
		response := rig.ask(t, hostUpdateStartPath, hostUpdateAdminEmail, approvedStart)
		if response.Code != http.StatusConflict || errorCodeOf(t, response) != "channel_not_stable" || len(rig.started) != 0 {
			t.Fatalf("a host on %q answered %d %s", channel, response.Code, response.Body.String())
		}
	}
}

func TestASecondUpdateIsRefusedWhileOneRuns(t *testing.T) {
	rig := newHostUpdateRig(t)
	if response := rig.ask(t, hostUpdateStartPath, hostUpdateAdminEmail, approvedStart); response.Code != http.StatusOK {
		t.Fatalf("the first update answered %d %s", response.Code, response.Body.String())
	}
	second := rig.ask(t, hostUpdateStartPath, hostUpdateAdminEmail, approvedStart)
	if second.Code != http.StatusConflict || errorCodeOf(t, second) != "update_in_progress" || len(rig.started) != 1 {
		t.Fatalf("a second update answered %d %s and started %v", second.Code, second.Body.String(), rig.started)
	}
	hostupdate.ClearNote(rig.notePath)
	rig.isRunning = true
	if third := rig.ask(t, hostUpdateStartPath, hostUpdateAdminEmail, approvedStart); errorCodeOf(t, third) != "update_in_progress" {
		t.Fatalf("an update while the unit runs answered %d %s", third.Code, third.Body.String())
	}
}

func TestThePendingNoteIsWrittenBeforeTheUnitStarts(t *testing.T) {
	rig := newHostUpdateRig(t)
	response := rig.ask(t, hostUpdateStartPath, hostUpdateAdminEmail, approvedStart)
	if response.Code != http.StatusOK {
		t.Fatalf("the update answered %d %s", response.Code, response.Body.String())
	}
	note := rig.noteAtStart
	if note == nil || note.Requester.Email != hostUpdateAdminEmail || note.Requester.ConversationID != "conversation-1" ||
		note.FromVersion != "v2026.10.01.000000" || note.ToVersion != "v2026.10.02.090000" || note.IsFinished() {
		t.Fatalf("when the unit started the note read %+v", note)
	}
	if rig.started[0] != "--unit=internkim-host-update --description=internkim host update to v2026.10.02.090000 --collect /usr/bin/internkim update --version v2026.10.02.090000" {
		t.Fatalf("systemd-run was given %s", rig.started[0])
	}
}

func TestAnUnapprovedStartIsRefused(t *testing.T) {
	rig := newHostUpdateRig(t)
	response := rig.ask(t, hostUpdateStartPath, hostUpdateAdminEmail, `{"input":{}}`)
	if response.Code != http.StatusForbidden || errorCodeOf(t, response) != "approval_required" || len(rig.started) != 0 {
		t.Fatalf("an unapproved start answered %d %s", response.Code, response.Body.String())
	}
}

func TestAnAdministratorCanGoBackToAnOlderStableRelease(t *testing.T) {
	rig := newHostUpdateRig(t)
	response := rig.ask(t, hostUpdateStartPath, hostUpdateAdminEmail, `{"isApproved":true,"input":{"targetVersion":"v2026.09.30.000000"}}`)
	if response.Code != http.StatusOK || !strings.HasSuffix(rig.started[0], "--version v2026.09.30.000000") {
		t.Fatalf("a rollback answered %d %s", response.Code, response.Body.String())
	}
	hostupdate.ClearNote(rig.notePath)
	for _, version := range []string{"v2026.10.03.000000", "v1999.01.01.000000"} {
		refused := rig.ask(t, hostUpdateStartPath, hostUpdateAdminEmail, `{"isApproved":true,"input":{"targetVersion":"`+version+`"}}`)
		if errorCodeOf(t, refused) != "unknown_stable_release" {
			t.Fatalf("%s answered %d %s", version, refused.Code, refused.Body.String())
		}
	}
}

func TestAMacHostIsRefusedWithWhatAnAdministratorRunsInstead(t *testing.T) {
	rig := newHostUpdateRig(t)
	rig.method = "brew"
	response := rig.ask(t, hostUpdateStartPath, hostUpdateAdminEmail, approvedStart)
	if errorCodeOf(t, response) != "update_method_unsupported" || len(rig.started) != 0 {
		t.Fatalf("a Mac answered %d %s", response.Code, response.Body.String())
	}
}

func finishedNote(t *testing.T, notePath string, isSucceeded bool) {
	t.Helper()
	outcome := &hostupdate.Outcome{FinishedAt: time.Now(), Succeeded: isSucceeded}
	if !isSucceeded {
		outcome.Error = "install.sh failed: exit status 1"
		outcome.OutputTail = "apt said no"
	}
	hostupdate.WriteNote(notePath, hostupdate.Note{
		Requester:   hostupdate.Requester{Email: hostUpdateAdminEmail, PersonID: "person-1", Platform: "buzz", ConversationID: "conversation-1", ReplyTargetID: "reply-1"},
		FromVersion: "v2026.10.01.000000", ToVersion: "v2026.10.02.090000", StartedAt: time.Now().Add(-time.Minute), Outcome: outcome,
	})
}

func TestAFinishedUpdateIsReportedInItsConversationAndTheNoteCleared(t *testing.T) {
	for _, isSucceeded := range []bool{true, false} {
		rig := newHostUpdateRig(t)
		finishedNote(t, rig.notePath, isSucceeded)
		rig.service.reportHostUpdateOnce(t.Context())
		if len(rig.reportBodies) != 1 {
			t.Fatalf("the result was reported %d times", len(rig.reportBodies))
		}
		var schedule hostUpdateReportSchedule
		json.Unmarshal([]byte(rig.reportBodies[0]), &schedule)
		if schedule.Kind != "once" || schedule.ConversationID != "conversation-1" || schedule.ReplyTargetID != "reply-1" || schedule.Platform != "buzz" || schedule.RunAt == "" || schedule.TimeZone == "" {
			t.Fatalf("the report is %s", rig.reportBodies[0])
		}
		var fields map[string]any
		json.Unmarshal([]byte(rig.reportBodies[0]), &fields)
		for name, value := range fields {
			if value == nil {
				t.Fatalf("the report carries %s as null, which the agent's schedule schema refuses: %s", name, rig.reportBodies[0])
			}
		}
		if !strings.Contains(schedule.TaskInstruction, `"succeeded":`+map[bool]string{true: "true", false: "false"}[isSucceeded]) {
			t.Fatalf("the report carries %s", schedule.TaskInstruction)
		}
		if strings.Contains(schedule.TaskInstruction, "apt said no") == isSucceeded || !strings.Contains(schedule.TaskInstruction, `"installedVersionNow":"v2026.10.01.000000"`) {
			t.Fatalf("the report carries %s", schedule.TaskInstruction)
		}
		if _, isPending, _ := hostupdate.ReadNote(rig.notePath); isPending {
			t.Fatal("a delivered result left its note")
		}
	}
}

func TestTheResultReportGivesItsTimesInTheCompanyTimeZone(t *testing.T) {
	rig := newHostUpdateRig(t)
	finishedNote(t, rig.notePath, true)
	note, _, _ := hostupdate.ReadNote(rig.notePath)
	note.StartedAt = note.StartedAt.UTC()
	note.Outcome.FinishedAt = note.Outcome.FinishedAt.UTC()
	body, errorValue := rig.service.hostUpdateReportSchedule(t.Context(), note)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var schedule hostUpdateReportSchedule
	json.Unmarshal(body, &schedule)
	companyLocation := rig.service.companyTimeLocation(t.Context())
	for _, want := range []string{
		`"startedAt":"` + note.StartedAt.In(companyLocation).Format(time.RFC3339) + `"`,
		`"finishedAt":"` + note.Outcome.FinishedAt.In(companyLocation).Format(time.RFC3339Nano) + `"`,
	} {
		if !strings.Contains(schedule.TaskInstruction, want) {
			t.Fatalf("the report lacks %s in %s: %s", want, companyLocation, schedule.TaskInstruction)
		}
	}
}

func TestAResultTheAgentCannotTakeYetIsKeptForTheNextTry(t *testing.T) {
	rig := newHostUpdateRig(t)
	rig.reportStatus = http.StatusServiceUnavailable
	finishedNote(t, rig.notePath, true)
	rig.service.reportHostUpdateOnce(t.Context())
	if _, isPending, _ := hostupdate.ReadNote(rig.notePath); !isPending {
		t.Fatal("a result the agent could not take was dropped")
	}
}

func TestAnUpdateStillRunningIsNotReported(t *testing.T) {
	rig := newHostUpdateRig(t)
	rig.isRunning = true
	hostupdate.WriteNote(rig.notePath, hostupdate.Note{Requester: hostupdate.Requester{Email: hostUpdateAdminEmail, PersonID: "person-1"}, StartedAt: time.Date(2026, 10, 2, 13, 0, 0, 0, time.UTC)})
	rig.service.hostUpdateDependencies = rig.dependencies()
	rig.service.reportHostUpdateOnce(t.Context())
	if len(rig.reportBodies) != 0 {
		t.Fatalf("an unfinished update was reported: %v", rig.reportBodies)
	}
}

func TestAnUpdateThatStoppedWithoutAResultIsReportedAsFailed(t *testing.T) {
	rig := newHostUpdateRig(t)
	hostupdate.WriteNote(rig.notePath, hostupdate.Note{Requester: hostupdate.Requester{Email: hostUpdateAdminEmail, PersonID: "person-1", ConversationID: "conversation-1"}, ToVersion: "v2026.10.02.090000", StartedAt: time.Date(2026, 10, 2, 13, 0, 0, 0, time.UTC)})
	rig.service.hostUpdateDependencies = rig.dependencies()
	rig.service.reportHostUpdateOnce(t.Context())
	if len(rig.reportBodies) != 1 || !strings.Contains(rig.reportBodies[0], `\"succeeded\":false`) || !strings.Contains(rig.reportBodies[0], "stopped before it recorded a result") {
		t.Fatalf("an abandoned update was reported as %v", rig.reportBodies)
	}
}

func blueclawScheduleCreateSchema(t *testing.T) json.RawMessage {
	t.Helper()
	source, errorValue := os.ReadFile("../../.dependency/blueclaw/internal/adminapi/schedule_contracts.go")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	match := regexp.MustCompile("(?s)var scheduleToolCreateInputSchema = json.RawMessage\\(`(.*?)`\\)").FindSubmatch(source)
	if match == nil {
		t.Fatal("blueclaw no longer declares scheduleToolCreateInputSchema where this reads it")
	}
	return json.RawMessage(match[1])
}

func TestTheResultReportIsAScheduleTheAgentAccepts(t *testing.T) {
	rig := newHostUpdateRig(t)
	finishedNote(t, rig.notePath, false)
	note, _, _ := hostupdate.ReadNote(rig.notePath)
	body, errorValue := rig.service.hostUpdateReportSchedule(t.Context(), note)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := capabilityschema.ValidateInput(blueclawScheduleCreateSchema(t), body); errorValue != nil {
		t.Fatalf("blueclaw refuses the result report %s: %v", body, errorValue)
	}
}
