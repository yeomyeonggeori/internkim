// Package auth automates the one-off browser dance needed to obtain a
// user-owned OAuth 2.0 Desktop client ID + secret on a personal Google
// account. Google blocks the standard consent flow for sensitive
// Workspace scopes on unverified third-party clients, so the user must
// create their own OAuth client inside their own GCP project. There is
// no public API for creating Desktop OAuth clients, so we drive the
// Cloud Console UI with chromedp in the user's own signed-in Chrome
// session — same technique we used previously for the Apps Script
// editor, but now targeting
// https://console.cloud.google.com/apis/credentials.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// DebugPort is the CDP port Chrome exposes for the maintainer's profile.
// Fixed so both the OAuth loopback launcher and the chromedp driver
// attach to the same browser.
const DebugPort = 9335

// Visible-text strings we match against the live Cloud Console UI.
// Google ships Console revisions every few weeks, but these labels are
// far more stable than Material class names. Tweak in one place when
// the automation starts missing.
const (
	consentGetStarted      = "Get started"
	consentExternal        = "External"
	consentCreate          = "Create"
	consentAppNameLabel    = "App name"
	consentSupportEmail    = "User support email"
	consentDeveloperEmail  = "Developer contact"
	consentSaveContinue    = "Save and continue"
	consentBackToDashboard = "Back to dashboard"
	consentEditApp         = "Edit app"

	credentialsHeading = "Credentials"
	credentialsCreate  = "Create credentials"
	credentialsOAuth   = "OAuth client ID"
	credentialsAppType = "Application type"
	credentialsDesktop = "Desktop app"
	credentialsName    = "Name"
	credentialsIDLabel = "Your Client ID"
	credentialsSecret  = "Your Client Secret"
	credentialsCreated = "OAuth client created"
)

// chromeProfileDir returns the dedicated Chrome profile path that keeps
// Google sign-in cookies across runs. Located under the maintainer's
// internkim state dir so it survives `rm -rf /tmp` between reboots.
func chromeProfileDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".internkim", "chrome-profile")
}

// chromeBinary locates Google Chrome / Chromium on macOS. Returns "" if
// none installed.
func chromeBinary() string {
	for _, candidate := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// LaunchChrome opens the user-profile Chrome on DebugPort, navigating to
// openURL. Subsequent calls re-use the same process — the caller just
// re-launches with a new URL. Returns immediately; chromedp attaches
// separately.
func LaunchChrome(openURL string) error {
	profileDir := chromeProfileDir()
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return err
	}
	// Strip Chrome's singleton lock files left over from a hard kill so
	// the next launch doesn't refuse the profile.
	for _, name := range []string{"SingletonLock", "SingletonCookie", "SingletonSocket"} {
		os.Remove(filepath.Join(profileDir, name))
	}
	binary := chromeBinary()
	if binary == "" {
		return errors.New("Google Chrome or Chromium not installed at /Applications")
	}
	cmd := exec.Command(binary,
		"--user-data-dir="+profileDir,
		fmt.Sprintf("--remote-debugging-port=%d", DebugPort),
		"--no-first-run",
		"--no-default-browser-check",
		openURL,
	)
	return cmd.Start()
}

// waitForDebugPort blocks until Chrome's CDP endpoint accepts TCP
// connections, or timeout elapses.
func waitForDebugPort(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	address := fmt.Sprintf("localhost:%d", DebugPort)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("chrome CDP not reachable at %s within %s", address, timeout)
}

// findGoogleTab returns the first Chrome target already on a google.com
// host so chromedp attaches to the user's signed-in session rather than
// opening a fresh context.
func findGoogleTab() (target.ID, error) {
	response, err := http.Get(fmt.Sprintf("http://localhost:%d/json/list", DebugPort))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var targets []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(response.Body).Decode(&targets); err != nil {
		return "", err
	}
	var best string
	for _, info := range targets {
		if info.Type != "page" {
			continue
		}
		if strings.Contains(info.URL, "console.cloud.google.com") {
			return target.ID(info.ID), nil
		}
		if strings.Contains(info.URL, "google.com") && best == "" {
			best = info.ID
		}
	}
	if best == "" {
		return "", errors.New("no google.com tab found in Chrome")
	}
	return target.ID(best), nil
}

