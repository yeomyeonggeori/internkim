package capabilityd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const companyBrowserRequesterEmail = "sample@example.com"

const signupPage = `<!doctype html>
<html><head><title>Signup</title></head><body>
<form onsubmit="event.preventDefault(); var received = document.createElement('button'); received.id = 'received'; received.textContent = 'received ' + document.getElementById('name').value + ' ' + document.getElementById('team').value; document.body.appendChild(received);">
<label>Name <input id="name"></label>
<label>Team <select id="team"><option value="sales">Sales</option><option value="support">Support</option></select></label>
<button type="submit">Send</button>
</form></body></html>`

type headlessBrowser struct {
	command *exec.Cmd
	exited  chan struct{}
}

func (browser *headlessBrowser) Exited() <-chan struct{} {
	return browser.exited
}

func (browser *headlessBrowser) Stop() {
	_ = browser.command.Process.Kill()
	<-browser.exited
}

func TestCompanyBrowserToolsDriveARealPage(t *testing.T) {
	service := companyBrowserService(t)
	page := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(signupPage))
	}))
	t.Cleanup(page.Close)

	invokeCompanyBrowserTool(t, service, "browser_open", fmt.Sprintf(`{"url":%q}`, page.URL))

	t.Run("browser_fill", func(t *testing.T) {
		invokeCompanyBrowserTool(t, service, "browser_fill", `{"selector":"#name","text":"이샘플"}`)
		expectCompanyBrowserPageShows(t, service, "이샘플")
	})
	t.Run("browser_select", func(t *testing.T) {
		invokeCompanyBrowserTool(t, service, "browser_select", `{"selector":"#team","value":"support"}`)
		expectCompanyBrowserPageShows(t, service, "Support")
	})
	t.Run("browser_press", func(t *testing.T) {
		invokeCompanyBrowserTool(t, service, "browser_press", `{"key":"Enter"}`)
	})
	t.Run("browser_wait", func(t *testing.T) {
		invokeCompanyBrowserTool(t, service, "browser_wait", `{"selector":"#received"}`)
		expectCompanyBrowserPageShows(t, service, "received 이샘플 support")
	})
}

