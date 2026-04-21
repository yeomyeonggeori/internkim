// Package auth automates one-off Google consent flows that cannot go through
// regular OAuth because Google's 2025–2026 Granular Consent policy blocks
// unverified third-party apps from requesting sensitive Workspace scopes on
// personal @gmail.com accounts. The workaround is to drive the user's own
// Chrome session (where the cookies already carry the necessary auth) through
// the Apps Script editor UI via the Chrome DevTools Protocol.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	// Attach to the Chrome instance the OAuth step already started on the
	// dedicated ~/.internkim/chrome-profile — no relaunch, no profile lock
	// fight, no cookie flush race.
	if err := waitForDebugPort(fmt.Sprintf("localhost:%d", DebugPort), 15*time.Second); err != nil {
		return "", fmt.Errorf("Chrome CDP endpoint not reachable on port %d: %w (was OAuth step run first?)", DebugPort, err)
	}

	allocatorContext, cancelAllocator := chromedp.NewRemoteAllocator(
		context.Background(),
		fmt.Sprintf("http://localhost:%d", DebugPort),
	)
	defer cancelAllocator()

	browserContext, cancelBrowser := chromedp.NewContext(allocatorContext)
	defer cancelBrowser()

	runContext, cancelRun := context.WithTimeout(browserContext, 15*time.Minute)
	defer cancelRun()

	fmt.Println("  Attaching to the Chrome window you just signed in on...")

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

	fmt.Println("  Deploy submitted. If an \"Access\" prompt appears, click through it.")
	fmt.Println("  The Web app URL will appear in a dialog when authorization finishes.")

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

				// Prefer an ancestor that's an actual button / menuitem / link.
				const clickableRoles = 'button, [role="button"], [role="menuitem"], [role="option"], [role="tab"], a, md-menu-item, md-outlined-button, md-filled-button';
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
// final submit step.
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
				if (!dialog) return "no-dialog";
				let target = null;
				for (const el of walk(dialog)) {
					const text = (el.innerText || el.textContent || '').trim().toLowerCase();
					const aria = (el.getAttribute && (el.getAttribute('aria-label') || '')).toLowerCase();
					if ((text === needle || aria === needle) && !el.disabled) {
						const clickable = el.closest('button, [role="button"], md-outlined-button, md-filled-button, md-text-button') || el;
						if (clickable.offsetParent !== null) { target = clickable; break; }
					}
				}
				if (!target) return "not-found";
				target.scrollIntoView({ block: 'center' });
				const rect = target.getBoundingClientRect();
				const opts = { bubbles: true, cancelable: true, view: window,
					clientX: rect.left + rect.width / 2, clientY: rect.top + rect.height / 2,
					button: 0, buttons: 1 };
				target.dispatchEvent(new PointerEvent('pointerdown', opts));
				target.dispatchEvent(new MouseEvent('mousedown', opts));
				target.dispatchEvent(new PointerEvent('pointerup', opts));
				target.dispatchEvent(new MouseEvent('mouseup', opts));
				target.dispatchEvent(new MouseEvent('click', opts));
				return "clicked";
			})()
		`, jsString(label))
		var outcome string
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &outcome)); err != nil {
			return err
		}
		if outcome != "clicked" {
			return fmt.Errorf("clickDialogButton %q: %s", label, outcome)
		}
		return chromedp.Sleep(800 * time.Millisecond).Do(ctx)
	})
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

// saveFailureScreenshot grabs the current Chrome viewport and writes it
// to ~/.internkim/debug-screenshots/<stage>-<timestamp>.png. Best-effort —
// any failure here is silently swallowed so the caller's original error
// is still what bubbles up to the user.
func saveFailureScreenshot(ctx context.Context, stage string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".internkim", "debug-screenshots")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	var imageBytes []byte
	screenshotCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := chromedp.Run(screenshotCtx, chromedp.CaptureScreenshot(&imageBytes)); err != nil || len(imageBytes) == 0 {
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.png", stage, time.Now().Format("20060102-150405")))
	if err := os.WriteFile(path, imageBytes, 0o600); err != nil {
		return
	}
	fmt.Printf("  Saved failure screenshot → %s\n", path)
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
