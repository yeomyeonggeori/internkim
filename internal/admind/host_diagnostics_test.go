package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func TestHostDiagnosticViewsMatchThePublishedContract(t *testing.T) {
	for _, descriptor := range capabilities.DefaultToolDescriptors() {
		if descriptor.CanonicalName != "host_diagnostics_get" {
			continue
		}
		var schema struct {
			Properties struct {
				View struct {
					Enum []string `json:"enum"`
				} `json:"view"`
			} `json:"properties"`
		}
		if errorValue := json.Unmarshal(descriptor.InputSchema, &schema); errorValue != nil {
			t.Fatal(errorValue)
		}
		readers := NewService(Configuration{}).hostDiagnosticReaders("admin@example.com")
		if len(readers) != len(schema.Properties.View.Enum) {
			t.Fatalf("diagnostics views differ: %v, %v", readers, schema.Properties.View.Enum)
		}
		for view := range readers {
			if !slices.Contains(schema.Properties.View.Enum, view) {
				t.Fatalf("unpublished diagnostic view %q", view)
			}
		}
		return
	}
	t.Fatal("host diagnostics is missing from the published catalog")
}

func TestPublicDiagnosticsReuseTheWebPageReaderAndTrustedAdminIdentity(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	previousTransport := service.HTTPClient.Transport
	service.HTTPClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/admin/api/run/detail" {
			return previousTransport.RoundTrip(request)
		}
		query := request.URL.Query()
		if query.Get("taskRunID") != "sample-run" || query.Get("viewerEmail") != "admin@example.com" || query.Get("viewerIsAdmin") != "true" {
			t.Fatalf("unexpected scoped detail query: %v", query)
		}
		return jsonResponse(http.StatusOK, `{"taskRun":{"taskRunID":"sample-run","status":"failed"},"taskEvents":[{"name":"tool.failed","body":"process exited"}]}`, nil), nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/run?taskRunID=sample-run&viewerEmail=other@example.com&viewerIsAdmin=false", nil)
	request.Header.Set(requesterEmailHeader, "admin@example.com")
	request.Header.Set(requesterPermissionHeader, publicAPIPermissionRead)
	publicResponse := httptest.NewRecorder()
	service.handlePublicAPI(publicResponse, arrivingOnTheRequesterSocket(request))
	webResponse := httptest.NewRecorder()
	service.proxyScopedTaskDetail(webResponse, request, "admin@example.com", true)
	if publicResponse.Code != http.StatusOK || !reflect.DeepEqual(publicResponse.Body.Bytes(), webResponse.Body.Bytes()) {
		t.Fatalf("public diagnostics differs from the page: %d %s", publicResponse.Code, publicResponse.Body.String())
	}
}

func TestPublicDiagnosticsRejectMembersAndUntrustedRequesterHeaders(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		email      string
		isTrusted  bool
		statusCode int
	}{
		{"member", "member@example.com", true, http.StatusForbidden},
		{"spoofed administrator", "admin@example.com", false, http.StatusUnauthorized},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service := newTaskAuthorizationTestService(t)
			service.RunCommand = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				t.Fatal("a refused caller reached the service logs")
				return nil, nil
			}
			request := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/service_logs?service=admind", nil)
			request.Header.Set(requesterEmailHeader, testCase.email)
			request.Header.Set(requesterPermissionHeader, publicAPIPermissionRead)
			if testCase.isTrusted {
				request = arrivingOnTheRequesterSocket(request)
			}
			response := httptest.NewRecorder()
			service.handlePublicAPI(response, request)
			if response.Code != testCase.statusCode || strings.Contains(response.Body.String(), "lines") {
				t.Fatalf("refused caller got %d %s", response.Code, response.Body.String())
			}
		})
	}
}
