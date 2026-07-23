package admind

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestEnsureMattermostEphemeralPluginUploadsEnablesAndPatchesSecret(t *testing.T) {
	stateDirectory := t.TempDir()
	bundlePath := filepath.Join(stateDirectory, "com.internkim.ephemeral-0.1.0.tar.gz")
	writeFile(t, bundlePath, "plugin-bundle")
	secretValues := make(chan string, 1)
	uploadedPlugin := false
	enabledPlugin := false
	patchedSecret := false
	service := NewService(Configuration{
		StateDirectory:              stateDirectory,
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-password"),
		MattermostPluginBundlePath:  bundlePath,
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/plugins?force=true" && request.Method == http.MethodPost:
			assertMattermostBearerToken(t, request, "admin-token")
			assertMattermostPluginUpload(t, request, "plugin-bundle")
			uploadedPlugin = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/plugins/com.internkim.ephemeral/enable" && request.Method == http.MethodPost:
			assertMattermostBearerToken(t, request, "admin-token")
			enabledPlugin = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config" && request.Method == http.MethodGet:
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"PluginSettings":{"Enable":true,"EnableUploads":true}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertMattermostBearerToken(t, request, "admin-token")
			settings := mattermostEphemeralPluginPatchSettings(t, request)
			if settings["runtimeHealthURL"] != blueclaw.BlueclawHealthCheckURL() {
				t.Fatalf("runtime health URL = %q", settings["runtimeHealthURL"])
			}
			if settings["botUsername"] != "internkim" {
				t.Fatalf("bot username = %q", settings["botUsername"])
			}
			secretValues <- settings["secret"]
			patchedSecret = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/plugins" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"active":[{"id":"com.internkim.ephemeral","version":"0.1.0"}],"inactive":[]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	secret, errorValue := service.ensureMattermostEphemeralPlugin(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if secret == "" || strings.TrimSpace(readTrimmedFile(service.mattermostEphemeralPluginSecretPath())) != secret {
		t.Fatalf("secret was not persisted")
	}
	if !uploadedPlugin || !enabledPlugin || !patchedSecret {
		t.Fatalf("plugin ensure flags uploaded=%v enabled=%v patched=%v", uploadedPlugin, enabledPlugin, patchedSecret)
	}
	if patchedSecretValue := <-secretValues; patchedSecretValue != secret {
		t.Fatalf("patched secret = %q, want %q", patchedSecretValue, secret)
	}
}

func TestEnableMattermostPluginUploadsUsesAPI(t *testing.T) {
	service := NewService(Configuration{MattermostBaseURL: "http://mattermost.local"})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"PluginSettings":{"Enable":false,"EnableUploads":false}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			var payload struct {
				PluginSettings struct {
					Enable                 bool `json:"Enable"`
					EnableUploads          bool `json:"EnableUploads"`
					RequirePluginSignature bool `json:"RequirePluginSignature"`
				} `json:"PluginSettings"`
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if !payload.PluginSettings.Enable || !payload.PluginSettings.EnableUploads || payload.PluginSettings.RequirePluginSignature {
				t.Fatalf("plugin settings = %+v", payload.PluginSettings)
			}
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	if errorValue := service.enableMattermostPluginUploads(context.Background(), "admin-token"); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func assertMattermostPluginUpload(t *testing.T, request *http.Request, expectedDocument string) {
	t.Helper()
	reader, errorValue := request.MultipartReader()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for {
		part, errorValue := reader.NextPart()
		if errorValue != nil {
			break
		}
		if part.FormName() != "plugin" {
			continue
		}
		document := readMultipartPart(t, part)
		if document != expectedDocument {
			t.Fatalf("plugin upload document = %q", document)
		}
		return
	}
	t.Fatal("plugin upload part missing")
}

func readMultipartPart(t *testing.T, part *multipart.Part) string {
	t.Helper()
	document, errorValue := io.ReadAll(part)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}

func mattermostEphemeralPluginPatchSettings(t *testing.T, request *http.Request) map[string]string {
	t.Helper()
	var payload struct {
		PluginSettings struct {
			Plugins map[string]map[string]string `json:"Plugins"`
		} `json:"PluginSettings"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	pluginSettings := payload.PluginSettings.Plugins[mattermostEphemeralPluginID]
	if pluginSettings == nil {
		t.Fatalf("missing plugin settings: %+v", payload)
	}
	return pluginSettings
}
