package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/eastriver/internkim/internal/mail"
)

func livePlaneOrSkip(t *testing.T) (appURL string, projectURL string, publishableKey string, agentKey string) {
	t.Helper()
	appURL = os.Getenv("LIVE_PLANE_APP_URL")
	projectURL = os.Getenv("LIVE_PLANE_PROJECT_URL")
	publishableKey = os.Getenv("LIVE_PLANE_PUBLISHABLE_KEY")
	agentKey = os.Getenv("LIVE_PLANE_AGENT_KEY")
	if appURL == "" || projectURL == "" || publishableKey == "" || agentKey == "" {
		t.Skip("no live plane named; set LIVE_PLANE_APP_URL, LIVE_PLANE_PROJECT_URL, LIVE_PLANE_PUBLISHABLE_KEY, LIVE_PLANE_AGENT_KEY")
	}
	return appURL, projectURL, publishableKey, agentKey
}

func liveRequesterOrSkip(t *testing.T) (personID string, externalID string) {
	t.Helper()
	personID = os.Getenv("LIVE_REQUESTER_PERSON_ID")
	externalID = os.Getenv("LIVE_REQUESTER_EXTERNAL_ID")
	if personID == "" || externalID == "" {
		t.Skip("no live requester named; set LIVE_REQUESTER_PERSON_ID and LIVE_REQUESTER_EXTERNAL_ID")
	}
	return personID, externalID
}

func directoryServing(t *testing.T, records []adminUserMutation) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if errorValue := json.NewEncoder(writer).Encode(pagesUsersResponse{Records: records}); errorValue != nil {
			t.Error(errorValue)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func blueclawServingRunsAndPolicy(t *testing.T, runsByCall ...[]taskNotifyRun) *httptest.Server {
	t.Helper()
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/admin/api/policy":
			writer.Write([]byte(`{"people":[]}`))
		case request.URL.Path == "/admin/api/run/detail":
			writer.Write([]byte(`{"taskEvents":[{"name":"confirmation.requested","userFacingMessage":"메일을 보내도 될까요?"}]}`))
		case request.URL.Path != "/admin/api/run":
			http.NotFound(writer, request)
		default:
			runs := runsByCall[len(runsByCall)-1]
			if call < len(runsByCall) {
				runs = runsByCall[call]
			}
			call++
			if errorValue := json.NewEncoder(writer).Encode(runs); errorValue != nil {
				t.Error(errorValue)
			}
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func writeLiveFile(t *testing.T, directory string, name string, contents string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if errorValue := os.WriteFile(path, []byte(contents), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

func TestALiveApprovalReachesTheRequestersDevices(t *testing.T) {
	appURL, projectURL, publishableKey, agentKey := livePlaneOrSkip(t)
	personID, externalID := liveRequesterOrSkip(t)

	waiting := []taskNotifyRun{{
		TaskRunID:         "live-run-1",
		Status:            "waiting_approval",
		RequesterPersonID: personID,
		Prompt:            "거래처에 보낼 메일 초안을 써줘",
	}}
	running := []taskNotifyRun{{
		TaskRunID:         "live-run-1",
		Status:            "running",
		RequesterPersonID: personID,
		Prompt:            "거래처에 보낼 메일 초안을 써줘",
	}}

	blueclaw := blueclawServingRunsAndPolicy(t, running, waiting, waiting)
	directory := directoryServing(t, []adminUserMutation{
		{MemberID: personID, MattermostUserID: externalID, Email: "live@example.com", Status: "active"},
	})

	state := t.TempDir()
	service := NewService(Configuration{
		DatabasePath:               filepath.Join(state, "internkim.sqlite"),
		BlueclawBaseURL:            blueclaw.URL,
		APIBaseURL:                 directory.URL,
		FleetIDPath:                writeLiveFile(t, state, "fleet-id", "live-fleet"),
		FleetSecretPath:            writeLiveFile(t, state, "fleet-secret", "live-secret"),
		CentralPlaneAppURL:         appURL,
		CentralPlaneProjectURL:     projectURL,
		CentralPlanePublishableKey: publishableKey,
		CentralPlaneAgentKeyPath:   writeLiveFile(t, state, "agent-key", agentKey),
	})

	ctx := context.Background()
	service.notifyTaskRunsOnce(ctx)
	service.notifyTaskRunsOnce(ctx)
	service.notifyTaskRunsOnce(ctx)

	marks, errorValue := service.readTaskNotifyMarks(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if marks["live-run-1"] != "waiting_approval" {
		t.Fatalf("the run was not marked as told: %+v", marks)
	}
}

func TestALiveMailArrivalReachesTheOwnersDevices(t *testing.T) {
	appURL, projectURL, publishableKey, agentKey := livePlaneOrSkip(t)
	_, externalID := liveRequesterOrSkip(t)

	actorEmail := "live@example.com"
	directory := directoryServing(t, []adminUserMutation{
		{MemberID: "live-member", MattermostUserID: externalID, Email: actorEmail, Status: "active"},
	})
	blueclaw := blueclawServingRunsAndPolicy(t, nil)

	state := t.TempDir()
	service := NewService(Configuration{
		DatabasePath:               filepath.Join(state, "internkim.sqlite"),
		BlueclawBaseURL:            blueclaw.URL,
		APIBaseURL:                 directory.URL,
		FleetIDPath:                writeLiveFile(t, state, "fleet-id", "live-fleet"),
		FleetSecretPath:            writeLiveFile(t, state, "fleet-secret", "live-secret"),
		CentralPlaneAppURL:         appURL,
		CentralPlaneProjectURL:     projectURL,
		CentralPlanePublishableKey: publishableKey,
		CentralPlaneAgentKeyPath:   writeLiveFile(t, state, "agent-key", agentKey),
	})

	ctx := context.Background()
	if errorValue := service.saveMailAccountRecord(ctx, mail.Account{
		ActorEmail:     actorEmail,
		Email:          actorEmail,
		IMAPHost:       "imap.example.test",
		IMAPPort:       993,
		DefaultMailbox: "INBOX",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	inbox := &fakeMailBackend{messages: []mail.MessageResponse{
		{UID: 101, Subject: "지난주에 온 메일", From: "이샘플 <lee@example.com>"},
	}}
	service.mailBackend = inbox

	service.announceMailOnce(ctx)
	adopted, marked, errorValue := service.readMailNotifyMark(ctx, actorEmail)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !marked || adopted != 101 {
		t.Fatalf("the first look did not adopt the inbox: seenUpTo=%d marked=%v", adopted, marked)
	}

	inbox.messages = append(inbox.messages, mail.MessageResponse{
		UID: 102, Subject: "8월 정산서 확인 부탁드립니다", From: "박예시 <sample@example.com>",
	})
	service.announceMailOnce(ctx)
	service.announceMailOnce(ctx)

	seenUpTo, marked, errorValue := service.readMailNotifyMark(ctx, actorEmail)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !marked || seenUpTo != 102 {
		t.Fatalf("the arriving mail was not marked as told: seenUpTo=%d marked=%v", seenUpTo, marked)
	}
}
