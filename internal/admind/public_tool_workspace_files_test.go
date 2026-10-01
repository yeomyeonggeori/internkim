package admind

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

func newPublicToolWorkspaceTestService(t *testing.T, workspaceFiles map[string]string, downloads *[]url.Values) *Service {
	t.Helper()
	service := NewService(Configuration{
		StateDirectory:        filepath.Join(t.TempDir(), "state"),
		APIBaseURL:            "https://api.example.test",
		AdminEmailPath:        writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath: writeTestFile(t, "admin@example.com"),
		FleetIDPath:           writeTestFile(t, "device-1"),
		FleetSecretPath:       writeTestFile(t, "secret-1"),
		BlueclawBaseURL:       "http://blueclaw.local",
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	directory := companyDirectoryHolding(centralplane.Member{MemberID: "person-member", Email: "member@example.com", Name: "Member", Role: "member", Status: "active"})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return directory.respond(t, request)
		}
		if request.URL.Path == "/admin/api/workspace/download" {
			*downloads = append(*downloads, request.URL.Query())
			content, isHeld := workspaceFiles[request.URL.Query().Get("path")]
			if !isHeld {
				return jsonResponse(http.StatusForbidden, "workspace path is not accessible", nil), nil
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(content)), Header: http.Header{}}, nil
		}
		if request.URL.Path == "/admin/api/policy" {
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	return service
}

func invokePublicToolForTest(t *testing.T, service *Service, toolName string, input string) *httptest.ResponseRecorder {
	t.Helper()
	token, _, errorValue := service.issuePublicAPIToken(context.Background(), "member@example.com", publicAPITokenCreateRequest{Scopes: []string{publicAPIPermissionWrite}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tools/"+toolName+"/invoke", strings.NewReader(`{"input":`+input+`}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	service.handlePublicAPI(response, request)
	return response
}

func TestPublicAPIMessageSendCarriesTheFileItNames(t *testing.T) {
	agentPath := "/workspace/private/people/person-member/inbox/api/mascot.png"
	downloads := []url.Values{}
	service := newPublicToolWorkspaceTestService(t, map[string]string{agentPath: "a picture"}, &downloads)
	var carried []capabilities.WorkspaceFile
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		carried = request.Transport.WorkspaceFiles
		return capabilities.ToolInvokeResponse{ToolName: request.ToolName, Status: "sent", Result: json.RawMessage(`{"ok":true}`)}
	})

	response := invokePublicToolForTest(t, service, "message_send", `{"targetType":"channel","channelName":"잡담","message":"보냅니다","attachments":["`+agentPath+`"]}`)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if len(carried) != 1 {
		t.Fatalf("the transport carried %d files, so capabilityd answers attachment_not_carried", len(carried))
	}
	digest := sha256.Sum256([]byte("a picture"))
	if carried[0].WorkspacePath != agentPath || carried[0].Filename != "mascot.png" {
		t.Fatalf("carried file = %+v", carried[0])
	}
	if carried[0].ContentBase64 != base64.StdEncoding.EncodeToString([]byte("a picture")) {
		t.Fatalf("carried content = %q", carried[0].ContentBase64)
	}
	if carried[0].SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("carried digest = %q", carried[0].SHA256)
	}
	if len(downloads) != 1 || downloads[0].Get("personID") != "person-member" {
		t.Fatalf("the file was read as %+v, and it must be read as the person who asked", downloads)
	}
}

func TestPublicAPICarriesNothingForACallNamingNoFile(t *testing.T) {
	downloads := []url.Values{}
	service := newPublicToolWorkspaceTestService(t, map[string]string{}, &downloads)
	carried := []capabilities.WorkspaceFile{{WorkspacePath: "never"}}
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		carried = request.Transport.WorkspaceFiles
		return capabilities.ToolInvokeResponse{ToolName: request.ToolName, Status: "sent", Result: json.RawMessage(`{"ok":true}`)}
	})

	response := invokePublicToolForTest(t, service, "message_send", `{"targetType":"channel","channelName":"잡담","message":"보냅니다"}`)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if len(carried) != 0 {
		t.Fatalf("a call naming no file carried %+v", carried)
	}
	if len(downloads) != 0 {
		t.Fatalf("a call naming no file read %+v", downloads)
	}
}

func TestPublicAPIRefusesAFileThePersonMayNotRead(t *testing.T) {
	downloads := []url.Values{}
	service := newPublicToolWorkspaceTestService(t, map[string]string{}, &downloads)
	service.Configuration.CapabilitySocketPath = startPublicToolGatewayCapabilityServer(t, func(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
		t.Fatalf("capabilityd was asked to send a file nobody could read: %+v", request)
		return capabilities.ToolInvokeResponse{}
	})

	response := invokePublicToolForTest(t, service, "message_send", `{"targetType":"channel","channelName":"잡담","message":"보냅니다","attachments":["/workspace/private/people/person-other/secret.png"]}`)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if len(downloads) != 1 {
		t.Fatalf("the refusal came from %d reads", len(downloads))
	}
}
