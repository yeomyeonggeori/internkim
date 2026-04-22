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
	consentEmailAddresses  = "Email addresses"
	consentNext            = "Next"
	consentAgree           = "I agree to the Google API Services"
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

// findGoogleTab polls Chrome's /json/list until a page target on a
// google.com host (ideally console.cloud.google.com) appears, or
// timeout elapses. Needed because Chrome opens the CDP port before
// the initial tab has finished navigating — a one-shot check races
// the launch.
func findGoogleTab(timeout time.Duration) (target.ID, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		response, err := http.Get(fmt.Sprintf("http://localhost:%d/json/list", DebugPort))
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		var targets []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			URL  string `json:"url"`
		}
		decodeErr := json.NewDecoder(response.Body).Decode(&targets)
		response.Body.Close()
		if decodeErr != nil {
			time.Sleep(500 * time.Millisecond)
			continue
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
		if best != "" {
			return target.ID(best), nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", errors.New("no google.com tab found in Chrome within " + timeout.String())
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

// clickByText clicks the first visible element whose own text matches
// needle (case-insensitive), walking up to the nearest genuinely
// interactive ancestor. tagCSV is retained in the signature for
// call-site compatibility; universal text search is more robust than
// tag filtering.
//
// Ancestry walk is strict: the ancestor must be a real click target —
// <button>, <a href>, <input>, <label>, <option>, <summary>, a Material
// component that wraps an actual button, or an element with an explicit
// interactive role / jslog track attribute. Generic cfc-* / mat-card /
// container wrappers are NOT treated as clickable; they match too many
// non-interactive elements (empty-state cards, list containers).
func clickByText(ctx context.Context, tagCSV, needle string) error {
	_ = tagCSV
	encoded, _ := json.Marshal(needle)
	js := fmt.Sprintf(`(() => {
	  const needle = %s.toLowerCase();
	  const clickableTags = new Set([
	    'button','a','input','label','option','summary',
	    'mat-button','mat-flat-button','mat-raised-button','mat-stroked-button',
	    'mat-icon-button','mat-fab','mat-mini-fab',
	    'mat-menu-item','mat-option','mat-radio-button','mat-checkbox',
	    'mat-chip','mat-tab','mat-list-option','mat-expansion-panel-header'
	  ]);
	  const clickableRoles = new Set([
	    'button','radio','option','menuitem','menuitemradio','link','tab','checkbox','switch'
	  ]);
	  const materialButtonAttrs = [
	    'mat-button','mat-raised-button','mat-flat-button','mat-stroked-button',
	    'mat-icon-button','mat-fab','mat-mini-fab'
	  ];
	  const visible = el => {
	    const rect = el.getBoundingClientRect();
	    return rect.width > 0 && rect.height > 0;
	  };
	  const clickableAncestor = el => {
	    let cur = el;
	    for (let i = 0; i < 8 && cur; i++) {
	      const tag = (cur.tagName || '').toLowerCase();
	      if (clickableTags.has(tag)) {
	        if (tag === 'a' && !cur.hasAttribute('href') && !cur.hasAttribute('jslog')) {
	          // bare <a> without href and without a tracked click target
	          // is a span-in-disguise; keep walking
	        } else {
	          return cur;
	        }
	      }
	      if (cur.getAttribute) {
	        const role = cur.getAttribute('role');
	        if (role && clickableRoles.has(role)) return cur;
	        for (const attr of materialButtonAttrs) {
	          if (cur.hasAttribute(attr)) return cur;
	        }
	        const jslog = cur.getAttribute('jslog');
	        if (jslog && jslog.includes('track:')) return cur;
	      }
	      cur = cur.parentElement;
	    }
	    return null;
	  };
	  const all = document.querySelectorAll('*');
	  const exact = [];
	  const loose = [];
	  for (const n of all) {
	    const text = (n.innerText || n.textContent || '').trim().toLowerCase();
	    if (!text || !text.includes(needle)) continue;
	    if (text === needle) exact.push(n);
	    else loose.push(n);
	  }
	  // Prefer candidates with the shortest total text — a <span> that
	  // is just "External" beats a paragraph that happens to mention it.
	  const byLen = (a, b) => (a.textContent || '').length - (b.textContent || '').length;
	  exact.sort(byLen);
	  loose.sort(byLen);
	  for (const list of [exact, loose]) {
	    for (const candidate of list) {
	      if (!visible(candidate)) continue;
	      const target = clickableAncestor(candidate);
	      if (!target || !visible(target)) continue;
	      target.scrollIntoView({block: 'center', behavior: 'instant'});
	      target.click();
	      return true;
	    }
	  }
	  return false;
	})()`, string(encoded))
	var clicked bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &clicked)); err != nil {
		return err
	}
	if !clicked {
		return fmt.Errorf("no clickable element matching %q", needle)
	}
	return nil
}

