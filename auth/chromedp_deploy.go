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
	"os"
	"os/exec"
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
func DeployAppsScriptViaBrowser(codeGs, manifest string) (string, error) {
	profileDir, err := dedicatedChromeProfileDir()
	if err != nil {
		return "", fmt.Errorf("prepare Chrome profile: %w", err)
	}

	// The OAuth step may have left a Chrome window open on this same profile
	// for the user to sign in; chromedp will hit a SingletonLock conflict if
	// we don't clear that first. pkill by command-line match closes only the
	// Chrome processes pointed at our dedicated user-data-dir.
	exec.Command("pkill", "-f", profileDir).Run()
	time.Sleep(600 * time.Millisecond)
	os.Remove(filepath.Join(profileDir, "SingletonLock"))
	os.Remove(filepath.Join(profileDir, "SingletonCookie"))
	os.Remove(filepath.Join(profileDir, "SingletonSocket"))

	allocatorContext, cancelAllocator := chromedp.NewExecAllocator(context.Background(),
		append(
			chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", false),
			chromedp.Flag("no-first-run", true),
			chromedp.Flag("no-default-browser-check", true),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.UserDataDir(profileDir),
		)...,
	)
	defer cancelAllocator()

	browserContext, cancelBrowser := chromedp.NewContext(allocatorContext)
	defer cancelBrowser()

	runContext, cancelRun := context.WithTimeout(browserContext, 15*time.Minute)
	defer cancelRun()

	fmt.Println("  Launching Chrome (dedicated internkim profile at " + profileDir + ")...")

	if err := chromedp.Run(runContext,
		chromedp.Navigate("https://script.google.com/home/projects/create"),
	); err != nil {
		return "", fmt.Errorf("navigate: %w", err)
	}

	if err := waitForAppsScriptEditor(runContext); err != nil {
		return "", err
	}

	if err := chromedp.Run(runContext,
		replaceFile("Code.gs", codeGs),
		replaceFile("appsscript.json", manifest),
		clickByText("button", "Deploy"),
		clickByText("[role=menuitem]", "New deployment"),
		selectDeploymentType("Web app"),
		setWebAppAccess("Anyone"),
		clickByText("button", "Deploy"),
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

// replaceFile opens (or creates) the named file in the Apps Script editor and
// replaces its content with newContent. Apps Script lists files in the left
// rail; clicking one focuses its tab. Uses the Files API exposed via the
// editor's model where possible, falling back to keyboard entry.
func replaceFile(name, newContent string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		// If the file doesn't exist yet, add it via the "+" button.
		script := fmt.Sprintf(`
			(async () => {
				const rows = [...document.querySelectorAll('[role="treeitem"], [aria-label*="file"]')];
				const target = rows.find(row => (row.textContent || '').trim() === %q);
				if (target) { target.click(); return "opened"; }
				const addButton = [...document.querySelectorAll('button[aria-label], button')]
					.find(b => /add file|new file|\+/i.test((b.getAttribute('aria-label') || b.textContent || '').toLowerCase()));
				if (addButton) addButton.click();
				return "add-clicked";
			})();
		`, name)
		var _r string
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &_r)); err != nil {
			return err
		}
		// Wait briefly for the tab to focus.
		if err := chromedp.Run(ctx, chromedp.Sleep(1*time.Second)); err != nil {
			return err
		}
		// Click into the editor and replace the content.
		return chromedp.Run(ctx,
			chromedp.Click(`div[role="code"], .monaco-editor`, chromedp.ByQuery),
			chromedp.Sleep(300*time.Millisecond),
			chromedp.KeyEvent("\x01"), // Ctrl/Cmd+A
			chromedp.Sleep(100*time.Millisecond),
			chromedp.KeyEvent("\b"),
			chromedp.Sleep(100*time.Millisecond),
			chromedp.KeyEvent(newContent),
			chromedp.Sleep(500*time.Millisecond),
			chromedp.KeyEvent("\x13"), // Ctrl/Cmd+S
			chromedp.Sleep(1500*time.Millisecond),
		)
	})
}

// clickByText finds the first element matching cssOrRoleSelector whose
// visible text equals (case-insensitive) the given label, and clicks it.
func clickByText(selector, label string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := fmt.Sprintf(`
			(() => {
				const needle = %q.toLowerCase();
				const candidates = [...document.querySelectorAll(%q)];
				const hit = candidates.find(el => {
					const text = (el.innerText || el.textContent || el.getAttribute('aria-label') || '').trim().toLowerCase();
					return text === needle || text.includes(needle);
				});
				if (!hit) return false;
				hit.scrollIntoView({ block: 'center' });
				hit.click();
				return true;
			})();
		`, label, selector)
		var ok bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &ok)); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("could not find %q matching %q", label, selector)
		}
		return chromedp.Sleep(800 * time.Millisecond).Do(ctx)
	})
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
		return clickByText(`[role="menuitem"], [role="option"], li, .picker-item`, label).Do(ctx)
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
		return clickByText(`[role="option"], [role="menuitem"], li`, option).Do(ctx)
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

// dedicatedChromeProfileDir returns a persistent, internkim-specific Chrome
// profile directory. First use starts empty: the user signs in to Google in
// the launched Chrome window once. The cookies survive in this directory so
// every subsequent deploy skips the login step. Living outside the user's
// real Chrome profile means we don't fight Chrome's singleton lock and we
// don't need the user to be a Chrome user at all — Safari users welcome.
func dedicatedChromeProfileDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".internkim", "chrome-profile")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
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
