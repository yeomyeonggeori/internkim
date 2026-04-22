// Package auth automates one-off Google consent flows that cannot go through
// regular OAuth because Google's 2025–2026 Granular Consent policy blocks
// unverified third-party apps from requesting sensitive Workspace scopes on
// personal @gmail.com accounts. The workaround is to drive the user's own
// Chrome session (where the cookies already carry the necessary auth) through
// the Apps Script editor UI via the Chrome DevTools Protocol.
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
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// DeployAppsScriptViaBrowser creates a new Apps Script project in the user's
// Drive, uploads codeGs + manifest, deploys it as a public-anonymous web app,
// and returns the resulting /macros/s/<id>/exec URL.
//
// The flow launches a non-headless Chrome window pointing at a fresh copy of
// the user's Chrome profile so the session cookies authenticate every Google
// request without OAuth. Each step has accessibility-label / text-based
// selectors so it survives minor DOM renames; severe UI changes surface as a
// clear error rather than silent hangs.
// DebugPort is the CDP port the OAuth-launch side opens on Chrome. Kept
// here (and duplicated in main.go as internkimChromeDebugPort) so the two
// sides stay wired to the same number.
const DebugPort = 9335

func DeployAppsScriptViaBrowser(codeGs, manifest string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Attach to the Chrome process launched earlier in googleAuth() — it
	// already opened on ~/.internkim/chrome-profile with CDP exposed on
	// DebugPort, and the user has signed in to Google in that window. A
	// fresh headless launch can't reuse the profile (Chrome locks it) and
	// misses the sensitive-scope warm-up the visible sign-in produced, so
	// remote-attach is the only option.
	debugAddress := fmt.Sprintf("localhost:%d", DebugPort)
	if err := waitForDebugPort(debugAddress, 20*time.Second); err != nil {
		return "", fmt.Errorf("chrome CDP not reachable at %s: %w (did googleAuth run first?)", debugAddress, err)
	}
	existingTargetID, err := findExistingGoogleTab()
	if err != nil {
		return "", fmt.Errorf("list Chrome tabs: %w", err)
	}
	if existingTargetID == "" {
		return "", errors.New("no google.com tab found in Chrome — sign-in callback may have failed")
	}

	allocatorContext, cancelAllocator := chromedp.NewRemoteAllocator(
		context.Background(),
		fmt.Sprintf("http://%s", debugAddress),
	)
	defer cancelAllocator()

	browserContext, cancelBrowser := chromedp.NewContext(
		allocatorContext,
		chromedp.WithTargetID(target.ID(existingTargetID)),
	)
	defer cancelBrowser()

	runContext, cancelRun := context.WithTimeout(browserContext, 15*time.Minute)
	defer cancelRun()

	// Shrink the Chrome window to a small corner position so the user
	// can see automation is happening but can't accidentally click on
	// dialogs chromedp is driving. We can't fully minimize because CDP
	// Input.dispatchMouseEvent (used for trusted clicks that open the
	// Authorize-access popup) requires the window to be in a painted
	// state — a minimized window silently drops those events. Use
	// Emulation.setDeviceMetricsOverride so the page's layout viewport
	// stays at 1280x800 even though the native window is 400x260, so
	// the Apps Script dialog and its buttons render at normal positions
	// for CDP's coordinate-based clicks.
	if err := chromedp.Run(runContext, chromedp.ActionFunc(func(ctx context.Context) error {
		windowID, _, err := browser.GetWindowForTarget().WithTargetID(target.ID(existingTargetID)).Do(ctx)
		if err != nil {
			return err
		}
		if err := browser.SetWindowBounds(windowID, &browser.Bounds{
			Left:        20,
			Top:         20,
			Width:       400,
			Height:      260,
			WindowState: browser.WindowStateNormal,
		}).Do(ctx); err != nil {
			return err
		}
		return emulation.SetDeviceMetricsOverride(1280, 800, 1, false).Do(ctx)
	})); err != nil {
		fmt.Printf("  (could not resize Chrome window: %v — continuing)\n", err)
	}

	notifyUser(
		"internkim 자동 설정 중",
		"Apps Script 배포를 자동으로 진행하고 있어요. 브라우저 창은 건드리지 않아도 됩니다.",
	)

	fmt.Println("  Driving Apps Script UI in your signed-in Chrome...")

	// If a previous run left a project behind, remove it first so the
	// user's Drive does not pile up duplicate Apps Script projects on
	// every re-run. Best-effort — failures don't block the new create.
	scriptIDPath := filepath.Join(home, ".internkim", "gas-script-id")
	if savedScriptID, readError := os.ReadFile(scriptIDPath); readError == nil {
		stale := strings.TrimSpace(string(savedScriptID))
		if stale != "" {
			fmt.Printf("  Removing previous Apps Script project %s...\n", stale)
			if err := trashAppsScriptProject(runContext, stale); err != nil {
				fmt.Printf("  (removal failed: %v — continuing)\n", err)
			}
			_ = os.Remove(scriptIDPath)
		}
	}

	// hl=en forces the Apps Script UI into English regardless of the user's
	// Google-account language preference. Our clickByText selectors target
	// English labels ("Deploy", "New deployment", "Web app", "Anyone"), so
	// this keeps automation stable across locales.
	if err := chromedp.Run(runContext,
		chromedp.Navigate("https://script.google.com/home/projects/create?hl=en"),
	); err != nil {
		return "", fmt.Errorf("navigate: %w", err)
	}

	if err := waitForAppsScriptEditor(runContext); err != nil {
		return "", err
	}

	// Remember the new project's scriptId so the next run's cleanup can
	// target it.
	var currentURL string
	if err := chromedp.Run(runContext, chromedp.Location(&currentURL)); err == nil {
		if scriptID := extractScriptID(currentURL); scriptID != "" {
			_ = os.WriteFile(scriptIDPath, []byte(scriptID+"\n"), 0o600)
		}
	}

	if err := chromedp.Run(runContext,
		setMonacoValue(codeGs),
		saveEditor(),
		clickByTextDeep("Deploy"),
		waitForTextDeep("New deployment", 10*time.Second),
		clickByTextDeep("New deployment"),
		waitForTextDeep("Please select a deployment type", 15*time.Second),
		selectDeploymentType("Web app"),
		waitForTextDeep("Who has access", 15*time.Second),
		fillDialogField("Description", "internkim-bridge"),
		setWebAppAccess("Anyone"),
		clickDialogButton("Deploy"),
	); err != nil {
		saveFailureScreenshot(runContext, "ui-automation")
		return "", fmt.Errorf("UI automation: %w", err)
	}

	// Apps Script prompts for consent the first time a deployment needs the
	// declared scopes. Drive the consent popup (choose account → 'Advanced'
	// → 'Go to internkim-bridge (unsafe)' → 'Allow') automatically.
	if err := authorizeAccessFlow(browserContext, runContext); err != nil {
		saveFailureScreenshot(runContext, "authorize-access")
		return "", fmt.Errorf("authorize access: %w", err)
	}

	var webAppURL string
	if err := chromedp.Run(runContext,
		waitForWebAppURL(&webAppURL),
	); err != nil {
		saveFailureScreenshot(runContext, "wait-url")
		return "", fmt.Errorf("read deployment URL: %w", err)
	}
	if webAppURL == "" {
		return "", errors.New("deployment URL was empty")
	}
	return webAppURL, nil
}

