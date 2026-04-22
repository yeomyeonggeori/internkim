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

	"github.com/chromedp/cdproto/browser"
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

// LaunchChrome makes sure a Chrome with our internkim profile is
// running on DebugPort. If the port is already reachable — meaning
// our Chrome is still running from a previous invocation — the
// function returns immediately without spawning a second Chrome,
// which would fight the first for the profile lock and surface as
// the "Something went wrong when opening your profile" dialog. The
// caller navigates to the target URL via chromedp once attached, so
// the openURL here is only needed when a truly fresh Chrome starts.
func LaunchChrome(openURL string) error {
	// Already-running Chrome with our profile? Reuse it.
	address := fmt.Sprintf("localhost:%d", DebugPort)
	if conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond); err == nil {
		conn.Close()
		return nil
	}

	profileDir := chromeProfileDir()
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return err
	}
	// Strip singleton lock files left behind by a hard-killed Chrome
	// so the fresh launch doesn't refuse the profile. Safe to remove
	// only because we already confirmed the debug port is unreachable
	// (no live Chrome is using it).
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

// clickByText finds the first visible interactive element whose own
// text matches needle and dispatches a real CDP mouse click at its
// center. Using CDP (not synthetic el.click()) is critical for
// Material / Angular apps whose (click) handlers react to the full
// pointerdown → mouseup → click sequence; a bare el.click() fires
// only the final click event and often gets ignored.
//
// Ancestor walk is strict: the clickable must be a real interactive
// tag (<button>, <a href>, <input>, <label>, <option>, <summary>),
// a specific Material button component, or an element with an explicit
// interactive role / jslog track attribute. Generic container
// wrappers (cfc-*, mat-card, etc.) are NOT considered clickable.
//
// tagCSV is retained for call-site compatibility but unused — universal
// text search + clickable-ancestor walk is more robust.
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
	  const byLen = (a, b) => (a.textContent || '').length - (b.textContent || '').length;
	  exact.sort(byLen);
	  loose.sort(byLen);
	  for (const list of [exact, loose]) {
	    for (const candidate of list) {
	      if (!visible(candidate)) continue;
	      const target = clickableAncestor(candidate);
	      if (!target || !visible(target)) continue;
	      target.scrollIntoView({block: 'center', behavior: 'instant'});
	      const rect = target.getBoundingClientRect();
	      const x = rect.left + rect.width / 2;
	      const y = rect.top + rect.height / 2;
	      const pInit = {pointerId: 1, pointerType: 'mouse', isPrimary: true,
	        bubbles: true, cancelable: true, composed: true, view: window,
	        clientX: x, clientY: y, screenX: x, screenY: y,
	        button: 0, buttons: 1};
	      const mInit = {bubbles: true, cancelable: true, composed: true, view: window,
	        clientX: x, clientY: y, screenX: x, screenY: y,
	        button: 0, buttons: 1, detail: 1};
	      try { target.focus({preventScroll: true}); } catch (e) {}
	      target.dispatchEvent(new PointerEvent('pointerover', pInit));
	      target.dispatchEvent(new MouseEvent('mouseover', mInit));
	      target.dispatchEvent(new PointerEvent('pointerenter', pInit));
	      target.dispatchEvent(new MouseEvent('mouseenter', mInit));
	      target.dispatchEvent(new PointerEvent('pointerdown', pInit));
	      target.dispatchEvent(new MouseEvent('mousedown', mInit));
	      target.dispatchEvent(new PointerEvent('pointerup', {...pInit, buttons: 0}));
	      target.dispatchEvent(new MouseEvent('mouseup', {...mInit, buttons: 0}));
	      target.dispatchEvent(new MouseEvent('click', {...mInit, buttons: 0}));
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

// fillFieldByLabel types value into the <input>/<textarea> inside the
// mat-form-field whose <mat-label> or <label> matches labelText
// exactly. Strictly scoped to the enclosing form-field container so
// an unbounded ancestor walk can't escape the form and land on the
// Cloud Console top-bar search input.
func fillFieldByLabel(ctx context.Context, labelText, value string) error {
	params, _ := json.Marshal(map[string]string{"label": labelText, "value": value})
	js := fmt.Sprintf(`(() => {
	  const p = %s;
	  const needle = p.label.trim().toLowerCase();
	  const labels = [
	    ...document.querySelectorAll('mat-label'),
	    ...document.querySelectorAll('label'),
	  ];
	  const label = labels.find(l => (l.textContent || '').trim().toLowerCase() === needle);
	  if (!label) return false;
	  const scope = label.closest('mat-form-field, apis-email-chip-list, cfc-select, [formcontrolname]');
	  if (!scope) return false;
	  const input = scope.querySelector(
	    'input:not([type=hidden]):not([type=button]):not([type=submit]):not([type=search]), textarea'
	  );
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
		return fmt.Errorf("no input in form-field for mat-label/label %q", labelText)
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

// pickFromSelect opens the combobox inside the mat-form-field whose
// <mat-label> or <label> matches labelText exactly, then clicks the
// option whose visible text contains optionText. Dispatches a full
// PointerEvent + MouseEvent sequence so Angular Material's cfc-select
// / mat-select overlays actually open (synthetic .click() alone is
// ignored by the CDK overlay trigger).
func pickFromSelect(ctx context.Context, labelText, optionText string) error {
	openParams, _ := json.Marshal(labelText)
	openJS := fmt.Sprintf(`(() => {
	  const needle = %s.trim().toLowerCase();
	  const labels = [
	    ...document.querySelectorAll('mat-label'),
	    ...document.querySelectorAll('label'),
	  ];
	  const label = labels.find(l => (l.textContent || '').trim().toLowerCase() === needle);
	  if (!label) return false;
	  const scope = label.closest('mat-form-field, cfc-select, [formcontrolname]');
	  if (!scope) return false;
	  const combo = scope.querySelector('[role="combobox"], cfc-select, mat-select, select');
	  if (!combo) return false;
	  combo.scrollIntoView({block: 'center', behavior: 'instant'});
	  const rect = combo.getBoundingClientRect();
	  const x = rect.left + rect.width / 2;
	  const y = rect.top + rect.height / 2;
	  const pInit = {pointerId: 1, pointerType: 'mouse', isPrimary: true,
	    bubbles: true, cancelable: true, composed: true, view: window,
	    clientX: x, clientY: y, screenX: x, screenY: y,
	    button: 0, buttons: 1};
	  const mInit = {bubbles: true, cancelable: true, composed: true, view: window,
	    clientX: x, clientY: y, screenX: x, screenY: y,
	    button: 0, buttons: 1, detail: 1};
	  try { combo.focus({preventScroll: true}); } catch (e) {}
	  combo.dispatchEvent(new PointerEvent('pointerdown', pInit));
	  combo.dispatchEvent(new MouseEvent('mousedown', mInit));
	  combo.dispatchEvent(new PointerEvent('pointerup', {...pInit, buttons: 0}));
	  combo.dispatchEvent(new MouseEvent('mouseup', {...mInit, buttons: 0}));
	  combo.dispatchEvent(new MouseEvent('click', {...mInit, buttons: 0}));
	  return true;
	})()`, string(openParams))
	var opened bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(openJS, &opened)); err != nil {
		return err
	}
	if !opened {
		return fmt.Errorf("no combobox in form-field for mat-label/label %q", labelText)
	}
	time.Sleep(800 * time.Millisecond)
	if err := pollClick(ctx, "", optionText, 10*time.Second); err != nil {
		return err
	}
	// Mark the combobox ng-touched by dispatching blur — Angular reactive
	// forms flip ng-untouched → ng-touched on blur, and the cfc-stepper's
	// Next button sometimes stays gated until the whole form group is
	// touched.
	blurParams, _ := json.Marshal(labelText)
	blurJS := fmt.Sprintf(`(() => {
	  const needle = %s.trim().toLowerCase();
	  const labels = [
	    ...document.querySelectorAll('mat-label'),
	    ...document.querySelectorAll('label'),
	  ];
	  const label = labels.find(l => (l.textContent || '').trim().toLowerCase() === needle);
	  if (!label) return false;
	  const scope = label.closest('mat-form-field, cfc-select, [formcontrolname]');
	  if (!scope) return false;
	  const combo = scope.querySelector('[role="combobox"], cfc-select, mat-select, select');
	  if (!combo) return false;
	  combo.dispatchEvent(new FocusEvent('blur', {bubbles: true, composed: true}));
	  combo.dispatchEvent(new Event('change', {bubbles: true}));
	  return true;
	})()`, string(blurParams))
	chromedp.Run(ctx, chromedp.Evaluate(blurJS, new(bool)))
	return nil
}

// fillChipListByLabel types value into the input of a chip-list
// component (e.g. apis-email-chip-list) whose label="" attribute or
// <mat-label> matches labelText, then dispatches Enter key events to
// commit the chip. Strictly scoped — won't fall back to generic text
// match that could land on the top-bar search input.
func fillChipListByLabel(ctx context.Context, labelText, value string) error {
	params, _ := json.Marshal(map[string]string{"label": labelText, "value": value})
	js := fmt.Sprintf(`(() => {
	  const p = %s;
	  const needle = p.label.trim().toLowerCase();
	  let scope = [...document.querySelectorAll('[label]')].find(
	    n => (n.getAttribute('label') || '').trim().toLowerCase() === needle
	  );
	  if (!scope) {
	    const labels = [
	      ...document.querySelectorAll('mat-label'),
	      ...document.querySelectorAll('label'),
	    ];
	    const lbl = labels.find(l => (l.textContent || '').trim().toLowerCase() === needle);
	    if (!lbl) return false;
	    scope = lbl.closest('apis-email-chip-list, mat-form-field, cfc-select, [formcontrolname]');
	    if (!scope) return false;
	  }
	  const input = scope.querySelector(
	    'input:not([type=hidden]):not([type=button]):not([type=submit]):not([type=search])'
	  );
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

	allocator, cancelAllocator := chromedp.NewRemoteAllocator(
		context.Background(),
		fmt.Sprintf("http://localhost:%d", DebugPort),
	)
	defer cancelAllocator()

	// Prefer attaching to an already-open google.com tab so chromedp
	// drives the visible window the user can see. If no such tab
	// exists (Chrome has other tabs, or none), open a fresh tab in the
	// same profile — cookies and signed-in session are shared.
	var browserCtx context.Context
	var cancelBrowser context.CancelFunc
	if tabID, findErr := findGoogleTab(10 * time.Second); findErr == nil {
		browserCtx, cancelBrowser = chromedp.NewContext(allocator,
			chromedp.WithTargetID(tabID))
	} else {
		browserCtx, cancelBrowser = chromedp.NewContext(allocator)
	}
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
	// If the page shows "Get started" within a short window, consent
	// is NOT configured and we need to run the wizard.
	if err := waitForText(ctx, consentGetStarted, 5*time.Second); err == nil {
		return automateConsent(ctx)
	}
	// Otherwise assume consent is already configured: the overview
	// page renders with "OAuth Overview" / "Create OAuth client" /
	// "Edit app" etc. depending on Console revision. If the assumption
	// is wrong, client creation will fail with its own snapshot and
	// a clearer error than we could produce here.
	return nil
}

// checkAgreementBox finds the "I agree to the Google API Services"
// checkbox (by proximity to the "I agree" label text), clicks its
// inner <input type="checkbox"> — synthetic clicks on the surrounding
// <label> don't always flip the underlying checkbox state — and
// returns an error unless the checkbox ends up checked.
func checkAgreementBox(ctx context.Context) error {
	js := `(() => {
	  const labels = [...document.querySelectorAll('label')];
	  const label = labels.find(l => (l.textContent || '').toLowerCase().includes('i agree'));
	  if (!label) return false;
	  const scope = label.closest('mat-checkbox') || label.parentElement;
	  if (!scope) return false;
	  const input = scope.querySelector('input[type="checkbox"]');
	  if (!input) return false;
	  input.scrollIntoView({block: 'center', behavior: 'instant'});
	  input.focus({preventScroll: true});
	  if (!input.checked) input.click();
	  input.dispatchEvent(new Event('change', {bubbles: true}));
	  input.dispatchEvent(new FocusEvent('blur', {bubbles: true, composed: true}));
	  return input.checked;
	})()`
	var checked bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &checked)); err != nil {
		return err
	}
	if !checked {
		return errors.New("agreement checkbox did not become checked")
	}
	return nil
}

// pressFinalCreate focuses the cfc-stepper submit button (text "Create"
// at the bottom of the stepper, marked by the stepper-submit-button
// class) and triggers it via a real keyboard Enter event — same
// technique as advanceStep for the stepper's Next clicks, needed
// because synthetic mouse events don't fire Angular's (click) here.
func pressFinalCreate(ctx context.Context) error {
	focusJS := `(() => {
	  const candidates = [
	    ...document.querySelectorAll('.cfc-stepper-submit-button'),
	    ...document.querySelectorAll('button[type="submit"]'),
	    ...document.querySelectorAll('button'),
	  ];
	  const target = candidates.find(b => {
	    const r = b.getBoundingClientRect();
	    if (r.width === 0 || r.height === 0) return false;
	    const t = (b.innerText || b.textContent || '').trim().toLowerCase();
	    return t === 'create';
	  });
	  if (!target) return false;
	  target.scrollIntoView({block: 'center', behavior: 'instant'});
	  target.focus({preventScroll: true});
	  return true;
	})()`
	var focused bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(focusJS, &focused)); err != nil {
		return err
	}
	if !focused {
		return errors.New("no visible Create button")
	}
	time.Sleep(120 * time.Millisecond)
	return chromedp.Run(ctx, chromedp.KeyEvent("\r"))
}

// pickSoleEnabledRadio clicks the inner <input type="radio"> of the
// only non-disabled mat-radio-button inside the given form group. Use
// this when a radio choice is selected by elimination (e.g. Audience
// External on a personal @gmail account where Internal is
// aria-disabled) — avoids text matching that would otherwise grab the
// disabled radio's tooltip copy containing the target word.
func pickSoleEnabledRadio(ctx context.Context, formGroupName string) error {
	encoded, _ := json.Marshal(formGroupName)
	js := fmt.Sprintf(`(() => {
	  const name = %s;
	  const group = document.querySelector('[formgroupname="' + name + '"]');
	  if (!group) return false;
	  const radios = [...group.querySelectorAll('mat-radio-button')];
	  const enabled = radios.find(r =>
	    r.getAttribute('aria-disabled') !== 'true' &&
	    !r.classList.contains('mat-mdc-radio-disabled'));
	  if (!enabled) return false;
	  const input = enabled.querySelector('input[type="radio"]');
	  if (!input) return false;
	  input.scrollIntoView({block: 'center', behavior: 'instant'});
	  input.focus({preventScroll: true});
	  input.click();
	  input.dispatchEvent(new Event('change', {bubbles: true}));
	  enabled.dispatchEvent(new FocusEvent('blur', {bubbles: true, composed: true}));
	  return true;
	})()`, string(encoded))
	var ok bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &ok)); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no enabled radio in form group %q", formGroupName)
	}
	return nil
}