// debugEnabled returns true when INTERNKIM_CHROMEDP_DEBUG=1.
func debugEnabled() bool {
	return os.Getenv("INTERNKIM_CHROMEDP_DEBUG") == "1"
}

// snapshot wraps saveFailureSnapshot; skipped unless force or
// debugEnabled().
func snapshot(ctx context.Context, stage string, force bool) {
	if !force && !debugEnabled() {
		return
	}
	saveFailureSnapshot(ctx, stage)
}

// gcloudAccount returns the user's active gcloud account (an email), or
// "" when none is set. Used to auto-fill the consent wizard.
func gcloudAccount() string {
	out, err := exec.Command("gcloud", "config", "get-value", "account", "--quiet").Output()
	if err != nil {
		return ""
	}
	value := strings.TrimSpace(string(out))
	if value == "(unset)" {
		return ""
	}
	return value
}

// clickByText clicks the first visible element matching any of the
// pipe-separated tags whose trimmed textContent contains needle
// (case-insensitive). Returns an error when nothing matches.
func clickByText(ctx context.Context, tagCSV, needle string) error {
	params, _ := json.Marshal(map[string]string{"tag": tagCSV, "needle": needle})
	js := fmt.Sprintf(`(() => {
	  const p = %s;
	  const tags = p.tag.split('|');
	  const needle = p.needle.toLowerCase();
	  for (const t of tags) {
	    const nodes = [...document.querySelectorAll(t)];
	    const el = nodes.find(n => {
	      if (n.offsetParent === null && n.tagName !== 'OPTION') return false;
	      const text = (n.innerText || n.textContent || '').trim().toLowerCase();
	      return text && text.includes(needle);
	    });
	    if (el) {
	      el.scrollIntoView({block: 'center', behavior: 'instant'});
	      el.click();
	      return true;
	    }
	  }
	  return false;
	})()`, string(params))
	var clicked bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &clicked)); err != nil {
		return err
	}
	if !clicked {
		return fmt.Errorf("no %s element matching %q", tagCSV, needle)
	}
	return nil
}

// pollClick retries clickByText every 500ms until it succeeds or
// timeout elapses.
func pollClick(ctx context.Context, tagCSV, needle string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := clickByText(ctx, tagCSV, needle); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("pollClick %s/%q timed out: %w", tagCSV, needle, lastErr)
}

// waitForText polls document.body.innerText for needle
// (case-insensitive) until it appears or timeout elapses.
func waitForText(ctx context.Context, needle string, timeout time.Duration) error {
	encoded, _ := json.Marshal(needle)
	js := fmt.Sprintf(`((document.body && document.body.innerText) || '').toLowerCase().includes(%s.toLowerCase())`, string(encoded))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var present bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(js, &present)); err == nil && present {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("waitForText %q timed out", needle)
}

// fillFieldByLabel types value into the input/textarea associated with
// a label whose text contains labelText. Uses the native value setter
// so Angular/React change detection picks up the update.
func fillFieldByLabel(ctx context.Context, labelText, value string) error {
	params, _ := json.Marshal(map[string]string{"label": labelText, "value": value})
	js := fmt.Sprintf(`(() => {
	  const p = %s;
	  const needle = p.label.toLowerCase();
	  const labels = [...document.querySelectorAll('label, span, div')];
	  const label = labels.find(l => (l.textContent || '').trim().toLowerCase().includes(needle));
	  if (!label) return false;
	  let input = null;
	  if (label.htmlFor) input = document.getElementById(label.htmlFor);
	  if (!input) {
	    let scope = label;
	    for (let i = 0; i < 4 && scope && !input; i++) {
	      input = scope.querySelector('input:not([type=hidden]):not([type=button]):not([type=submit]), textarea');
	      scope = scope.parentElement;
	    }
	  }
	  if (!input) return false;
	  input.focus();
	  const proto = Object.getPrototypeOf(input);
	  const desc = Object.getOwnPropertyDescriptor(proto, 'value');
	  if (desc && desc.set) desc.set.call(input, p.value);
	  else input.value = p.value;
	  input.dispatchEvent(new Event('input', {bubbles: true}));
	  input.dispatchEvent(new Event('change', {bubbles: true}));
	  return true;
	})()`, string(params))
	var ok bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &ok)); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no input near label %q", labelText)
	}
	return nil
}

