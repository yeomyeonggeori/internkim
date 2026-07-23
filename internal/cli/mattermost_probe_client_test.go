package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestMattermostProbeClientLifecycle(t *testing.T) {
	requests := make([]string, 0, 10)
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.EscapedPath()+querySuffix(request))
		if request.URL.Path != "/api/v4/users/login" && request.Header.Get("Authorization") != "Bearer probe-token" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		switch request.Method + " " + request.URL.EscapedPath() {
		case "POST /api/v4/users/login":
			assertJSONDocument(t, request, `{"login_id":"admin","password":"secret"}`)
			responseWriter.Header().Set("Token", "probe-token")
			writeJSONDocument(t, responseWriter, http.StatusOK, `{"id":"admin-id"}`)
		case "GET /api/v4/users/me":
			writeJSONDocument(t, responseWriter, http.StatusOK, `{"id":"admin-id","username":"admin","email":"admin@example.com","is_bot":false}`)
		case "POST /api/v4/users":
			assertJSONDocument(t, request, `{"username":"probe","email":"probe@example.com","password":"probe-password","first_name":"Mattermost","last_name":"Probe"}`)
			writeJSONDocument(t, responseWriter, http.StatusCreated, `{"id":"probe/id","username":"probe","email":"probe@example.com","is_bot":false}`)
		case "GET /api/v4/teams/name/internkim":
			writeJSONDocument(t, responseWriter, http.StatusOK, `{"id":"team/id"}`)
		case "POST /api/v4/teams/team%2Fid/members":
			assertJSONDocument(t, request, `{"team_id":"team/id","user_id":"probe/id"}`)
			writeJSONDocument(t, responseWriter, http.StatusCreated, `{}`)
		case "POST /api/v4/channels/direct":
			assertJSONDocument(t, request, `["probe/id","bot/id"]`)
			writeJSONDocument(t, responseWriter, http.StatusCreated, `{"id":"channel/id"}`)
		case "POST /api/v4/posts":
			assertJSONDocument(t, request, `{"channel_id":"channel/id","root_id":"root/id","message":"업무 요청입니다"}`)
			writeJSONDocument(t, responseWriter, http.StatusCreated, `{"id":"post/id","root_id":"root/id","user_id":"probe/id","message":"업무 요청입니다","file_ids":["file/id"],"create_at":1700000000000}`)
		case "GET /api/v4/channels/channel%2Fid/posts":
			writeJSONDocument(t, responseWriter, http.StatusOK, `{"order":["reply/id","post/id"],"posts":{"reply/id":{"id":"reply/id","root_id":"root/id","user_id":"bot/id","message":"완료했습니다","file_ids":null,"create_at":1700000002000},"post/id":{"id":"post/id","root_id":"root/id","user_id":"probe/id","message":"업무 요청입니다","file_ids":["file/id"],"create_at":1700000000000}}}`)
		case "GET /api/v4/files/file%2Fid/info":
			writeJSONDocument(t, responseWriter, http.StatusOK, `{"id":"file/id","name":"결과.docx","mime_type":"application/vnd.openxmlformats-officedocument.wordprocessingml.document","size":7}`)
		case "GET /api/v4/files/file%2Fid":
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte("content"))
		case "DELETE /api/v4/posts/post%2Fid":
			responseWriter.WriteHeader(http.StatusOK)
		case "DELETE /api/v4/users/probe%2Fid":
			if request.URL.Query().Get("permanent") != "true" {
				t.Fatalf("permanent = %q", request.URL.Query().Get("permanent"))
			}
			responseWriter.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	client, errorValue := newMattermostProbeClient(server.URL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if client.httpClient.Timeout != 0 {
		t.Fatalf("HTTP timeout = %v", client.httpClient.Timeout)
	}
	contextValue := context.Background()
	token, errorValue := client.Login(contextValue, "admin", "secret")
	if errorValue != nil || token != "probe-token" {
		t.Fatalf("login token = %q, error = %v", token, errorValue)
	}
	currentUser, errorValue := client.CurrentUser(contextValue, token)
	if errorValue != nil || currentUser.ID != "admin-id" {
		t.Fatalf("current user = %+v, error = %v", currentUser, errorValue)
	}
	createdUser, errorValue := client.CreateUser(contextValue, token, mattermostProbeNewUser{Username: "probe", Email: "probe@example.com", Password: "probe-password", FirstName: "Mattermost", LastName: "Probe"})
	if errorValue != nil || createdUser.ID != "probe/id" {
		t.Fatalf("created user = %+v, error = %v", createdUser, errorValue)
	}
	team, errorValue := client.TeamByName(contextValue, token, "internkim")
	if errorValue != nil || team.ID != "team/id" {
		t.Fatalf("team = %+v, error = %v", team, errorValue)
	}
	if errorValue := client.AddTeamMember(contextValue, token, team.ID, createdUser.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	channel, errorValue := client.CreateDirectChannel(contextValue, token, createdUser.ID, "bot/id")
	if errorValue != nil || channel.ID != "channel/id" {
		t.Fatalf("channel = %+v, error = %v", channel, errorValue)
	}
	post, errorValue := client.PostMessage(contextValue, token, mattermostProbeMessage{ChannelID: channel.ID, RootID: "root/id", Message: "업무 요청입니다"})
	if errorValue != nil || post.RootID != "root/id" || post.CreatedAt != 1700000000000 || !reflect.DeepEqual(post.FileIDs, []string{"file/id"}) {
		t.Fatalf("post = %+v, error = %v", post, errorValue)
	}
	posts, errorValue := client.ListChannelPosts(contextValue, token, channel.ID)
	if errorValue != nil || len(posts) != 2 || posts[0].ID != "reply/id" || posts[0].FileIDs == nil {
		t.Fatalf("posts = %+v, error = %v", posts, errorValue)
	}
	metadata, errorValue := client.FileMetadata(contextValue, token, "file/id")
	if errorValue != nil || metadata.Name != "결과.docx" || metadata.Size != 7 {
		t.Fatalf("metadata = %+v, error = %v", metadata, errorValue)
	}
	document, errorValue := client.DownloadFile(contextValue, token, "file/id")
	if errorValue != nil || string(document) != "content" {
		t.Fatalf("document = %q, error = %v", document, errorValue)
	}
	if errorValue := client.DeletePost(contextValue, token, post.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := client.DeleteUser(contextValue, token, createdUser.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedRequests := []string{
		"POST /api/v4/users/login",
		"GET /api/v4/users/me",
		"POST /api/v4/users",
		"GET /api/v4/teams/name/internkim",
		"POST /api/v4/teams/team%2Fid/members",
		"POST /api/v4/channels/direct",
		"POST /api/v4/posts",
		"GET /api/v4/channels/channel%2Fid/posts?per_page=200",
		"GET /api/v4/files/file%2Fid/info",
		"GET /api/v4/files/file%2Fid",
		"DELETE /api/v4/posts/post%2Fid",
		"DELETE /api/v4/users/probe%2Fid?permanent=true",
	}
	if !reflect.DeepEqual(requests, expectedRequests) {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestMattermostProbeClientDeactivatesUserWhenPermanentDeletionRejectsDirectChannel(t *testing.T) {
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		if request.URL.Query().Get("permanent") == "true" {
			writeJSONDocument(t, responseWriter, http.StatusBadRequest, `{"message":"Cannot delete a direct message channel"}`)
			return
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client, errorValue := newMattermostProbeClient(server.URL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := client.DeleteUser(context.Background(), "token", "probe/id"); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedRequests := []string{
		"DELETE /api/v4/users/probe%2Fid?permanent=true",
		"DELETE /api/v4/users/probe%2Fid",
	}
	if !reflect.DeepEqual(requests, expectedRequests) {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestMattermostProbeClientTreatsMissingPostAsDeleted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		writeJSONDocument(t, responseWriter, http.StatusNotFound, `{"message":"post not found"}`)
	}))
	defer server.Close()
	client, errorValue := newMattermostProbeClient(server.URL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := client.DeletePost(context.Background(), "token", "missing"); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestMattermostProbeClientReportsBothUserDeletionFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		writeJSONDocument(t, responseWriter, http.StatusForbidden, `{"message":"deletion denied"}`)
	}))
	defer server.Close()
	client, errorValue := newMattermostProbeClient(server.URL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	errorValue = client.DeleteUser(context.Background(), "token", "probe")
	if errorValue == nil || strings.Count(errorValue.Error(), "returned HTTP 403") != 2 {
		t.Fatalf("error = %v", errorValue)
	}
}

func TestMattermostProbeClientReportsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		writeJSONDocument(t, responseWriter, http.StatusUnauthorized, `{"message":"invalid credentials"}`)
	}))
	defer server.Close()
	client, errorValue := newMattermostProbeClient(server.URL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = client.Login(context.Background(), "admin", "wrong")
	var httpError mattermostProbeHTTPError
	if !errors.As(errorValue, &httpError) {
		t.Fatalf("error = %T %v", errorValue, errorValue)
	}
	if httpError.StatusCode != http.StatusUnauthorized || !strings.Contains(httpError.Body, "invalid credentials") {
		t.Fatalf("HTTP error = %+v", httpError)
	}
}

func TestMattermostProbeClientPropagatesCancellation(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		close(requestStarted)
		<-request.Context().Done()
	}))
	defer server.Close()
	client, errorValue := newMattermostProbeClient(server.URL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	contextValue, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, requestError := client.CurrentUser(contextValue, "token")
		result <- requestError
	}()
	<-requestStarted
	cancel()
	if errorValue := <-result; !errors.Is(errorValue, context.Canceled) {
		t.Fatalf("error = %v", errorValue)
	}
}