// waitForEditor blocks until the Apps Script editor's code pane is visible.
func waitForEditor() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.Run(ctx,
			chromedp.WaitVisible(`div[role="code"], .monaco-editor`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second),
		)
	})
}

// setMonacoValue replaces the contents of the currently-focused Monaco
// editor model with newContent via the Apps Script editor's in-page
// Monaco API. This bypasses keyboard typing (which fights Monaco's
// auto-indent) and cleanly wipes whatever boilerplate Apps Script's
// "new project" flow inserted.
func setMonacoValue(newContent string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := fmt.Sprintf(`
			(() => {
				if (!window.monaco || !monaco.editor) return "no-monaco";
				const editors = monaco.editor.getEditors ? monaco.editor.getEditors() : [];
				if (editors.length === 0) return "no-editors";
				editors[0].setValue(%s);
				editors[0].focus();
				return "set";
			})()
		`, jsString(newContent))
		var outcome string
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &outcome)); err != nil {
			return err
		}
		if outcome != "set" {
			return fmt.Errorf("setMonacoValue: %s (Apps Script editor likely still loading or Monaco API absent)", outcome)
		}
		return chromedp.Sleep(500 * time.Millisecond).Do(ctx)
	})
}

// saveEditor sends Cmd+S and waits for Apps Script's save indicator to
// settle so the Deploy button becomes enabled.
func saveEditor() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.Run(ctx,
			chromedp.KeyEvent("\x13"), // Ctrl/Cmd+S
			chromedp.Sleep(2*time.Second),
		)
	})
}

