package cli

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestPrintPublicStatusShowsMattermostWhenSSHIsUnavailable(t *testing.T) {
	originalStatusHTTPClient := statusHTTPClient
	defer func() { statusHTTPClient = originalStatusHTTPClient }()
	requestPaths := []string{}
	statusHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestPaths = append(requestPaths, request.URL.Path)
		switch request.URL.Path {
		case "/api/v4/system/ping":
			return textHTTPResponse(http.StatusOK, `{"status":"OK"}`), nil
		case "/admin/api/health":
			return textHTTPResponse(http.StatusFound, ""), nil
		default:
			t.Fatalf("unexpected request path %s", request.URL.Path)
			return nil, nil
		}
	})}

	output := captureStandardOutput(t, func() {
		printPublicStatusForCommandTarget(newMsg("ko"), commandTarget{
			boardType: setupBoardForStatusTest,
			mode:      commandTargetModePhysical,
			deviceURL: "https://device.example",
		})
	})

	for _, expectedPath := range []string{"/api/v4/system/ping", "/admin/api/health"} {
		if !containsString(requestPaths, expectedPath) {
			t.Fatalf("expected request path %s, got %+v", expectedPath, requestPaths)
		}
	}
	for _, expectedText := range []string{"Mattermost 공개 URL", "✓ HTTP 200", "Admin 공개 URL", "HTTP 302 redirect"} {
		if !strings.Contains(output, expectedText) {
			t.Fatalf("expected output to contain %q, got %s", expectedText, output)
		}
	}
}

func textHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func captureStandardOutput(t *testing.T, run func()) string {
	t.Helper()
	originalStandardOutput := os.Stdout
	reader, writer, errorValue := os.Pipe()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	os.Stdout = writer
	run()
	writer.Close()
	os.Stdout = originalStandardOutput
	var output bytes.Buffer
	if _, errorValue := io.Copy(&output, reader); errorValue != nil {
		t.Fatal(errorValue)
	}
	return output.String()
}

func containsString(values []string, expectedValue string) bool {
	for _, value := range values {
		if value == expectedValue {
			return true
		}
	}
	return false
}

const setupBoardForStatusTest = "jetson-orin-nano"

func TestRemoteSSHErrorNamesTheHostAndNoTransport(t *testing.T) {
	errorValue := remoteSSHError("0.ssh.example.com", "Connection timed out during banner exchange", os.ErrDeadlineExceeded)

	errorMessage := errorValue.Error()
	for _, expectedText := range []string{"0.ssh.example.com", "Connection timed out during banner exchange"} {
		if !strings.Contains(errorMessage, expectedText) {
			t.Fatalf("expected error to contain %q, got %s", expectedText, errorMessage)
		}
	}
	if strings.Contains(errorMessage, "cloudflared") {
		t.Fatalf("how the operator reaches the host is their configuration, got %s", errorMessage)
	}
}