func TestMattermostProbeClientValidatesResponses(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		errorText string
	}{
		{name: "missing user identity", response: `{"id":"","username":""}`, errorText: "no ID or username"},
		{name: "trailing JSON", response: `{"id":"user","username":"probe"}{}`, errorText: "invalid character"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
				writeJSONDocument(t, responseWriter, http.StatusOK, test.response)
			}))
			defer server.Close()
			client, errorValue := newMattermostProbeClient(server.URL)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			_, errorValue = client.CurrentUser(context.Background(), "token")
			if errorValue == nil || !strings.Contains(errorValue.Error(), test.errorText) {
				t.Fatalf("error = %v", errorValue)
			}
		})
	}
}

func TestMattermostProbeClientGetPostParsesApprovalActions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.EscapedPath() != "/api/v4/posts/post%2Fid" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		}
		writeJSONDocument(t, responseWriter, http.StatusOK, `{"id":"post/id","root_id":"root/id","user_id":"bot/id","message":"승인해 주세요","props":{"attachments":[{"actions":[{"id":"askConfirm","name":"확인","type":"button"},{"id":"askCancel","name":"취소","type":"button"}]}]}}`)
	}))
	defer server.Close()
	client, errorValue := newMattermostProbeClient(server.URL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	post, errorValue := client.GetPost(context.Background(), "token", "post/id")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(post.Props.Attachments) != 1 || len(post.Props.Attachments[0].Actions) != 2 {
		t.Fatalf("unexpected post props: %+v", post.Props)
	}
	actionID, hasApprovalAction := mattermostProbeApprovalActionID(post)
	if !hasApprovalAction || actionID != "askConfirm" {
		t.Fatalf("actionID = %q, hasApprovalAction = %t", actionID, hasApprovalAction)
	}
}

