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

// EnsureUserOAuthClient returns a (clientID, clientSecret) pair for the
// user's own OAuth 2.0 Desktop client in the given GCP project,
// creating the client if one does not already exist.
//
// Flow: attach to the maintainer's Chrome on DebugPort, navigate the
// Cloud Console credentials page for the project, configure the OAuth
// consent screen (External, testing mode, user themselves as test
// user) the first time, then Create Credentials → OAuth client ID →
// Desktop. Extract client_id and client_secret from the resulting
// modal via DOM scraping.
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

	credentialsURL := "https://console.cloud.google.com/apis/credentials?project=" + projectID

	if err := chromedp.Run(runCtx,
		chromedp.Navigate(credentialsURL),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return "", "", fmt.Errorf("navigate credentials: %w", err)
	}

	// If the consent screen isn't configured yet, Console redirects to
	// an "OAuth consent screen" landing inside the credentials page.
	// Detect that and automate it first.
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

// configureConsentIfNeeded runs the OAuth consent screen wizard when the
// project doesn't have one configured yet. Idempotent: no-ops when
// consent is already set up.
func configureConsentIfNeeded(ctx context.Context, projectID string) error {
	consentURL := "https://console.cloud.google.com/apis/credentials/consent?project=" + projectID
	if err := chromedp.Run(ctx,
		chromedp.Navigate(consentURL),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return err
	}
	// Detect "Get started" / "Configure consent screen" entry point.
	// If it's missing, consent is already configured.
	var needsSetup bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const text = (document.body.innerText || '').toLowerCase();
			return text.includes('get started') || text.includes('configure consent screen');
		})()
	`, &needsSetup)); err != nil {
		return err
	}
	if !needsSetup {
		return nil
	}
	// Consent setup is a multi-page wizard whose DOM varies between
	// Cloud Console revisions. Rather than hard-code selectors that
	// break every few months, we let the user click through: open the
	// browser, wait for them to finish. This is still better UX than
	// the current gws-auth-setup flow because the URL is pre-navigated
	// and the user's signed-in session auto-fills most fields.
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
// resulting modal.
func createOAuthDesktopClient(ctx context.Context, projectID string) (string, string, error) {
	credentialsURL := "https://console.cloud.google.com/apis/credentials?project=" + projectID
	if err := chromedp.Run(ctx,
		chromedp.Navigate(credentialsURL),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return "", "", err
	}

	// Similar rationale: Console Credentials UI changes enough that
	// hard-coded selectors for "Create Credentials → OAuth client ID →
	// Desktop app" rot quickly. Ask the user to click through once;
	// internkim captures the resulting client_id + secret.
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

// saveFailureSnapshot drops a PNG + DOM dump under
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