// waitForURL blocks until window.location.href contains needle (case
// insensitive), or timeout elapses. Used to detect whether a click
// that was *supposed* to navigate actually did.
func waitForURL(ctx context.Context, needle string, timeout time.Duration) error {
	encoded, _ := json.Marshal(strings.ToLower(needle))
	js := fmt.Sprintf(`(window.location.href || '').toLowerCase().includes(%s)`, string(encoded))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var present bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(js, &present)); err == nil && present {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("waitForURL %q timed out", needle)
}

// pollClick retries clickByText every 500ms until it succeeds or
// timeout elapses. In debug mode, dumps a screenshot on timeout so
// the implementer can see which element the automation missed.
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
	snapshot(ctx, "pollClick-"+sanitizeStage(needle), false)
	return fmt.Errorf("pollClick %q timed out: %w", needle, lastErr)
}

// sanitizeStage produces a filename-safe suffix from an arbitrary
// needle string used in debug screenshot filenames.
func sanitizeStage(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			builder.WriteRune('-')
		}
	}
	result := builder.String()
	if result == "" {
		return "unknown"
	}
	if len(result) > 40 {
		result = result[:40]
	}
	return result
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

// pickFromSelect opens the combobox associated with the given label,
// then clicks the option whose visible text contains optionText. Works
// against Cloud Console's cfc-select + listbox overlay where plain
// fillFieldByLabel can't type into a role="combobox" element.
func pickFromSelect(ctx context.Context, labelText, optionText string) error {
	openParams, _ := json.Marshal(labelText)
	openJS := fmt.Sprintf(`(() => {
	  const needle = %s.toLowerCase();
	  const labels = [...document.querySelectorAll('label, mat-label, span, div')];
	  const label = labels.find(l => (l.textContent || '').trim().toLowerCase().includes(needle));
	  if (!label) return false;
	  let scope = label;
	  for (let i = 0; i < 6 && scope; i++) {
	    const combo = scope.querySelector('[role="combobox"], cfc-select, mat-select, select');
	    if (combo) {
	      combo.scrollIntoView({block: 'center', behavior: 'instant'});
	      combo.click();
	      return true;
	    }
	    scope = scope.parentElement;
	  }
	  return false;
	})()`, string(openParams))
	var opened bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(openJS, &opened)); err != nil {
		return err
	}
	if !opened {
		return fmt.Errorf("no combobox near label %q", labelText)
	}
	time.Sleep(800 * time.Millisecond)
	return pollClick(ctx, "", optionText, 10*time.Second)
}

// fillChipListByLabel types value into the input of a chip-list
// component (e.g. apis-email-chip-list) whose visible label or
// label="" attribute matches labelText, then dispatches an Enter
// keydown to commit the chip. Angular Material chip-lists observe
// keydown on the input, so synthetic events are enough.
func fillChipListByLabel(ctx context.Context, labelText, value string) error {
	params, _ := json.Marshal(map[string]string{"label": labelText, "value": value})
	js := fmt.Sprintf(`(() => {
	  const p = %s;
	  const needle = p.label.toLowerCase();
	  // apis-email-chip-list and friends expose a label="" attribute.
	  const attrHit = [...document.querySelectorAll('[label]')].find(
	    n => (n.getAttribute('label') || '').toLowerCase().includes(needle));
	  let scope = attrHit;
	  if (!scope) {
	    const labels = [...document.querySelectorAll('label, mat-label, span, div')];
	    const lbl = labels.find(l => (l.textContent || '').trim().toLowerCase().includes(needle));
	    if (!lbl) return false;
	    scope = lbl.closest('mat-form-field, apis-email-chip-list, cfc-select') || lbl.parentElement;
	  }
	  const input = scope.querySelector('input:not([type=hidden]):not([type=button]):not([type=submit])');
	  if (!input) return false;
	  input.focus();
	  const proto = Object.getPrototypeOf(input);
	  const desc = Object.getOwnPropertyDescriptor(proto, 'value');
	  if (desc && desc.set) desc.set.call(input, p.value);
	  else input.value = p.value;
	  input.dispatchEvent(new Event('input', {bubbles: true}));
	  const enterInit = {key: 'Enter', code: 'Enter', keyCode: 13, which: 13, bubbles: true, cancelable: true};
	  input.dispatchEvent(new KeyboardEvent('keydown', enterInit));
	  input.dispatchEvent(new KeyboardEvent('keypress', enterInit));
	  input.dispatchEvent(new KeyboardEvent('keyup', enterInit));
	  input.dispatchEvent(new Event('change', {bubbles: true}));
	  input.dispatchEvent(new Event('blur', {bubbles: true}));
	  return true;
	})()`, string(params))
	var ok bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &ok)); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no chip-list near label %q", labelText)
	}
	return nil
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
	tabID, err := findGoogleTab(60 * time.Second)
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