// clickByTextDeep finds an element whose visible text (or aria-label)
// matches label and clicks it. Walks both the normal DOM and shadow
// roots. Prefers elements with a clickable role but falls back to
// clicking the smallest visible text-bearing match when no clickable
// ancestor is found — enough for material-web items whose <span>s
// delegate their own click handler.
func clickByTextDeep(label string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := fmt.Sprintf(`
			(() => {
				const needle = %s.toLowerCase().trim();

				function* walk(root) {
					const queue = [root];
					while (queue.length) {
						const node = queue.shift();
						if (!node) continue;
						if (node.nodeType === 1) yield node;
						if (node.shadowRoot) queue.push(node.shadowRoot);
						for (const child of node.children || []) queue.push(child);
					}
				}

				const visible = (el) => {
					if (el.disabled) return false;
					if (el.offsetParent === null && el.getClientRects().length === 0) return false;
					return true;
				};
				const textMatches = (el) => {
					const text = (el.innerText || el.textContent || '').trim().toLowerCase();
					const aria = (el.getAttribute && (el.getAttribute('aria-label') || '')).trim().toLowerCase();
					return (text === needle || aria === needle || text.includes(needle) || aria.includes(needle));
				};

				const matches = [];
				for (const el of walk(document)) {
					if (textMatches(el) && visible(el)) matches.push(el);
				}
				if (!matches.length) return "nothing-matches";

				// Prefer exact-text matches first ('Anyone' beats 'Anyone
				// with Google account'), then prefix matches, then contains.
				matches.sort((a, b) => {
					const aText = (a.innerText || a.textContent || '').trim().toLowerCase();
					const bText = (b.innerText || b.textContent || '').trim().toLowerCase();
					const score = (t) => t === needle ? 0 : t.startsWith(needle) ? 1 : 2;
					return score(aText) - score(bText);
				});

				// Prefer an ancestor that's an actual button / menuitem / link.
				const clickableRoles = 'button, [role="button"], [role="menuitem"], [role="option"], [role="tab"], a, md-menu-item, md-outlined-button, md-filled-button, md-select-option';
				const withAncestor = matches
					.map(el => el.closest(clickableRoles))
					.find(el => el && visible(el));
				const target = withAncestor || matches.reduce((best, el) => {
					if (!best) return el;
					let depthBest = 0, depthEl = 0;
					for (let n = best; n; n = n.parentElement) depthBest++;
					for (let n = el; n; n = n.parentElement) depthEl++;
					return depthEl > depthBest ? el : best;
				}, null);

				target.scrollIntoView({ block: 'center' });
				const rect = target.getBoundingClientRect();
				const opts = {
					bubbles: true,
					cancelable: true,
					view: window,
					clientX: rect.left + rect.width / 2,
					clientY: rect.top + rect.height / 2,
					button: 0,
					buttons: 1,
				};
				// Material-web components listen for pointer events, not
				// just the synthetic click(). Fire the full sequence so
				// every gesture-detecting listener in the component tree
				// agrees the element was activated.
				target.dispatchEvent(new PointerEvent('pointerdown', opts));
				target.dispatchEvent(new MouseEvent('mousedown', opts));
				target.dispatchEvent(new PointerEvent('pointerup', opts));
				target.dispatchEvent(new MouseEvent('mouseup', opts));
				target.dispatchEvent(new MouseEvent('click', opts));
				return withAncestor ? "ancestor" : "direct";
			})()
		`, jsString(label))
		var outcome string
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &outcome)); err != nil {
			return err
		}
		if outcome == "nothing-matches" {
			return fmt.Errorf("could not find element with text %q", label)
		}
		return chromedp.Sleep(900 * time.Millisecond).Do(ctx)
	})
}