func automateConsent(ctx context.Context) error {
	if err := pollClick(ctx, "", consentGetStarted, 10*time.Second); err != nil {
		return fail(ctx, "click-get-started", err)
	}
	// Get started CTA target is /auth/overview/create, which now
	// redirects to /auth/branding. Accept either.
	if err := waitForURL(ctx, "/auth/overview/create", 10*time.Second); err != nil {
		if err := waitForURL(ctx, "/auth/branding", 3*time.Second); err != nil {
			return fail(ctx, "after-get-started", err)
		}
	}
	time.Sleep(2 * time.Second)

	email := gcloudAccount()
	if email == "" {
		return fail(ctx, "gcloud-account", errors.New("gcloud has no active account"))
	}

	// Step 1: App Information — App name + User support email.
	if err := fillFieldByLabel(ctx, consentAppNameLabel, "Intern Kim"); err != nil {
		return fail(ctx, "fill-app-name", err)
	}
	if err := pickFromSelect(ctx, consentSupportEmail, email); err != nil {
		return fail(ctx, "pick-support-email", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := advanceStep(ctx, "audienceGroup"); err != nil {
		return fail(ctx, "step1-next", err)
	}

	// Step 2: Audience — External radio. On personal @gmail the
	// Internal option is aria-disabled, so External is the only
	// selectable choice. Pick the sole enabled radio in the audience
	// group by property (not by text match, which could hit Internal's
	// disabled tooltip that also contains the word "external"). Treat
	// the click as best-effort: if it misses, advanceStep's expansion
	// check will surface a real failure.
	if err := pickSoleEnabledRadio(ctx, "audienceGroup"); err != nil {
		snapshot(ctx, "pick-external-miss", false)
	}
	time.Sleep(500 * time.Millisecond)
	if err := advanceStep(ctx, "developerEmailsGroup"); err != nil {
		return fail(ctx, "step2-next", err)
	}

	// Step 3: Contact Information — Email addresses chip-list.
	if err := fillChipListByLabel(ctx, consentEmailAddresses, email); err != nil {
		return fail(ctx, "fill-email-addresses", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := advanceStep(ctx, "termsAgreementGroup"); err != nil {
		return fail(ctx, "step3-next", err)
	}

	// Step 4: Finish — agree checkbox + Create submit button.
	if err := checkAgreementBox(ctx); err != nil {
		return fail(ctx, "agree-terms", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := pressFinalCreate(ctx); err != nil {
		return fail(ctx, "final-create", err)
	}

	// Confirm the form was accepted: URL leaves /auth/branding (the
	// create wizard) and returns to the overview, or the overview's
	// "Edit app" / "Back to dashboard" signal text shows up.
	if err := waitForURL(ctx, "/auth/overview", 20*time.Second); err == nil {
		return nil
	}
	if err := waitForText(ctx, consentEditApp, 5*time.Second); err == nil {
		return nil
	}
	if err := waitForText(ctx, consentBackToDashboard, 2*time.Second); err == nil {
		return nil
	}
	return fail(ctx, "after-create", errors.New("create did not navigate away from wizard"))
}

// advanceStep focuses the current stepper's Next button and presses
// Enter via CDP. Keyboard Enter on a focused button fires a real
// trusted click event that Angular Material's stepper reliably acts
// on — synthetic mouse events weren't advancing the stepper even when
// the underlying form was valid + touched. Verifies the advance by
// waiting for the next step's form group to become expanded (its
// .cfc-stepper-step-content-and-continue child no longer display:none).
func advanceStep(ctx context.Context, nextFormGroupName string) error {
	focusJS := `(() => {
	  const buttons = [...document.querySelectorAll('.cfc-stepper-step-continue-button')];
	  const visible = buttons.find(b => {
	    const r = b.getBoundingClientRect();
	    return r.width > 0 && r.height > 0;
	  });
	  if (!visible) return false;
	  visible.scrollIntoView({block: 'center', behavior: 'instant'});
	  visible.focus({preventScroll: true});
	  return true;
	})()`
	var focused bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(focusJS, &focused)); err != nil {
		return err
	}
	if !focused {
		return errors.New("no visible cfc-stepper-step-continue-button")
	}
	time.Sleep(120 * time.Millisecond)
	if err := chromedp.Run(ctx, chromedp.KeyEvent("\r")); err != nil {
		return err
	}
	return waitForStepExpanded(ctx, nextFormGroupName, 15*time.Second)
}

// waitForStepExpanded polls for the cfc-stepper step identified by
// formGroupName becoming visible (its content-and-continue container
// leaving display:none). Definitive signal that the stepper actually
// advanced rather than the previous step staying expanded.
func waitForStepExpanded(ctx context.Context, formGroupName string, timeout time.Duration) error {
	encoded, _ := json.Marshal(formGroupName)
	js := fmt.Sprintf(`(() => {
	  const name = %s;
	  const step = document.querySelector('[formgroupname="' + name + '"]');
	  if (!step) return false;
	  const content = step.querySelector('.cfc-stepper-step-content-and-continue');
	  if (!content) return false;
	  if (content.style.display === 'none') return false;
	  return getComputedStyle(content).display !== 'none';
	})()`, string(encoded))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var expanded bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(js, &expanded)); err == nil && expanded {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("step %q did not expand within %s", formGroupName, timeout)
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
	if err := pickFromSelect(ctx, credentialsAppType, credentialsDesktop); err != nil {
		return "", "", fail(ctx, "pick-desktop-app", err)
	}
	if err := fillFieldByLabel(ctx, credentialsName, "Intern Kim Desktop"); err != nil {
		return "", "", fail(ctx, "fill-client-name", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := pressFinalCreate(ctx); err != nil {
		return "", "", fail(ctx, "click-final-create", err)
	}
	if err := waitForText(ctx, credentialsCreated, 20*time.Second); err != nil {
		return "", "", fail(ctx, "result-modal", err)
	}
	clientID, clientSecret, err := downloadAndParseClientJSON(ctx)
	if err != nil {
		return "", "", fail(ctx, "download-client-json", err)
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

// downloadAndParseClientJSON clicks the "Download JSON" button in the
// OAuth-client-created modal, waits for the resulting
// client_secret_*.apps.googleusercontent.com.json file to appear in
// the user's ~/Downloads, and parses out client_id + client_secret.
// Also tries a CDP-directed download dir as a best-effort short-circuit
// — if that takes effect we find the file there; otherwise we fall
// back to watching ~/Downloads where Chrome defaults.
func downloadAndParseClientJSON(ctx context.Context) (string, string, error) {
	downloadDir, err := os.MkdirTemp("", "internkim-oauth-*")
	if err != nil {
		return "", "", fmt.Errorf("mktemp download dir: %w", err)
	}
	defer os.RemoveAll(downloadDir)

	// Best-effort: redirect downloads to our temp dir. Not all Chrome
	// versions honor this so we also watch ~/Downloads below.
	chromedp.Run(ctx,
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath(downloadDir).
			WithEventsEnabled(true),
	)

	home, _ := os.UserHomeDir()
	userDownloads := filepath.Join(home, "Downloads")
	clickAt := time.Now()

	if err := pollClick(ctx, "", "Download JSON", 10*time.Second); err != nil {
		return "", "", fmt.Errorf("click Download JSON: %w", err)
	}

	jsonPath, err := waitForClientJSON(clickAt, 20*time.Second, downloadDir, userDownloads)
	if err != nil {
		return "", "", err
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return "", "", fmt.Errorf("read JSON: %w", err)
	}
	var parsed struct {
		Installed struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		} `json:"installed"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", "", fmt.Errorf("parse JSON: %w", err)
	}
	// Clean up the downloaded secret file from the user's Downloads —
	// credentials should not linger outside our state dir.
	os.Remove(jsonPath)
	return parsed.Installed.ClientID, parsed.Installed.ClientSecret, nil
}

// waitForClientJSON scans the given dirs for a client_secret_*.json
// file modified at or after since, returning its path as soon as one
// appears or timeout elapses. Tolerates Chrome's .crdownload temp
// suffix while the transfer is in-flight.
func waitForClientJSON(since time.Time, timeout time.Duration, dirs ...string) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, dir := range dirs {
			entries, _ := os.ReadDir(dir)
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasSuffix(name, ".crdownload") {
					continue
				}
				if !strings.HasSuffix(name, ".json") {
					continue
				}
				if !strings.HasPrefix(name, "client_secret") {
					continue
				}
				info, err := entry.Info()
				if err != nil {
					continue
				}
				if info.ModTime().Before(since.Add(-2 * time.Second)) {
					continue
				}
				return filepath.Join(dir, name), nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return "", fmt.Errorf("no client_secret_*.json appeared in %v within %s", dirs, timeout)
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