// fail snapshots the page and wraps the given error so the caller can
// just `return "", "", fail(ctx, "stage", err)`. No manual fallback —
// the user explicitly asked that any automation miss abort the run
// with enough artifacts to debug.
func fail(ctx context.Context, stage string, err error) error {
	saveFailureSnapshot(ctx, stage)
	return fmt.Errorf("%s: %w", stage, err)
}

// configureConsentIfNeeded drives the OAuth consent screen wizard when
// the project doesn't have one configured yet. No-ops when consent is
// already set up.
func configureConsentIfNeeded(ctx context.Context, projectID string) error {
	consentURL := "https://console.cloud.google.com/auth/overview?project=" + projectID + "&hl=en"
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
		// Neither signal present — the page layout changed, or we are
		// on a locale/variant we do not recognise. Snapshot and abort.
		return fail(ctx, "consent-landing", err)
	}
	return automateConsent(ctx)
}

func automateConsent(ctx context.Context) error {
	if err := pollClick(ctx, "", consentGetStarted, 10*time.Second); err != nil {
		return fail(ctx, "click-get-started", err)
	}
	// The Get started CTA is an <a href="/auth/overview/create">. If
	// the click lands on the wrong wrapper element the URL will never
	// change — catch that immediately rather than waiting out the next
	// step's timeout.
	if err := waitForURL(ctx, "/auth/overview/create", 10*time.Second); err != nil {
		return fail(ctx, "after-get-started", err)
	}
	time.Sleep(2 * time.Second)

	email := gcloudAccount()
	if email == "" {
		return fail(ctx, "gcloud-account", errors.New("gcloud has no active account"))
	}

	// Step 1: App Information — App name + User support email.
	// Steps 2-4 are collapsed (display: none) until step 1 advances.
	if err := fillFieldByLabel(ctx, consentAppNameLabel, "Intern Kim"); err != nil {
		return fail(ctx, "fill-app-name", err)
	}
	if err := pickFromSelect(ctx, consentSupportEmail, email); err != nil {
		return fail(ctx, "pick-support-email", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := pollClick(ctx, "", consentNext, 10*time.Second); err != nil {
		return fail(ctx, "step1-next", err)
	}
	time.Sleep(1500 * time.Millisecond)

	// Step 2: Audience — External radio.
	if err := pollClick(ctx, "", consentExternal, 10*time.Second); err != nil {
		return fail(ctx, "pick-external", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := pollClick(ctx, "", consentNext, 10*time.Second); err != nil {
		return fail(ctx, "step2-next", err)
	}
	time.Sleep(1500 * time.Millisecond)

	// Step 3: Contact Information — Email addresses chip-list.
	if err := fillChipListByLabel(ctx, consentEmailAddresses, email); err != nil {
		return fail(ctx, "fill-email-addresses", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := pollClick(ctx, "", consentNext, 10*time.Second); err != nil {
		return fail(ctx, "step3-next", err)
	}
	time.Sleep(1500 * time.Millisecond)

	// Step 4: Finish — agree checkbox + Create.
	if err := pollClick(ctx, "", consentAgree, 10*time.Second); err != nil {
		return fail(ctx, "agree-terms", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := pollClick(ctx, "", consentCreate, 10*time.Second); err != nil {
		return fail(ctx, "final-create", err)
	}

	// Confirm we land back on the overview page.
	if err := waitForText(ctx, consentEditApp, 20*time.Second); err != nil {
		if err := waitForText(ctx, consentBackToDashboard, 2*time.Second); err != nil {
			return fail(ctx, "after-create", err)
		}
	}
	return nil
}

// createOAuthDesktopClient drives the Console Credentials page to
// create a Desktop OAuth client and returns the ID/secret shown in the
// resulting modal.
func createOAuthDesktopClient(ctx context.Context, projectID string) (string, string, error) {
	credentialsURL := "https://console.cloud.google.com/apis/credentials?project=" + projectID + "&hl=en"
	if err := chromedp.Run(ctx,
		chromedp.Navigate(credentialsURL),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return "", "", err
	}
	return automateClientCreate(ctx)
}

func automateClientCreate(ctx context.Context) (string, string, error) {
	if err := waitForText(ctx, credentialsHeading, 15*time.Second); err != nil {
		return "", "", fail(ctx, "credentials-page", err)
	}
	if err := pollClick(ctx, "", credentialsCreate, 10*time.Second); err != nil {
		return "", "", fail(ctx, "open-create-credentials", err)
	}
	// Wait for the dropdown menu to render before picking the OAuth
	// item, otherwise we might match a text occurrence elsewhere on
	// the page (existing client rows, docs links).
	time.Sleep(1 * time.Second)
	if err := pollClick(ctx, "", credentialsOAuth, 10*time.Second); err != nil {
		return "", "", fail(ctx, "pick-oauth-client", err)
	}
	if err := waitForText(ctx, credentialsAppType, 15*time.Second); err != nil {
		return "", "", fail(ctx, "application-type-page", err)
	}
	if err := pollClick(ctx, "", credentialsAppType, 5*time.Second); err != nil {
		return "", "", fail(ctx, "open-app-type-select", err)
	}
	if err := pollClick(ctx, "", credentialsDesktop, 10*time.Second); err != nil {
		return "", "", fail(ctx, "pick-desktop-app", err)
	}
	if err := fillFieldByLabel(ctx, credentialsName, "Intern Kim Desktop"); err != nil {
		return "", "", fail(ctx, "fill-client-name", err)
	}
	if err := pollClick(ctx, "", credentialsCreate, 10*time.Second); err != nil {
		return "", "", fail(ctx, "click-final-create", err)
	}
	if err := waitForText(ctx, credentialsIDLabel, 20*time.Second); err != nil {
		if err := waitForText(ctx, credentialsCreated, 5*time.Second); err != nil {
			return "", "", fail(ctx, "result-modal", err)
		}
	}
	clientID, err := readFieldByLabel(ctx, credentialsIDLabel)
	if err != nil || clientID == "" {
		return "", "", fail(ctx, "read-client-id", err)
	}
	clientSecret, err := readFieldByLabel(ctx, credentialsSecret)
	if err != nil || clientSecret == "" {
		return "", "", fail(ctx, "read-client-secret", err)
	}
	if !strings.HasSuffix(clientID, ".apps.googleusercontent.com") {
		return "", "", fail(ctx, "validate-client-id",
			fmt.Errorf("client id %q does not match *.apps.googleusercontent.com", clientID))
	}
	if len(clientSecret) < 20 {
		return "", "", fail(ctx, "validate-client-secret",
			fmt.Errorf("client secret too short (%d chars)", len(clientSecret)))
	}
	return clientID, clientSecret, nil
}

// saveFailureSnapshot drops a PNG screenshot + the current page's
// outerHTML under ~/.internkim/debug-screenshots for post-mortem when
// the automation misses a selector. Best-effort; ignores any write or
// capture errors since it's purely diagnostic.
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
	base := filepath.Join(dir, fmt.Sprintf("%s-%s", stage, timestamp))

	captureCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var imageBytes []byte
	var html string
	_ = chromedp.Run(captureCtx,
		chromedp.CaptureScreenshot(&imageBytes),
		chromedp.Evaluate(`document.documentElement.outerHTML`, &html),
	)
	if len(imageBytes) > 0 {
		os.WriteFile(base+".png", imageBytes, 0o600)
	}
	if html != "" {
		os.WriteFile(base+".html", []byte(html), 0o600)
	}
}