// notifyUser shows a brief macOS alert that auto-dismisses after two
// seconds — no OK click required. We intentionally avoid osascript's
// `display notification`: on Big Sur and later, Notification Center
// silently drops it unless the user has granted Script Editor
// notification permission in System Settings, which is off by default.
// `display dialog ... giving up after` works without any permission.
func notifyUser(title, message string) {
	escape := func(value string) string {
		return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`)
	}
	script := fmt.Sprintf(
		`display dialog "%s" with title "%s" buttons {"OK"} default button "OK" giving up after 2`,
		escape(message), escape(title),
	)
	_ = exec.Command("osascript", "-e", script).Start()
}

// jsString renders s as a JavaScript string literal safe to embed inside
// an IIFE passed to chromedp.Evaluate. Uses JSON.stringify semantics
// (Go's encoding/json produces a valid JS string literal for all inputs).
func jsString(s string) string {
	quoted := strings.ReplaceAll(s, "\\", "\\\\")
	quoted = strings.ReplaceAll(quoted, "`", "\\`")
	quoted = strings.ReplaceAll(quoted, "${", "\\${")
	return "`" + quoted + "`"
}

// selectDeploymentType clicks the gear icon in the deploy dialog and picks
// the requested deployment type (e.g. "Web app"). Walks shadow roots to
// find the gear (Apps Script renders it via material-web).
func selectDeploymentType(label string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := `
			(() => {
				function* walk(root) {
					const queue = [root];
					while (queue.length) {
						const node = queue.shift();
						if (!node) continue;
						if (node.nodeType === 1) yield node;
						if (node.shadowRoot) queue.push(node.shadowRoot);
						for (const child of node.children || []) queue.push(child);
					}
				}
				for (const el of walk(document)) {
					const aria = (el.getAttribute && (el.getAttribute('aria-label') || '')).toLowerCase();
					if (/select type|deployment type/.test(aria)) {
						el.click();
						return "clicked:" + aria;
					}
				}
				return "no-gear";
			})()
		`
		var outcome string
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &outcome)); err != nil {
			return err
		}
		if outcome == "no-gear" {
			return fmt.Errorf("selectDeploymentType: could not find 'Select type' gear button")
		}
		if err := chromedp.Run(ctx, chromedp.Sleep(1500*time.Millisecond)); err != nil {
			return err
		}
		return clickByTextDeep(label).Do(ctx)
	})
}

// setWebAppAccess picks the "Who has access" dropdown option. Apps Script
// renders this as a Material Outlined Select — a plain .click() on the
// trigger does not open it; we dispatch the full pointer event sequence
// the same way clickByTextDeep does for menu items.
func setWebAppAccess(option string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := `
			(() => {
				function* walk(root) {
					const queue = [root];
					while (queue.length) {
						const node = queue.shift();
						if (!node) continue;
						if (node.nodeType === 1) yield node;
						if (node.shadowRoot) queue.push(node.shadowRoot);
						for (const child of node.children || []) queue.push(child);
					}
				}
				// Narrow to labels that literally read "Who has access", not
				// merely contain the phrase somewhere in a subtree.
				let label = null;
				for (const el of walk(document)) {
					const own = (el.textContent || '').trim().toLowerCase();
					if (own === 'who has access') { label = el; break; }
				}
				if (!label) return "no-label";

				const field = label.closest('label, [role="group"], md-outlined-select, div, section, form') || label.parentElement;
				if (!field) return "no-field";

				// Prefer a real form-control inside the same field wrapper.
				const candidates = [
					...field.querySelectorAll('md-outlined-select, [role="combobox"], button, [role="button"], .goog-flat-menu-button, .select-trigger')
				];
				let trigger = candidates.find(el => el.offsetParent !== null) || candidates[0];
				if (!trigger) {
					// Last-ditch: whichever descendant actually displays the
					// current value ("Only myself" / "Anyone").
					trigger = [...walk(field)].find(el => /only myself|anyone/i.test(el.textContent || ''));
				}
				if (!trigger) return "no-trigger";

				trigger.scrollIntoView({ block: 'center' });
				const rect = trigger.getBoundingClientRect();
				const opts = {
					bubbles: true, cancelable: true, view: window,
					clientX: rect.left + rect.width / 2,
					clientY: rect.top + rect.height / 2,
					button: 0, buttons: 1,
				};
				trigger.dispatchEvent(new PointerEvent('pointerdown', opts));
				trigger.dispatchEvent(new MouseEvent('mousedown', opts));
				trigger.dispatchEvent(new PointerEvent('pointerup', opts));
				trigger.dispatchEvent(new MouseEvent('mouseup', opts));
				trigger.dispatchEvent(new MouseEvent('click', opts));
				return "trigger-clicked";
			})()
		`
		var outcome string
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &outcome)); err != nil {
			return err
		}
		if outcome != "trigger-clicked" {
			return fmt.Errorf("setWebAppAccess trigger: %s", outcome)
		}
		if err := chromedp.Run(ctx, chromedp.Sleep(1500*time.Millisecond)); err != nil {
			return err
		}
		return clickByTextDeep(option).Do(ctx)
	})
}

// waitForWebAppURL polls the page for the Web app URL shown after a
// successful deployment. The "Deployment successful" dialog exposes the URL
// either as an anchor/input or as text matching /macros/s/<id>/exec.
func waitForWebAppURL(out *string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := `
			(() => {
				const anchor = [...document.querySelectorAll('a[href*="/macros/s/"]')][0];
				if (anchor) return anchor.href;
				const input = [...document.querySelectorAll('input, textarea')]
					.find(e => (e.value || '').includes('/macros/s/'));
				if (input) return input.value;
				const text = document.body ? document.body.innerText : '';
				const match = text.match(/https:\/\/script\.google\.com\/macros\/s\/[^\s"']+/);
				return match ? match[0] : '';
			})();
		`
		deadline := time.Now().Add(3 * time.Minute)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var url string
			if err := chromedp.Run(ctx, chromedp.Evaluate(script, &url)); err == nil && strings.Contains(url, "/macros/s/") {
				*out = strings.TrimSpace(url)
				// Strip any trailing characters that commonly get pulled in
				// from surrounding markup (quotes, punctuation).
				*out = strings.TrimRight(*out, `")'.,;`)
				return nil
			}
			if time.Now().After(deadline) {
				return errors.New("timed out waiting for /macros/s/ URL")
			}
			time.Sleep(2 * time.Second)
		}
	})
}

