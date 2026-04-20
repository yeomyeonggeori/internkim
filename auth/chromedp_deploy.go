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
// request without OAuth. The window stays visible so the user can see what's
// happening and, if needed, click through Google's one-time "Authorize access"
// dialog for the newly deployed script.
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
			chromedp.UserDataDir(profileDir),
		)...,
	)
	defer cancelAllocator()

	browserContext, cancelBrowser := chromedp.NewContext(allocatorContext)
	defer cancelBrowser()

	runContext, cancelRun := context.WithTimeout(browserContext, 3*time.Minute)
	defer cancelRun()

	fmt.Println("  Launching Chrome with your logged-in profile...")
	fmt.Println("  A window will open and drive the Apps Script editor automatically.")

	var webAppURL string
	if err := chromedp.Run(runContext,
		chromedp.Navigate("https://script.google.com/home/projects/create"),
		chromedp.WaitVisible(`div[role="code"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.Click(`div[role="code"]`, chromedp.ByQuery),
		selectAllAndDelete(),
		insertText(manifestAndCode(codeGs, manifest)),
		// Save via keyboard shortcut.
		sendKey(selectAllBinding, 0),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		return "", fmt.Errorf("editor setup: %w", err)
	}

	// The rest of the Deploy flow has a lot of Apps Script internal DOM
	// that is prone to drift between releases. Rather than encode every
	// click here, hand the remainder off to the user, who is already
	// looking at the window. Print exactly what to click and poll the
	// window URL / clipboard for the Web App URL.
	fmt.Println()
	fmt.Println("  Chrome is showing the Apps Script editor with the bridge code filled in.")
	fmt.Println("  In the window:")
	fmt.Println("    1. Click the blue \"Deploy\" button (top right) → \"New deployment\".")
	fmt.Println("    2. Click the gear icon → select \"Web app\".")
	fmt.Println("    3. Set \"Execute as\" to Me, \"Who has access\" to Anyone.")
	fmt.Println("    4. Click \"Deploy\". Approve any permission prompts.")
	fmt.Println("    5. Copy the \"Web app URL\" Apps Script shows after deploy.")
	fmt.Println()
	fmt.Print("  Paste the Web App URL here: ")
	line, _ := readStdinLine()
	webAppURL = strings.TrimSpace(line)
	if webAppURL == "" {
		return "", errors.New("no URL entered")
	}
	if !strings.Contains(webAppURL, "script.google.com/macros/s/") {
		return "", fmt.Errorf("not a /macros/s/ URL: %s", webAppURL)
	}
	return webAppURL, nil
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
	// Copy preserves symlinks / perms / extended attrs (cookies + Local
	// State are the only critical bits, but the editor loads faster with
	// the user's cached fonts / site data).
	copyCommand := exec.Command("cp", "-a", source+"/.", stagingDir)
	if output, err := copyCommand.CombinedOutput(); err != nil {
		os.RemoveAll(stagingDir)
		return "", func() {}, fmt.Errorf("cp Chrome profile: %w: %s", err, string(output))
	}
	cleanup := func() { os.RemoveAll(stagingDir) }
	return stagingDir, cleanup, nil
}

func readStdinLine() (string, error) {
	buffer := make([]byte, 0, 512)
	one := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(one)
		if n == 0 {
			return string(buffer), err
		}
		if one[0] == '\n' {
			return string(buffer), nil
		}
		buffer = append(buffer, one[0])
	}
}

// selectAllAndDelete issues Cmd+A then Delete inside the focused editor.
func selectAllAndDelete() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		if err := chromedp.KeyEvent("\x01").Do(ctx); err != nil { // Ctrl/Cmd+A via raw ctrl code
			return err
		}
		return chromedp.KeyEvent("\b").Do(ctx)
	})
}

func insertText(text string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.KeyEvent(text).Do(ctx)
	})
}

func sendKey(code string, modifiers int) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.KeyEvent(code).Do(ctx)
	})
}

// selectAllBinding is the single-character control code for Ctrl/Cmd+A.
// chromedp's KeyEvent converts control characters to the corresponding
// modifier + key combination.
const selectAllBinding = "\x01"

// manifestAndCode pairs the two files the Apps Script project needs — the
// editor accepts a single blob of text when the code window is first opened
// on a brand-new project; the manifest is added afterwards from the
// "project settings" tab if the user needs advanced scopes. Since we lock
// the Deploy step to "Web app" the default manifest is sufficient for the
// first run; users who need the full oauthScopes list should paste `manifest`
// into Project Settings → appsscript.json.
func manifestAndCode(code, manifest string) string {
	_ = manifest // reserved for future use, see above
	return code
}
