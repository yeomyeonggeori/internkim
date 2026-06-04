package admind

import (
	"context"
	"net/http"
	"testing"
)

func TestEnsureMattermostChannelMembershipConfirmsExistingMembershipAfterBadRequest(t *testing.T) {
	requests := []string{}
	service := NewService(Configuration{MattermostBaseURL: "http://mattermost.local"})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/channel-1/members":
			return jsonResponse(http.StatusBadRequest, `{"message":"already a member"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/channels/channel-1/members/user-1":
			return jsonResponse(http.StatusOK, `{"user_id":"user-1"}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	if errorValue := service.ensureMattermostChannelMembership(context.Background(), "admin-token", "channel-1", "user-1"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %+v", requests)
	}
}

func TestEnsureMattermostChannelMembershipReturnsBadRequestWhenMembershipIsMissing(t *testing.T) {
	service := NewService(Configuration{MattermostBaseURL: "http://mattermost.local"})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/channel-1/members":
			return jsonResponse(http.StatusBadRequest, `{"message":"invalid membership"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/channels/channel-1/members/user-1":
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	if errorValue := service.ensureMattermostChannelMembership(context.Background(), "admin-token", "channel-1", "user-1"); errorValue == nil {
		t.Fatal("expected membership confirmation error")
	}
}