// fillDialogField fills the input/textarea associated with the given
// label inside the currently-open dialog. Walks shadow roots so
// material-web fields are reachable.
func fillDialogField(label, value string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := fmt.Sprintf(`
			(() => {
				function* walk(root) {
					const queue = [root];
					while (queue.length) {
						const node = queue.shift();
						if (!node) continue;
						if (node.nodeType === 1) yield node;
						if (node.shadowRoot) queue.push(node.shadowRoot);
						for (const child of node.children || []) queue.push(child);
					}
				}
				const needle = %s.toLowerCase();

				// Find dialog open at the moment.
				let dialog = null;
				for (const el of walk(document)) {
					if (el.getAttribute && el.getAttribute('role') === 'dialog' && el.offsetParent) {
						dialog = el;
					}
				}
				const scope = dialog || document;

				// Locate an input/textarea near the label text.
				let targetInput = null;
				for (const el of walk(scope)) {
					if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || /md-(outlined|filled)-(text|textarea)/i.test(el.tagName)) {
						const labelText = (el.getAttribute('aria-label') || el.getAttribute('label') || el.getAttribute('placeholder') || '').toLowerCase();
						if (labelText.includes(needle)) { targetInput = el; break; }
					}
				}
				if (!targetInput) {
					// Fallback: find <label> text matching and follow to sibling input.
					for (const el of walk(scope)) {
						const text = (el.textContent || '').toLowerCase().trim();
						if (text === needle || text.includes(needle)) {
							const parent = el.closest('div, section, label, form') || el.parentElement;
							if (!parent) continue;
							const input = [...walk(parent)].find(n => n.tagName === 'INPUT' || n.tagName === 'TEXTAREA');
							if (input) { targetInput = input; break; }
						}
					}
				}
				if (!targetInput) return "no-input";
				targetInput.focus();
				// For material-web text-field, setting .value is not enough —
				// we also need to dispatch an 'input' event so Apps Script's
				// form validation sees the new value.
				const nativeSetter = Object.getOwnPropertyDescriptor(targetInput.__proto__, 'value')?.set;
				if (nativeSetter) nativeSetter.call(targetInput, %s);
				else targetInput.value = %s;
				targetInput.dispatchEvent(new Event('input', { bubbles: true }));
				targetInput.dispatchEvent(new Event('change', { bubbles: true }));
				return "set";
			})()
		`, jsString(label), jsString(value), jsString(value))
		var outcome string
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &outcome)); err != nil {
			return err
		}
		if outcome != "set" {
			// Non-fatal — some deployment types don't require description;
			// log and continue.
			fmt.Printf("  (could not fill %q field: %s)\n", label, outcome)
		}
		return chromedp.Sleep(400 * time.Millisecond).Do(ctx)
	})
}

// clickDialogButton clicks the button whose text matches label that lives
// INSIDE the currently-open dialog. Prevents us from re-clicking the
// editor's top-toolbar Deploy when we meant the dialog's Deploy at the
// final submit step. Uses CDP's Input.dispatchMouseEvent so the click
// carries isTrusted=true — required for window.open() popups (e.g. the
// Authorize access OAuth window) to survive Chrome's popup-blocker
// heuristics.
func clickDialogButton(label string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := fmt.Sprintf(`
			(() => {
				const needle = %s.toLowerCase().trim();
				function* walk(root) {
					const queue = [root];
					while (queue.length) {
						const node = queue.shift();
						if (!node) continue;
						if (node.nodeType === 1) yield node;
						if (node.shadowRoot) queue.push(node.shadowRoot);
						for (const child of node.children || []) queue.push(child);
					}
				}
				let dialog = null;
				for (const el of walk(document)) {
					if (el.getAttribute && el.getAttribute('role') === 'dialog' && el.offsetParent) {
						dialog = el;
					}
				}
				if (!dialog) return {error: "no-dialog"};

				const clickableSelector = 'button, [role="button"], md-outlined-button, md-filled-button, md-text-button, md-filled-tonal-button';
				const buttons = [...walk(dialog)].filter(el => el.matches && el.matches(clickableSelector));
				const candidates = buttons.filter(el => {
					if (el.disabled || el.getAttribute('aria-disabled') === 'true') return false;
					if (el.offsetParent === null && el.getClientRects().length === 0) return false;
					const text = (el.innerText || el.textContent || '').trim().toLowerCase();
					const aria = (el.getAttribute('aria-label') || '').trim().toLowerCase();
					return text === needle || aria === needle || text.includes(needle) || aria.includes(needle);
				});
				candidates.sort((a, b) => b.getBoundingClientRect().top - a.getBoundingClientRect().top);
				const target = candidates[0];
				if (!target) return {error: "not-found"};

				target.scrollIntoView({ block: 'center' });
				const rect = target.getBoundingClientRect();
				return {
					x: rect.left + rect.width / 2,
					y: rect.top + rect.height / 2,
				};
			})()
		`, jsString(label))
		var result struct {
			Error string  `json:"error"`
			X     float64 `json:"x"`
			Y     float64 `json:"y"`
		}
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &result)); err != nil {
			return err
		}
		if result.Error != "" {
			return fmt.Errorf("clickDialogButton %q: %s", label, result.Error)
		}
		// MouseClickXY dispatches via CDP Input.dispatchMouseEvent, which
		// produces a trusted event; window.open() called from the button's
		// click handler is therefore allowed (synthetic dispatchEvent would
		// hit the popup-blocker).
		return chromedp.Run(ctx,
			chromedp.MouseClickXY(result.X, result.Y),
			chromedp.Sleep(1200*time.Millisecond),
		)
	})
}