// readFieldByLabel returns the trimmed value near a label matching
// labelText. Tries readonly <input>, <code>, and sibling text patterns
// in a small ancestor window.
func readFieldByLabel(ctx context.Context, labelText string) (string, error) {
	encoded, _ := json.Marshal(labelText)
	js := fmt.Sprintf(`(() => {
	  const needle = %s.toLowerCase();
	  const candidates = [...document.querySelectorAll('label, span, div, p')];
	  const label = candidates.find(l => (l.textContent || '').trim().toLowerCase().includes(needle));
	  if (!label) return '';
	  let scope = label;
	  for (let i = 0; i < 5 && scope; i++) {
	    const input = scope.querySelector('input[readonly], input[value], textarea');
	    if (input && input.value) return input.value.trim();
	    const code = scope.querySelector('code, pre');
	    if (code && (code.textContent || '').trim()) return code.textContent.trim();
	    scope = scope.parentElement;
	  }
	  return '';
	})()`, string(encoded))
	var value string
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &value)); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

// EnsureUserOAuthClient returns a (clientID, clientSecret) pair for the
// user's own OAuth 2.0 Desktop client in the given GCP project,
// creating the client if one does not already exist.
//
// Flow: attach to the maintainer's Chrome on DebugPort, drive the
// Cloud Console UI to configure the OAuth consent screen (External,
// user themselves as test user) the first time, then Create Credentials
// → OAuth client ID → Desktop. Extract client_id and client_secret
// from the resulting modal via DOM scraping.
//
// The caller is responsible for launching Chrome first via LaunchChrome
// (so the user can complete Google sign-in once). Subsequent calls in
// the same Chrome session skip the sign-in prompt.
func EnsureUserOAuthClient(projectID string) (string, string, error) {
	if err := waitForDebugPort(15 * time.Second); err != nil {
		return "", "", err
	}
	tabID, err := findGoogleTab()
	if err != nil {
		return "", "", err
	}

	allocator, cancelAllocator := chromedp.NewRemoteAllocator(
		context.Background(),
		fmt.Sprintf("http://localhost:%d", DebugPort),
	)
	defer cancelAllocator()

	browserCtx, cancelBrowser := chromedp.NewContext(allocator,
		chromedp.WithTargetID(tabID))
	defer cancelBrowser()

	runCtx, cancelRun := context.WithTimeout(browserCtx, 10*time.Minute)
	defer cancelRun()

	if err := configureConsentIfNeeded(runCtx, projectID); err != nil {
		return "", "", fmt.Errorf("consent screen: %w", err)
	}

	clientID, clientSecret, err := createOAuthDesktopClient(runCtx, projectID)
	if err != nil {
		saveFailureSnapshot(runCtx, "oauth-client")
		return "", "", err
	}
	if clientID == "" || clientSecret == "" {
		return "", "", errors.New("failed to extract OAuth client credentials from Console")
	}
	return clientID, clientSecret, nil
}

// configureConsentIfNeeded drives the OAuth consent screen wizard if
// the project doesn't have one configured yet. Idempotent: no-ops when
// consent is already set up. Falls through to manual prompts on any
// automation failure so the user is never stuck.
func configureConsentIfNeeded(ctx context.Context, projectID string) error {
	consentURL := "https://console.cloud.google.com/auth/overview?project=" + projectID
	if err := chromedp.Run(ctx,
		chromedp.Navigate(consentURL),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return err
	}
	// Already configured → the overview page shows "Edit app" instead
	// of "Get started".
	if err := waitForText(ctx, consentEditApp, 3*time.Second); err == nil {
		return nil
	}
	if err := waitForText(ctx, consentGetStarted, 10*time.Second); err != nil {
		// Some projects land on the legacy credentials/consent path.
		fallbackURL := "https://console.cloud.google.com/apis/credentials/consent?project=" + projectID
		chromedp.Run(ctx, chromedp.Navigate(fallbackURL), chromedp.Sleep(3*time.Second))
		if err := waitForText(ctx, consentGetStarted, 10*time.Second); err != nil {
			return nil
		}
	}
	if err := automateConsent(ctx); err != nil {
		fmt.Println()
		fmt.Printf("  동의 화면 자동화 실패 (%v) — 수동 진행으로 전환합니다.\n", err)
		snapshot(ctx, "consent-fail", true)
		return manualConsentPrompt()
	}
	return nil
}

