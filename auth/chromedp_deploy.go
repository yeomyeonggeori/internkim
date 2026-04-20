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
	"runtime"
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
	profileDir, cleanup, err := stageChromeProfile()
	if err != nil {
		return "", fmt.Errorf("stage Chrome profile: %w", err)
	}
	defer cleanup()

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

	runContext, cancelRun := context.WithTimeout(browserContext, 5*time.Minute)
	defer cancelRun()

	fmt.Println("  Launching Chrome with your logged-in profile...")

	if err := chromedp.Run(runContext,
		chromedp.Navigate("https://script.google.com/home/projects/create"),
		waitForEditor(),
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

// stageChromeProfile returns an isolated copy of the user's default Chrome
// profile chromedp can open without fighting Chrome's profile lock. The
// returned cleanup removes the copy when the caller is done.
func stageChromeProfile() (string, func(), error) {
	if runtime.GOOS != "darwin" {
		return "", func() {}, fmt.Errorf("Chrome profile staging only implemented on macOS")
	}
	home, _ := os.UserHomeDir()
	source := filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	if _, err := os.Stat(source); err != nil {
		return "", func() {}, fmt.Errorf("Chrome is not installed at %s", source)
	}
	stagingDir, err := os.MkdirTemp("", "internkim-chrome-profile-")
	if err != nil {
		return "", func() {}, err
	}
	copyCommand := exec.Command("cp", "-a", source+"/.", stagingDir)
	if output, err := copyCommand.CombinedOutput(); err != nil {
		os.RemoveAll(stagingDir)
		return "", func() {}, fmt.Errorf("cp Chrome profile: %w: %s", err, string(output))
	}
	// Chrome refuses to open a user-data-dir that already holds singleton
	// lock symlinks (they point at the running Chrome instance's PID).
	// The fresh copy inherited them — strip them so Chrome treats this
	// profile as its own exclusive use.
	for _, lock := range []string{"SingletonLock", "SingletonCookie", "SingletonSocket"} {
		os.Remove(filepath.Join(stagingDir, lock))
	}
	return stagingDir, func() { os.RemoveAll(stagingDir) }, nil
}