func TestCompanyBrowserScreenshotReturnsTheRenderedPage(t *testing.T) {
	service := companyBrowserService(t)
	page := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(`<!doctype html><body style="margin:0;background:#fff"><div style="width:300px;height:200px;background:#0a0"></div></body>`))
	}))
	t.Cleanup(page.Close)
	invokeCompanyBrowserTool(t, service, "browser_open", fmt.Sprintf(`{"url":%q}`, page.URL))

	response := invokeCompanyBrowserTool(t, service, "browser_screenshot", `{}`)

	var result struct {
		Attachments []struct {
			ContentType   string `json:"contentType"`
			SizeBytes     int64  `json:"sizeBytes"`
			ContentBase64 string `json:"contentBase64"`
		} `json:"attachments"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil || len(result.Attachments) != 1 {
		t.Fatalf("expected one screenshot attachment, got %s (%v)", response.Result, errorValue)
	}
	content, errorValue := base64.StdEncoding.DecodeString(result.Attachments[0].ContentBase64)
	if errorValue != nil || int64(len(content)) != result.Attachments[0].SizeBytes || result.Attachments[0].ContentType != "image/png" {
		t.Fatalf("expected a PNG attachment whose size matches its bytes: %v", errorValue)
	}
	picture, errorValue := png.Decode(bytes.NewReader(content))
	if errorValue != nil {
		t.Fatalf("screenshot is not a PNG: %v", errorValue)
	}
	if !showsMoreThanOneColor(picture) {
		t.Fatal("screenshot is blank: the green block on the white page did not render")
	}
}

func showsMoreThanOneColor(picture image.Image) bool {
	bounds := picture.Bounds()
	firstRed, firstGreen, firstBlue, _ := picture.At(bounds.Min.X, bounds.Min.Y).RGBA()
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 8 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 8 {
			red, green, blue, _ := picture.At(x, y).RGBA()
			if red != firstRed || green != firstGreen || blue != firstBlue {
				return true
			}
		}
	}
	return false
}

func companyBrowserService(t *testing.T) Service {
	executablePath := strings.TrimSpace(os.Getenv("INTERNKIM_TEST_CDP_BROWSER"))
	if executablePath == "" {
		t.Skip("INTERNKIM_TEST_CDP_BROWSER names a Chromium-family executable to stand in for Moli")
	}
	agentBrowserPath := firstNonEmpty(os.Getenv("INTERNKIM_AGENT_BROWSER_PATH"), "agent-browser")
	if _, errorValue := exec.LookPath(agentBrowserPath); errorValue != nil {
		t.Skip("agent-browser is not installed")
	}
	browsers := browserruntime.NewDeviceBrowsers(browserruntime.DeviceBrowserSettings{
		ExecutablePath: executablePath,
		StateDirectory: t.TempDir(),
		FirstPort:      freeLocalPort(t),
		Capacity:       1,
		Launch:         launchHeadlessBrowser,
	})
	t.Cleanup(browsers.StopAll)
	service := Service{
		Configuration:  Configuration{AgentBrowserPath: agentBrowserPath},
		DeviceBrowsers: browsers,
	}
	t.Cleanup(func() { closeCompanyBrowserSession(service) })
	return service
}

func launchHeadlessBrowser(ctx context.Context, launch browserruntime.DeviceBrowserLaunch) (browserruntime.RunningDeviceBrowser, error) {
	command := exec.Command(launch.ExecutablePath,
		"--headless",
		"--password-store=basic",
		"--use-mock-keychain",
		"--no-first-run",
		"--remote-debugging-port="+fmt.Sprint(launch.Port),
		"--user-data-dir="+launch.ProfileDirectory,
		"about:blank",
	)
	if errorValue := command.Start(); errorValue != nil {
		return nil, errorValue
	}
	browser := &headlessBrowser{command: command, exited: make(chan struct{})}
	go func() {
		_ = command.Wait()
		close(browser.exited)
	}()
	if errorValue := waitForDevtoolsVersion(ctx, launch.Port); errorValue != nil {
		browser.Stop()
		return nil, errorValue
	}
	return browser, nil
}

func waitForDevtoolsVersion(ctx context.Context, port int) error {
	deadline := time.Now().Add(20 * time.Second)
	versionURL := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	for time.Now().Before(deadline) {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, nil)
		if response, errorValue := http.DefaultClient.Do(request); errorValue == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("the headless browser on port %d never served the DevTools Protocol", port)
}

func freeLocalPort(t *testing.T) int {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func invokeCompanyBrowserTool(t *testing.T, service Service, toolName string, input string) capabilities.ToolInvokeResponse {
	t.Helper()
	body := fmt.Sprintf(`{"input":%s,"context":{"requesterEmail":%q}}`, input, companyBrowserRequesterEmail)
	response, errorValue := service.invokeCapabilityTool(context.Background(), toolName, strings.NewReader(body))
	if errorValue != nil {
		t.Fatalf("%s failed: %v", toolName, errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded {
		t.Fatalf("%s did not succeed: %+v", toolName, response)
	}
	return response
}

func expectCompanyBrowserPageShows(t *testing.T, service Service, text string) {
	t.Helper()
	response := invokeCompanyBrowserTool(t, service, "browser_snapshot", `{}`)
	var snapshot struct {
		SnapshotText string `json:"snapshotText"`
	}
	if errorValue := json.Unmarshal(response.Result, &snapshot); errorValue != nil {
		t.Fatalf("browser_snapshot result is not a snapshot: %v", errorValue)
	}
	if !strings.Contains(snapshot.SnapshotText, text) {
		t.Fatalf("expected the page to show %q, got %q", text, snapshot.SnapshotText)
	}
}

func closeCompanyBrowserSession(service Service) {
	runtime, errorValue := service.deviceBrowserRuntime(context.Background(), capabilities.ToolInvokeContext{RequesterEmail: companyBrowserRequesterEmail})
	if errorValue != nil {
		return
	}
	_ = runtime.CloseSession(context.Background())
}
