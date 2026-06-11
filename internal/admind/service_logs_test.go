package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceLogsBlueclawWithoutFilterReturnsCatCommand(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com")})
	capturedName := ""
	capturedArguments := []string{}
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		capturedName = name
		capturedArguments = arguments
		return []byte("line one\nline two\nline three\n"), nil
	}

	request := httptest.NewRequest(http.MethodGet, "/admin/api/diagnostics/service-logs?service=blueclaw", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()

	service.handleAdmin(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if capturedName != "sh" {
		t.Fatalf("expected command 'sh', got %q", capturedName)
	}
	if len(capturedArguments) < 2 || capturedArguments[0] != "-c" {
		t.Fatalf("expected arguments [-c <cmd>], got %v", capturedArguments)
	}
	if !strings.Contains(capturedArguments[1], "cat") || !strings.Contains(capturedArguments[1], "tail") {
		t.Fatalf("expected cat+tail command, got %q", capturedArguments[1])
	}
	var logsResponse serviceLogsResponse
	if decodeError := json.NewDecoder(response.Body).Decode(&logsResponse); decodeError != nil {
		t.Fatalf("decode error: %v", decodeError)
	}
	if logsResponse.Service != "blueclaw" {
		t.Fatalf("expected service 'blueclaw', got %q", logsResponse.Service)
	}
	if logsResponse.Count != 3 {
		t.Fatalf("expected 3 lines, got %d", logsResponse.Count)
	}
}

func TestServiceLogsBlueclawWithTaskRunIDUsesGrep(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com")})
	capturedArguments := []string{}
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		capturedArguments = arguments
		return []byte("matching-run-abc line\n"), nil
	}

	request := httptest.NewRequest(http.MethodGet, "/admin/api/diagnostics/service-logs?service=blueclaw&taskRunID=matching-run-abc", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()

	service.handleAdmin(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if len(capturedArguments) < 2 || !strings.Contains(capturedArguments[1], "grep") {
		t.Fatalf("expected grep command, got %v", capturedArguments)
	}
	if !strings.Contains(capturedArguments[1], "matching-run-abc") {
		t.Fatalf("expected taskRunID in command, got %q", capturedArguments[1])
	}
	var logsResponse serviceLogsResponse
	if decodeError := json.NewDecoder(response.Body).Decode(&logsResponse); decodeError != nil {
		t.Fatalf("decode error: %v", decodeError)
	}
	if logsResponse.TaskRunID != "matching-run-abc" {
		t.Fatalf("expected taskRunID in response, got %q", logsResponse.TaskRunID)
	}
}

func TestServiceLogsRejectsIllegalTaskRunID(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com")})
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		t.Fatal("RunCommand should not be called for illegal taskRunID")
		return nil, nil
	}

	request := httptest.NewRequest(http.MethodGet, "/admin/api/diagnostics/service-logs?service=blueclaw&taskRunID=abc%27%3B+rm", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()

	service.handleAdmin(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for illegal taskRunID, got %d: %s", response.Code, response.Body.String())
	}
}

func TestServiceLogsJournalctlWithTaskRunIDFiltersInGo(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com")})
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		return []byte("2026-06-11T10:00:00+0900 line about target-run-id\n2026-06-11T10:00:01+0900 unrelated line\n2026-06-11T10:00:02+0900 also target-run-id here\n"), nil
	}

	request := httptest.NewRequest(http.MethodGet, "/admin/api/diagnostics/service-logs?service=admind&taskRunID=target-run-id", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()

	service.handleAdmin(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var logsResponse serviceLogsResponse
	if decodeError := json.NewDecoder(response.Body).Decode(&logsResponse); decodeError != nil {
		t.Fatalf("decode error: %v", decodeError)
	}
	if logsResponse.Count != 2 {
		t.Fatalf("expected 2 matching lines, got %d: %v", logsResponse.Count, logsResponse.Lines)
	}
	for _, line := range logsResponse.Lines {
		if !strings.Contains(line, "target-run-id") {
			t.Fatalf("returned line does not contain taskRunID: %q", line)
		}
	}
}

func TestServiceLogsRejectsUnknownService(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com")})
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		t.Fatal("RunCommand should not be called for unknown service")
		return nil, nil
	}

	request := httptest.NewRequest(http.MethodGet, "/admin/api/diagnostics/service-logs?service=unknown-daemon", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()

	service.handleAdmin(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown service, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "blueclaw") {
		t.Fatalf("expected error to list valid services, got %q", response.Body.String())
	}
}
