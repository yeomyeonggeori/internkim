package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
	"github.com/yeomyeonggeori/internkim/internal/companion/browserextension"
)

func defaultBrowserProfilePath() string {
	configurationDirectory, errorValue := os.UserConfigDir()
	if errorValue == nil && strings.TrimSpace(configurationDirectory) != "" {
		return filepath.Join(configurationDirectory, "internkim", "BrowserProfile")
	}
	homeDirectory, homeError := os.UserHomeDir()
	if homeError == nil && strings.TrimSpace(homeDirectory) != "" {
		return filepath.Join(homeDirectory, ".internkim-companion", "browser-profile")
	}
	return filepath.Join(os.TempDir(), "internkim-companion-browser-profile")
}

func defaultBrowserExecutablePath() string {
	if path := executableBrowserPath(os.Getenv("INTERNKIM_BROWSER_EXECUTABLE_PATH")); path != "" {
		return path
	}
	for _, path := range defaultBrowserExecutableCandidates() {
		if executablePath := executableBrowserPath(path); executablePath != "" {
			return executablePath
		}
	}
	return ""
}

func defaultBrowserExecutableCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			filepath.Join(os.Getenv("HOME"), "Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome"),
		}
	case "windows":
		return []string{
			filepath.Join(os.Getenv("PROGRAMFILES"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
		}
	default:
		candidates := []string{}
		for _, name := range []string{"google-chrome", "google-chrome-stable"} {
			if path, errorValue := exec.LookPath(name); errorValue == nil {
				candidates = append(candidates, path)
			}
		}
		return candidates
	}
}

func executableBrowserPath(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ""
	}
	if isExecutableFile(trimmedPath) {
		return trimmedPath
	}
	return ""
}

// extensionBrowserRuntimeReadiness checks the two local prerequisites for
// browserruntime.ExtensionInputRuntime: a Chrome binary to launch with
// --load-extension, and the unpacked extension directory that flag points
// to. There is no agent-browser doctor/install step to run here (unlike
// AgentBrowserRuntime.EnsureInstalled) since neither piece is a companion-
// managed download.
func extensionBrowserRuntimeReadiness(chromeExecutablePath string, extensionPath string) browserruntime.RuntimeReadiness {
	if !isExecutableFile(chromeExecutablePath) {
		return browserruntime.RuntimeReadiness{Status: "not_ready", Error: "Google Chrome is not installed"}
	}
	if !isDirectory(extensionPath) {
		return browserruntime.RuntimeReadiness{Status: "not_ready", Error: "companion browser extension is not bundled"}
	}
	return browserruntime.RuntimeReadiness{Status: "ready"}
}

func resolveBrowserExtensionPath(flagValue string) string {
	if strings.TrimSpace(flagValue) != "" {
		return strings.TrimSpace(flagValue)
	}
	if environmentValue := strings.TrimSpace(os.Getenv("INTERNKIM_BROWSER_EXTENSION_PATH")); environmentValue != "" {
		return environmentValue
	}
	installed, errorValue := browserextension.Install(defaultBrowserExtensionPath())
	if errorValue != nil {
		return ""
	}
	return installed
}

func defaultBrowserExtensionPath() string {
	return filepath.Join(filepath.Dir(defaultBrowserProfilePath()), "browser-extension")
}

func isExecutableFile(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && !information.IsDir() && information.Mode()&0o111 != 0
}

func isDirectory(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && information.IsDir()
}
