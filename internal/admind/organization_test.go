package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestDirectoryUsersResponseBodyFallsBackWhenOrganizationMetadataFails(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.Configuration.StateDirectory = writeTestFile(t, "not a directory")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy" {
			return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	responseBody, errorValue := service.directoryUsersResponseBody(context.Background(), pagesUsersResponse{Records: []adminUserMutation{{
		MemberID: "user-member",
		Email:    "member@example.com",
		Name:     "Member User",
		Role:     "member",
	}}})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var response pagesUsersResponse
	if errorValue := json.Unmarshal(responseBody, &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.Records) != 1 || response.Records[0].Email != "member@example.com" {
		t.Fatalf("records = %#v; want original response", response.Records)
	}
}

func localUsersPolicyDocumentWithOrgGroups() string {
	return `{
		"people":[
			{"personID":"user-member","displayName":"Member User","emails":["member@example.com"],"circles":["member"],"isAdmin":false}
		],
		"circles":[{"circleID":"member","displayName":"Member"}],
		"orgGroups":[{"id":"legacy","name":"Legacy"}],
		"circleSync":{"mattermostPrivateChannels":[{"circleID":"member","channelName":"circle-member"}]}
	}`
}