func TestMattermostProbeClientDoPostActionSendsActionRequest(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		requestedPath = request.Method + " " + request.URL.EscapedPath()
		if request.Header.Get("Authorization") != "Bearer user-token" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client, errorValue := newMattermostProbeClient(server.URL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := client.DoPostAction(context.Background(), "user-token", "post/id", "askConfirm"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if requestedPath != "POST /api/v4/posts/post%2Fid/actions/askConfirm" {
		t.Fatalf("unexpected request path: %q", requestedPath)
	}
}

func TestMattermostProbeApprovalActionIDMatchesNameWhenIDDiffers(t *testing.T) {
	post := mattermostProbePost{Props: mattermostProbePostProps{Attachments: []mattermostProbePostAttachment{{
		Actions: []mattermostProbePostAction{{ID: "action-1", Name: "Approve"}, {ID: "action-2", Name: "Cancel"}},
	}}}}
	actionID, hasApprovalAction := mattermostProbeApprovalActionID(post)
	if !hasApprovalAction || actionID != "action-1" {
		t.Fatalf("actionID = %q, hasApprovalAction = %t", actionID, hasApprovalAction)
	}
}

func TestMattermostProbeApprovalActionIDReportsMissingApproval(t *testing.T) {
	post := mattermostProbePost{Props: mattermostProbePostProps{Attachments: []mattermostProbePostAttachment{{
		Actions: []mattermostProbePostAction{{ID: "askChoiceA", Name: "옵션 A"}},
	}}}}
	if _, hasApprovalAction := mattermostProbeApprovalActionID(post); hasApprovalAction {
		t.Fatal("expected no approval action to be found")
	}
}

func querySuffix(request *http.Request) string {
	if request.URL.RawQuery == "" {
		return ""
	}
	return "?" + request.URL.RawQuery
}

func assertJSONDocument(t *testing.T, request *http.Request, expectedDocument string) {
	t.Helper()
	document, errorValue := io.ReadAll(request.Body)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	actualDocument := new(bytes.Buffer)
	if errorValue := json.Compact(actualDocument, document); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedCompactDocument := new(bytes.Buffer)
	if errorValue := json.Compact(expectedCompactDocument, []byte(expectedDocument)); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !bytes.Equal(actualDocument.Bytes(), expectedCompactDocument.Bytes()) {
		t.Fatalf("JSON document = %s", document)
	}
}

func writeJSONDocument(t *testing.T, responseWriter http.ResponseWriter, statusCode int, document string) {
	t.Helper()
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	if _, errorValue := responseWriter.Write([]byte(document)); errorValue != nil {
		t.Fatal(errorValue)
	}
}
