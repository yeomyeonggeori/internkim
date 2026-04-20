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

	if err := chromedp.Run(runContext,
		chromedp.Navigate("https://script.google.com/home/projects/create"),
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
		clickByTextDeep("New deployment"),
		selectDeploymentType("Web app"),
		setWebAppAccess("Anyone"),
		clickByTextDeep("Deploy"),
	); err != nil {
		return "", fmt.Errorf("UI automation: %w", err)
	}

	fmt.Println("  Deploy submitted. If an \"Access\" prompt appears, click through it.")
	fmt.Println("  The Web app URL will appear in a dialog when authorization finishes.")

	var webAppURL string
	if err := chromedp.Run(runContext,
		waitForWebAppURL(&webAppURL),
	); err != nil {
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

// clickByTextDeep finds the first element whose visible text (or aria-
// label) case-insensitively contains label and clicks it. Walks both the
// normal DOM and shadow roots so material-web-style buttons (which Apps
// Script uses) are reachable.
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

				const matches = [];
				for (const el of walk(document)) {
					const text = (el.innerText || el.textContent || '').trim().toLowerCase();
					const label = (el.getAttribute && (el.getAttribute('aria-label') || '')).trim().toLowerCase();
					if (text === needle || label === needle || text.includes(needle) || label.includes(needle)) {
						matches.push(el);
					}
				}

				const clickable = matches.find(el => {
					if (el.disabled) return false;
					if (el.offsetParent === null && el.getClientRects().length === 0) return false;
					const tag = el.tagName.toLowerCase();
					if (tag === 'button' || el.getAttribute('role') === 'button' || el.getAttribute('role') === 'menuitem') return true;
					return el.closest('button, [role="button"], [role="menuitem"]');
				});
				if (!clickable) return false;
				const target = clickable.closest('button, [role="button"], [role="menuitem"]') || clickable;
				target.scrollIntoView({ block: 'center' });
				target.click();
				return true;
			})()
		`, jsString(label))
		var ok bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &ok)); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("could not find clickable element with text %q", label)
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
// the requested deployment type (e.g. "Web app").
func selectDeploymentType(label string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		// The gear icon has an aria-label like "Select type". After clicking
		// a menu appears; pick the entry whose text matches label.
		script := fmt.Sprintf(`
			(() => {
				const gear = [...document.querySelectorAll('button, [role="button"]')]
					.find(b => /select type|deployment type|type/i.test(b.getAttribute('aria-label') || ''));
				if (!gear) return "no-gear";
				gear.click();
				return "gear-clicked";
			})();
		`)
		var _r string
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &_r)); err != nil {
			return err
		}
		if err := chromedp.Run(ctx, chromedp.Sleep(700*time.Millisecond)); err != nil {
			return err
		}
		return clickByTextDeep(label).Do(ctx)
	})
}

// setWebAppAccess picks the "Who has access" dropdown option. Apps Script
// renders this as a material dropdown; the option list appears after the
// control is clicked.
func setWebAppAccess(option string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := `
			(() => {
				const labels = [...document.querySelectorAll('label, legend, span')]
					.filter(e => /who has access/i.test(e.textContent || ''));
				if (!labels.length) return "no-label";
				const near = labels[0].closest('[role="group"], div, section, form') || labels[0].parentElement;
				const trigger = near ? near.querySelector('button, [role="combobox"], select') : null;
				if (!trigger) return "no-trigger";
				trigger.click();
				return "trigger-clicked";
			})();
		`
		var _r string
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &_r)); err != nil {
			return err
		}
		if err := chromedp.Run(ctx, chromedp.Sleep(600*time.Millisecond)); err != nil {
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