func automateConsent(ctx context.Context) error {
	if err := pollClick(ctx, "button|span", consentGetStarted, 10*time.Second); err != nil {
		return fmt.Errorf("click Get started: %w", err)
	}
	if err := pollClick(ctx, "label|span|div", consentExternal, 10*time.Second); err != nil {
		return fmt.Errorf("select External: %w", err)
	}
	if err := pollClick(ctx, "button", consentCreate, 10*time.Second); err != nil {
		return fmt.Errorf("click Create: %w", err)
	}
	if err := fillFieldByLabel(ctx, consentAppNameLabel, "Intern Kim"); err != nil {
		return fmt.Errorf("fill app name: %w", err)
	}
	email := gcloudAccount()
	if email != "" {
		if err := fillFieldByLabel(ctx, consentSupportEmail, email); err != nil {
			clickByText(ctx, "div|input", consentSupportEmail)
			clickByText(ctx, "span|li", email)
		}
		if err := fillFieldByLabel(ctx, consentDeveloperEmail, email); err != nil {
			clickByText(ctx, "div|input", consentDeveloperEmail)
			clickByText(ctx, "span|li", email)
		}
	}
	// Walk through the multi-page wizard ("Save and continue" →
	// Scopes → Test users → Summary). Bail as soon as the overview
	// page's "Back to dashboard"/"Edit app" confirms completion.
	for i := 0; i < 5; i++ {
		if err := waitForText(ctx, consentBackToDashboard, 2*time.Second); err == nil {
			return nil
		}
		if err := waitForText(ctx, consentEditApp, 1*time.Second); err == nil {
			return nil
		}
		if err := pollClick(ctx, "button", consentSaveContinue, 15*time.Second); err != nil {
			return fmt.Errorf("save-and-continue step %d: %w", i, err)
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return nil
}

func manualConsentPrompt() error {
	fmt.Println()
	fmt.Println("  Cloud Console의 OAuth 동의 화면 설정이 필요합니다.")
	fmt.Println("  열린 Chrome 창에서 아래 순서로 진행해주세요:")
	fmt.Println("  1. User type: External → Create")
	fmt.Println("  2. App information: 앱 이름 아무거나 (예: Intern Kim), 지원 이메일에 본인 이메일")
	fmt.Println("  3. Developer contact에 본인 이메일")
	fmt.Println("  4. Save and continue 계속 (Scopes 비워두고 Save, Test users에 본인 이메일 추가 후 Save)")
	fmt.Println("  5. Back to Dashboard")
	fmt.Println()
	fmt.Println("  완료 후 Enter를 눌러주세요...")
	fmt.Scanln()
	return nil
}

// createOAuthDesktopClient drives the Console Credentials page to create
// a Desktop OAuth client and returns the ID/secret shown in the
// resulting modal. Falls back to terminal paste prompts on any
// automation failure so the user is never stuck.
func createOAuthDesktopClient(ctx context.Context, projectID string) (string, string, error) {
	credentialsURL := "https://console.cloud.google.com/apis/credentials?project=" + projectID
	if err := chromedp.Run(ctx,
		chromedp.Navigate(credentialsURL),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return "", "", err
	}
	clientID, clientSecret, err := automateClientCreate(ctx)
	if err == nil && clientID != "" && clientSecret != "" {
		return clientID, clientSecret, nil
	}
	if err != nil {
		fmt.Println()
		fmt.Printf("  클라이언트 생성 자동화 실패 (%v) — 수동 진행으로 전환합니다.\n", err)
		snapshot(ctx, "client-fail", true)
	}
	return manualClientPrompt()
}

func automateClientCreate(ctx context.Context) (string, string, error) {
	if err := waitForText(ctx, credentialsHeading, 15*time.Second); err != nil {
		return "", "", fmt.Errorf("wait for credentials page: %w", err)
	}
	if err := pollClick(ctx, "button|span|a", credentialsCreate, 10*time.Second); err != nil {
		return "", "", fmt.Errorf("open Create credentials menu: %w", err)
	}
	if err := pollClick(ctx, "li|span|div", credentialsOAuth, 10*time.Second); err != nil {
		return "", "", fmt.Errorf("pick OAuth client ID: %w", err)
	}
	if err := waitForText(ctx, credentialsAppType, 15*time.Second); err != nil {
		return "", "", fmt.Errorf("wait for Application type: %w", err)
	}
	// Open the select, then pick Desktop app. Best-effort on the select
	// click since the combobox sometimes opens on focus alone.
	pollClick(ctx, "div|mat-select|span", credentialsAppType, 5*time.Second)
	if err := pollClick(ctx, "li|mat-option|span", credentialsDesktop, 10*time.Second); err != nil {
		return "", "", fmt.Errorf("pick Desktop app: %w", err)
	}
	if err := fillFieldByLabel(ctx, credentialsName, "Intern Kim Desktop"); err != nil {
		// Name field is sometimes auto-populated; not fatal.
		snapshot(ctx, "client-name", false)
	}
	if err := pollClick(ctx, "button", credentialsCreate, 10*time.Second); err != nil {
		return "", "", fmt.Errorf("click final Create: %w", err)
	}
	if err := waitForText(ctx, credentialsIDLabel, 20*time.Second); err != nil {
		if err := waitForText(ctx, credentialsCreated, 5*time.Second); err != nil {
			return "", "", fmt.Errorf("wait for result modal: %w", err)
		}
	}
	clientID, err := readFieldByLabel(ctx, credentialsIDLabel)
	if err != nil || clientID == "" {
		return "", "", fmt.Errorf("read client id: %w", err)
	}
	clientSecret, err := readFieldByLabel(ctx, credentialsSecret)
	if err != nil || clientSecret == "" {
		return "", "", fmt.Errorf("read client secret: %w", err)
	}
	if !strings.HasSuffix(clientID, ".apps.googleusercontent.com") {
		return "", "", fmt.Errorf("client id %q does not look valid", clientID)
	}
	if len(clientSecret) < 20 {
		return "", "", fmt.Errorf("client secret too short (%d chars)", len(clientSecret))
	}
	// Best-effort modal dismiss so the Console is clean for the next run.
	pollClick(ctx, "button", "OK", 3*time.Second)
	return clientID, clientSecret, nil
}

func manualClientPrompt() (string, string, error) {
	fmt.Println()
	fmt.Println("  Chrome 창에서 OAuth Desktop client를 생성해주세요:")
	fmt.Println("  1. '+ Create Credentials' → 'OAuth client ID'")
	fmt.Println("  2. Application type: Desktop app")
	fmt.Println("  3. Name: 아무거나 (예: Intern Kim Desktop)")
	fmt.Println("  4. Create 클릭")
	fmt.Println("  5. 뜨는 모달에서 Client ID / Client Secret 복사해 아래 붙여넣기")
	fmt.Println()
	fmt.Print("  Client ID: ")
	var clientID, clientSecret string
	fmt.Scanln(&clientID)
	fmt.Print("  Client Secret: ")
	fmt.Scanln(&clientSecret)
	return strings.TrimSpace(clientID), strings.TrimSpace(clientSecret), nil
}

// saveFailureSnapshot drops a PNG screenshot of the current page under
// ~/.internkim/debug-screenshots for post-mortem when the automation
// misses a selector. Best-effort.
func saveFailureSnapshot(ctx context.Context, stage string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".internkim", "debug-screenshots")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	timestamp := time.Now().Format("20060102-150405")
	var imageBytes []byte
	screenshotCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	_ = chromedp.Run(screenshotCtx, chromedp.CaptureScreenshot(&imageBytes))
	cancel()
	if len(imageBytes) > 0 {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%s.png", stage, timestamp)), imageBytes, 0o600)
	}
}