// trashAppsScriptProject moves a previously-deployed Apps Script project
// to the user's Drive trash via the script.google.com home UI. The home
// page renders a project list; each row has a "More actions" (kebab)
// button that opens a menu with "Remove" as one of the entries. We drive
// that menu via chromedp.
func trashAppsScriptProject(ctx context.Context, scriptID string) error {
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://script.google.com/home?hl=en"),
	); err != nil {
		return fmt.Errorf("navigate home: %w", err)
	}
	// Wait for the project list to render — any row will do as a sentinel.
	if err := chromedp.Run(ctx, waitForTextDeep("My projects", 15*time.Second)); err != nil {
		return fmt.Errorf("home did not render: %w", err)
	}

	// The row is an <a> / <tr> carrying the scriptId in its href or data-id.
	// Click its kebab, then the "Remove" menuitem, then the confirmation.
	clickKebab := fmt.Sprintf(`
		(() => {
			const id = %s;
			function* walk(root) {
				const queue = [root];
				while (queue.length) {
					const node = queue.shift();
					if (!node) continue;
					if (node.nodeType === 1) yield node;
					if (node.shadowRoot) queue.push(node.shadowRoot);
					for (const child of node.children || []) queue.push(child);
				}
			}
			let row = null;
			for (const el of walk(document)) {
				const href = (el.getAttribute && el.getAttribute('href')) || '';
				if (href.includes('/projects/' + id + '/')) { row = el; break; }
			}
			if (!row) return "no-row";
			const container = row.closest('[role="row"], tr, .project-row, .RrEUbf') || row.parentElement;
			if (!container) return "no-container";
			const kebab = [...walk(container)].find(el => /more actions|more|options/i.test(
				(el.getAttribute && (el.getAttribute('aria-label') || '')) || ''
			));
			if (!kebab) return "no-kebab";
			kebab.scrollIntoView({ block: 'center' });
			const rect = kebab.getBoundingClientRect();
			return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
		})()
	`, jsString(scriptID))
	var kebabResult struct {
		Error string  `json:"error"`
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(clickKebab, &kebabResult)); err != nil {
		return err
	}
	// Some responses come back as a bare string if the expression returned
	// a plain value; we only care whether coords are valid.
	if kebabResult.X == 0 && kebabResult.Y == 0 {
		return fmt.Errorf("could not locate More-actions button for %s", scriptID)
	}
	if err := chromedp.Run(ctx, chromedp.MouseClickXY(kebabResult.X, kebabResult.Y)); err != nil {
		return err
	}
	time.Sleep(700 * time.Millisecond)

	if err := chromedp.Run(ctx, clickByTextDeep("Remove")); err != nil {
		return fmt.Errorf("click Remove menuitem: %w", err)
	}
	time.Sleep(700 * time.Millisecond)

	// Confirmation dialog — the confirm button is usually labeled "Remove"
	// again or "OK". Try Remove first.
	if err := chromedp.Run(ctx, clickDialogButton("Remove")); err != nil {
		if err := chromedp.Run(ctx, clickDialogButton("OK")); err != nil {
			// No confirmation dialog present — some Apps Script builds
			// remove immediately after the menu click.
		}
	}
	time.Sleep(1 * time.Second)
	return nil
}

// extractScriptID pulls the project scriptId out of an Apps Script editor
// URL like https://script.google.com/home/projects/<id>/edit. Returns ""
// when the URL does not match — we treat that as "don't cache anything".
func extractScriptID(rawURL string) string {
	const marker = "/home/projects/"
	index := strings.Index(rawURL, marker)
	if index < 0 {
		return ""
	}
	tail := rawURL[index+len(marker):]
	end := strings.IndexAny(tail, "/?#")
	if end < 0 {
		return tail
	}
	return tail[:end]
}

// waitForTextDeep polls the rendered page (including shadow roots) for
// any element whose visible text or aria-label contains label. Returns
// when the element appears, or errors after timeout. Use this to guard
// the transition between UI states so clickByTextDeep never targets a
// half-rendered dialog.
func waitForTextDeep(label string, timeout time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := fmt.Sprintf(`
			(() => {
				const needle = %s.toLowerCase();
				function* walk(root) {
					const queue = [root];
					while (queue.length) {
						const node = queue.shift();
						if (!node) continue;
						if (node.nodeType === 1) yield node;
						if (node.shadowRoot) queue.push(node.shadowRoot);
						for (const child of node.children || []) queue.push(child);
					}
				}
				for (const el of walk(document)) {
					const text = (el.innerText || el.textContent || '').toLowerCase();
					const aria = (el.getAttribute && (el.getAttribute('aria-label') || '')).toLowerCase();
					if (text.includes(needle) || aria.includes(needle)) {
						if (el.offsetParent !== null || el.getClientRects().length > 0) return true;
					}
				}
				return false;
			})()
		`, jsString(label))
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var found bool
			if err := chromedp.Run(ctx, chromedp.Evaluate(script, &found)); err == nil && found {
				return nil
			}
			time.Sleep(300 * time.Millisecond)
		}
		return fmt.Errorf("text %q did not appear within %s", label, timeout)
	})
}

