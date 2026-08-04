package cli

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

func writeCloudflareAccessToken(t *testing.T, hostname string) {
	t.Helper()
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	tokenDirectory := filepath.Join(homeDirectory, ".cloudflared")
	if errorValue := os.MkdirAll(tokenDirectory, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(tokenDirectory, hostname+"-abc123-token"), []byte("token"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestFormatCloudflareSSHErrorNamesTheMissingAccessToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	errorValue := formatCloudflareSSHError("0.ssh.example.com", "Connection timed out during banner exchange", os.ErrDeadlineExceeded)

	errorMessage := errorValue.Error()
	if !strings.Contains(errorMessage, "cloudflared access login https://0.ssh.example.com") {
		t.Fatalf("expected the missing token to be named, because without it cloudflared waits for a browser and the request never reaches the device, got %s", errorMessage)
	}
	if strings.Contains(errorMessage, "sshd") {
		t.Fatalf("expected no hint to inspect the device, because the device is not involved when no stream is ever opened, got %s", errorMessage)
	}
}

func TestFormatCloudflareSSHErrorIdentifiesMissingSSHBanner(t *testing.T) {
	writeCloudflareAccessToken(t, "0.ssh.example.com")

	errorValue := formatCloudflareSSHError("0.ssh.example.com", "Connection timed out during banner exchange", os.ErrDeadlineExceeded)
	errorMessage := errorValue.Error()
	for _, expectedText := range []string{
		"0.ssh.example.com",
		"Connection timed out during banner exchange",
		"SSH banner",
		"sshd",
		"cloudflared-node-ssh",
		"./internkim recover ssh",
	} {
		if !strings.Contains(errorMessage, expectedText) {
			t.Fatalf("expected error to contain %q, got %s", expectedText, errorMessage)
		}
	}
}

func TestCloudflareSSHFailureClassDistinguishesBannerTimeout(t *testing.T) {
	failureClass := cloudflareSSHFailureClass("Connection timed out during banner exchange")
	if failureClass != "origin_banner_timeout" {
		t.Fatalf("failure class = %q, expected origin_banner_timeout", failureClass)
	}
	summary := cloudflareSSHFailureSummary(os.ErrDeadlineExceeded)
	if !strings.Contains(summary, "unknown") {
		t.Fatalf("generic summary should not invent a recovery class, got %s", summary)
	}
}

func TestFormatCloudflareSSHErrorKeepsAccessLoginHintForGenericFailures(t *testing.T) {
	writeCloudflareAccessToken(t, "0.ssh.example.com")

	errorValue := formatCloudflareSSHError("0.ssh.example.com", "access token expired", os.ErrPermission)
	errorMessage := errorValue.Error()
	for _, expectedText := range []string{
		"0.ssh.example.com",
		"cloudflared access ssh --hostname 0.ssh.example.com",
	} {
		if !strings.Contains(errorMessage, expectedText) {
			t.Fatalf("expected error to contain %q, got %s", expectedText, errorMessage)
		}
	}
}

const setupBoardForStatusTest = "jetson-orin-nano"

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
