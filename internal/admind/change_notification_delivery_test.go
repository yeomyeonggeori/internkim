package admind

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func newTwoRecipientFlowService(t *testing.T, failSecondRecipientOnce *bool, postedChannels *[]string) *Service {
	t.Helper()
	service := NewService(Configuration{
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-pass"),
		MattermostBotTokenPath:      writeTestFile(t, "bot-token"),
		FlowDatabasePath:            filepath.Join(t.TempDir(), "flow.sqlite"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.Path == "/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.Path == "/api/v4/users/me":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case strings.HasPrefix(request.URL.Path, "/api/v4/users/username/"):
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users?per_page=200":
			return jsonResponse(http.StatusOK, `[{"id":"user-owner","username":"owner","nickname":"김민수","email":"owner@example.com"},{"id":"user-mate","username":"mate","nickname":"박예시","email":"mate@example.com"}]`, nil), nil
		case request.URL.Path == "/api/v4/channels/direct" && request.Method == http.MethodPost:
			recipientID := mattermostDirectChannelOtherUserID(t, request, "bot-1")
			return jsonResponse(http.StatusCreated, `{"id":"direct-`+recipientID+`"}`, nil), nil
		case strings.HasSuffix(request.URL.Path, "/preferences"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.Path == "/api/v4/posts" && request.Method == http.MethodPost:
			channelID := mattermostPostChannelID(t, request)
			if channelID == "direct-user-mate" && *failSecondRecipientOnce {
				*failSecondRecipientOnce = false
				return jsonResponse(http.StatusServiceUnavailable, `{}`, nil), nil
			}
			*postedChannels = append(*postedChannels, channelID)
			return jsonResponse(http.StatusCreated, `{"id":"post-1"}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostExistingFlowSetupResponse(t, request), nil
		default:
			if response, handled := companyPlumbingAnswerForTest(request); handled {
				return response, nil
			}
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	return service
}

func TestFlowNoticeRetryDoesNotRepeatDeliveredRecipients(t *testing.T) {
	failSecondRecipientOnce := true
	postedChannels := []string{}
	service := newTwoRecipientFlowService(t, &failSecondRecipientOnce, &postedChannels)
	task := flowNotificationTestTask("요청")
	task.ParticipantNames = []string{"김민수", "박예시"}
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	service.applyFlowMattermostProjection(context.Background(), task)
	if strings.Join(postedChannels, "|") != "direct-user-owner" {
		t.Fatalf("first attempt delivered %+v", postedChannels)
	}
	assertFlowProjectionOutboxCount(t, service, 1)

	service.drainFlowMattermostProjectionOutbox(context.Background())

	if strings.Join(postedChannels, "|") != "direct-user-owner|direct-user-mate" {
		t.Fatalf("retry must reach only the recipient that missed the notice, got %+v", postedChannels)
	}
	assertFlowProjectionOutboxCount(t, service, 0)
}

func TestChangeNoticeKeepsDetailWhenWordingIsUnavailable(t *testing.T) {
	service := NewService(Configuration{})
	detail := "| 일시 | 일정 |\n|---|---|\n| 9월 2일 | 촬영 |"

	message := service.changeNoticeWording(context.Background(), []string{"일정: 촬영"}, detail)

	if message != detail {
		t.Fatalf("a notice must survive a failed wording call, got %q", message)
	}
}

func TestChangeNoticeSkipsWordingWithoutFacts(t *testing.T) {
	service := NewService(Configuration{})
	detail := "| 일시 | 일정 |"

	if message := service.changeNoticeWording(context.Background(), []string{"", "  "}, detail); message != detail {
		t.Fatalf("empty facts must not reach the model, got %q", message)
	}
}