// checkAllConsentScopes ticks every unchecked permission checkbox on the
// OAuth consent page so that clicking Continue / Allow actually grants the
// scopes. Google's Granular Consent rollout means checkboxes default to
// unchecked; submitting empty triggers the "You did not allow any access"
// modal and leaves the deployment unauthorized.
func checkAllConsentScopes() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := `
			(() => {
				const boxes = [
					...document.querySelectorAll('[role="checkbox"]'),
					...document.querySelectorAll('input[type="checkbox"]'),
				];
				let clicked = 0;
				for (const box of boxes) {
					const state = box.getAttribute('aria-checked');
					const isChecked = box.checked || state === 'true';
					if (isChecked) continue;
					if (box.disabled || box.getAttribute('aria-disabled') === 'true') continue;
					if (box.offsetParent === null && box.getClientRects().length === 0) continue;
					box.click();
					clicked++;
				}
				return clicked;
			})()
		`
		var clicked int
		return chromedp.Run(ctx, chromedp.Evaluate(script, &clicked))
	})
}

// findExistingGoogleTab queries Chrome's devtools /json/list endpoint and
// returns the first page target whose URL is already on a google.com host.
// Returns "" (without error) if no matching target exists yet — the caller
// will fall back to opening a new tab.
func findExistingGoogleTab() (string, error) {
	endpoint := fmt.Sprintf("http://localhost:%d/json/list", DebugPort)
	response, err := http.Get(endpoint)
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
	// Prefer a tab already on script.google.com; otherwise any google.com
	// page. Skip about:blank / devtools pages / service workers.
	var best string
	for _, info := range targets {
		if info.Type != "page" {
			continue
		}
		if strings.Contains(info.URL, "script.google.com") {
			return info.ID, nil
		}
		if strings.Contains(info.URL, "google.com") && best == "" {
			best = info.ID
		}
	}
	return best, nil
}

// authorizeAccessFlow clicks the "Authorize access" button in the open
// New deployment dialog, waits for the OAuth consent popup to appear,
// drives account selection + the "unverified app" advanced-continue
// detour + the final Allow click, and waits for the popup to close.
// On return, the original dialog in the main tab will show the web app
// URL.
func authorizeAccessFlow(browserContext, runContext context.Context) error {
	// Apps Script takes a beat to swap the dialog contents from the
	// deployment form to the "Authorize access" state after Deploy is
	// clicked. Poll until the button text shows up.
	if err := chromedp.Run(runContext, waitForTextDeep("Authorize access", 20*time.Second)); err != nil {
		return fmt.Errorf("wait Authorize access button: %w", err)
	}

	// Catch any new page target that opens after clicking. At creation the
	// URL may still be 'about:blank'; don't filter by URL here or we'll
	// miss the popup.
	popupCh := chromedp.WaitNewTarget(browserContext, func(info *target.Info) bool {
		return info.Type == "page"
	})

	if err := chromedp.Run(runContext, clickDialogButton("Authorize access")); err != nil {
		return fmt.Errorf("click Authorize access: %w", err)
	}

	var popupTargetID target.ID
	select {
	case popupTargetID = <-popupCh:
	case <-time.After(15 * time.Second):
		return errors.New("Authorize-access popup did not open")
	case <-runContext.Done():
		return runContext.Err()
	}

	popupContext, cancelPopup := chromedp.NewContext(browserContext, chromedp.WithTargetID(popupTargetID))
	defer cancelPopup()
	popupRun, cancelRun := context.WithTimeout(popupContext, 3*time.Minute)
	defer cancelRun()

	// 1. Account picker — click the first visible email row.
	_ = chromedp.Run(popupRun, clickByTextDeep("@"))
	time.Sleep(2 * time.Second)

	// 2. "This app isn't verified" warning.
	//    Click "Advanced", then "Go to internkim-bridge (unsafe)".
	_ = chromedp.Run(popupRun, clickByTextDeep("Advanced"))
	time.Sleep(1500 * time.Millisecond)
	_ = chromedp.Run(popupRun, clickByTextDeep("Go to"))
	time.Sleep(2 * time.Second)

	// 3. Scope consent page — Google's Granular Consent shows a checkbox
	//    per scope, defaulting to unchecked. Continue without checking any
	//    leads to a "You did not allow any access" modal. Check every
	//    unchecked checkbox first.
	if err := chromedp.Run(popupRun, checkAllConsentScopes()); err != nil {
		return fmt.Errorf("tick consent checkboxes: %w", err)
	}
	time.Sleep(1 * time.Second)

	// 4. Final Continue / Allow.
	if err := chromedp.Run(popupRun, clickByTextDeep("Continue")); err != nil {
		if err := chromedp.Run(popupRun, clickByTextDeep("Allow")); err != nil {
			return fmt.Errorf("click final consent: %w", err)
		}
	}

	// 4. Popup should close shortly after Allow. Wait up to 30s.
	closed := false
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		targets, err := chromedp.Targets(browserContext)
		if err == nil {
			found := false
			for _, t := range targets {
				if t.TargetID == popupTargetID {
					found = true
					break
				}
			}
			if !found {
				closed = true
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !closed {
		return errors.New("consent popup did not close after Allow")
	}
	return nil
}

// saveFailureScreenshot grabs the current Chrome viewport + a HTML dump
// and writes them to ~/.internkim/debug-screenshots/<stage>-<ts>.{png,html}.
// Best-effort — any failure here is silently swallowed so the caller's
// original error is still what bubbles up to the user. The HTML dump
// walks shadow roots so custom-element contents (material-web) are
// included.
func saveFailureScreenshot(ctx context.Context, stage string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".internkim", "debug-screenshots")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	timestamp := time.Now().Format("20060102-150405")

	// Screenshot
	screenshotCtx, cancelScreenshot := context.WithTimeout(ctx, 10*time.Second)
	var imageBytes []byte
	_ = chromedp.Run(screenshotCtx, chromedp.CaptureScreenshot(&imageBytes))
	cancelScreenshot()
	if len(imageBytes) > 0 {
		pngPath := filepath.Join(dir, fmt.Sprintf("%s-%s.png", stage, timestamp))
		if err := os.WriteFile(pngPath, imageBytes, 0o600); err == nil {
			fmt.Printf("  Saved failure screenshot → %s\n", pngPath)
		}
	}

	// HTML dump (with shadow roots expanded).
	htmlCtx, cancelHTML := context.WithTimeout(ctx, 10*time.Second)
	var html string
	_ = chromedp.Run(htmlCtx, chromedp.Evaluate(`
		(() => {
			function serialize(node, indent) {
				const pad = '  '.repeat(indent);
				if (node.nodeType === 3) {
					const text = node.textContent.trim();
					return text ? pad + text : '';
				}
				if (node.nodeType !== 1) return '';
				const tag = node.tagName.toLowerCase();
				const attrs = [...node.attributes || []]
					.map(a => a.name + '="' + a.value.replace(/"/g, '&quot;') + '"')
					.join(' ');
				const open = pad + '<' + tag + (attrs ? ' ' + attrs : '') + '>';
				if (node.shadowRoot) {
					const shadowChildren = [...node.shadowRoot.childNodes]
						.map(c => serialize(c, indent + 1))
						.filter(s => s)
						.join('\n');
					const lightChildren = [...node.childNodes]
						.map(c => serialize(c, indent + 1))
						.filter(s => s)
						.join('\n');
					return open + '\n' + pad + '  #shadow-root\n' + shadowChildren
						+ (lightChildren ? '\n' + lightChildren : '')
						+ '\n' + pad + '</' + tag + '>';
				}
				const children = [...node.childNodes]
					.map(c => serialize(c, indent + 1))
					.filter(s => s)
					.join('\n');
				if (!children) return open + '</' + tag + '>';
				return open + '\n' + children + '\n' + pad + '</' + tag + '>';
			}
			return serialize(document.documentElement, 0);
		})()
	`, &html))
	cancelHTML()
	if html != "" {
		htmlPath := filepath.Join(dir, fmt.Sprintf("%s-%s.html", stage, timestamp))
		if err := os.WriteFile(htmlPath, []byte(html), 0o600); err == nil {
			fmt.Printf("  Saved failure HTML dump → %s\n", htmlPath)
		}
	}
}

// waitForDebugPort blocks until Chrome's CDP endpoint accepts TCP
// connections on addr, or the timeout elapses.
func waitForDebugPort(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err == nil {
			connection.Close()
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("no response at %s within %s", address, timeout)
}

// waitForAppsScriptEditor polls the page until the Apps Script editor is
// loaded. If Google redirects to a sign-in screen (no session cookie in the
// dedicated profile yet), prints instructions and waits up to 10 min for the
// user to finish signing in.
func waitForAppsScriptEditor(ctx context.Context) error {
	fmt.Println("  Waiting for the Apps Script editor to load...")
	deadline := time.Now().Add(10 * time.Minute)
	notifiedSignIn := false
	for {
		if time.Now().After(deadline) {
			return errors.New("timed out waiting for Apps Script editor")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var currentURL, editorSelector string
		_ = chromedp.Run(ctx,
			chromedp.Location(&currentURL),
			chromedp.Evaluate(`
				(() => {
					const el = document.querySelector('div[role="code"], .monaco-editor');
					return el ? 'ready' : '';
				})()
			`, &editorSelector),
		)
		if editorSelector == "ready" {
			fmt.Println("  Editor ready.")
			return nil
		}
		if strings.Contains(currentURL, "accounts.google.com") {
			if !notifiedSignIn {
				fmt.Println()
				fmt.Println("  The Chrome window needs you to sign in to Google.")
				fmt.Println("  Click through the sign-in screens in the window that opened.")
				fmt.Println("  Once done, the editor will load automatically (no action in this terminal).")
				fmt.Println()
				notifiedSignIn = true
			}
		}
		time.Sleep(2 * time.Second)
	}
}
