package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
)

func defaultBrowserProfilePath() string {
	configurationDirectory, errorValue := os.UserConfigDir()
	if errorValue == nil && strings.TrimSpace(configurationDirectory) != "" {
		return filepath.Join(configurationDirectory, "InternKim", "BrowserProfile")
	}
	homeDirectory, homeError := os.UserHomeDir()
	if homeError == nil && strings.TrimSpace(homeDirectory) != "" {
		return filepath.Join(homeDirectory, ".internkim-companion", "browser-profile")
	}
	return filepath.Join(os.TempDir(), "internkim-companion-browser-profile")
}

func defaultBrowserExecutablePath() string {
	for _, value := range []string{
		os.Getenv("INTERNKIM_BROWSER_EXECUTABLE_PATH"),
		// AGENT_BROWSER_EXECUTABLE_PATH is an unprefixed legacy fallback kept
		// for any external scripts still setting it; INTERNKIM_BROWSER_EXECUTABLE_PATH
		// is the primary name and takes precedence.
		os.Getenv("AGENT_BROWSER_EXECUTABLE_PATH"),
	} {
		if path := executableBrowserPath(value); path != "" {
			return path
		}
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

// resolveBrowserExtensionPath locates the unpacked companion/browser-extension
// directory. The Tauri shell (companion/src/lib/sidecar.ts) resolves the
// bundled resource path via resolveResource() and always passes it through
// --browser-extension-path, so this fallback chain only matters when the
// companion binary runs outside the Tauri shell (packaged binary invoked
// directly, or a repository-checkout dev run).
func resolveBrowserExtensionPath(flagValue string) string {
	if strings.TrimSpace(flagValue) != "" {
		return strings.TrimSpace(flagValue)
	}
	if environmentValue := strings.TrimSpace(os.Getenv("INTERNKIM_BROWSER_EXTENSION_PATH")); environmentValue != "" {
		return environmentValue
	}
	if bundledPath := bundledBrowserExtensionPath(); bundledPath != "" {
		return bundledPath
	}
	if developmentPath := filepath.Join("companion", "browser-extension"); isDirectory(developmentPath) {
		return developmentPath
	}
	return ""
}

// bundledBrowserExtensionPath checks the packaged-app locations where Tauri's
// bundle.resources places companion/browser-extension: next to the executable
// (Windows NSIS/MSI and Linux AppImage put resources alongside the binary,
// matching how bundledAgentBrowserPath finds sidecars), and one level up under
// Resources/ (the macOS .app bundle keeps Contents/MacOS/<binary> separate
// from Contents/Resources/<resource>).
func bundledBrowserExtensionPath() string {
	executablePath, errorValue := os.Executable()
	if errorValue != nil || strings.TrimSpace(executablePath) == "" {
		return ""
	}
	executableDirectory := filepath.Dir(executablePath)
	candidatePaths := []string{
		filepath.Join(executableDirectory, "browser-extension"),
		filepath.Join(executableDirectory, "..", "Resources", "browser-extension"),
	}
	for _, candidatePath := range candidatePaths {
		if isDirectory(candidatePath) {
			return candidatePath
		}
	}
	return ""
}

// resolveAgentBrowserPath locates the legacy agent-browser CLI. Companion
// browser automation no longer runs through this binary (see
// browserruntime.ExtensionInputRuntime); this only feeds the status-only
// agent-browser CLI check reported by `internkim-companion status`.
func resolveAgentBrowserPath(flagValue string) string {
	if strings.TrimSpace(flagValue) != "" {
		return strings.TrimSpace(flagValue)
	}
	if environmentValue := strings.TrimSpace(os.Getenv("INTERNKIM_AGENT_BROWSER_PATH")); environmentValue != "" {
		return environmentValue
	}
	if bundledPath := bundledAgentBrowserPath(); bundledPath != "" {
		return bundledPath
	}
	if lookupPath, errorValue := exec.LookPath("agent-browser"); errorValue == nil {
		return lookupPath
	}
	return "agent-browser"
}

func bundledAgentBrowserPath() string {
	executablePath, errorValue := os.Executable()
	if errorValue != nil || strings.TrimSpace(executablePath) == "" {
		return ""
	}
	executableDirectory := filepath.Dir(executablePath)
	for _, filename := range bundledAgentBrowserFilenames() {
		path := filepath.Join(executableDirectory, filename)
		if isExecutableFile(path) {
			return path
		}
	}
	return ""
}

func bundledAgentBrowserFilenames() []string {
	names := []string{"agent-browser"}
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			names = append(names, "agent-browser-aarch64-apple-darwin", "agent-browser-darwin-arm64")
		} else {
			names = append(names, "agent-browser-x86_64-apple-darwin", "agent-browser-darwin-x64")
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			names = append(names, "agent-browser-aarch64-unknown-linux-gnu", "agent-browser-linux-arm64")
		} else {
			names = append(names, "agent-browser-x86_64-unknown-linux-gnu", "agent-browser-linux-x64")
		}
	case "windows":
		names = append(names, "agent-browser.exe", "agent-browser-x86_64-pc-windows-msvc.exe", "agent-browser-win32-x64.exe")
	}
	return names
}

func isExecutableFile(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && !information.IsDir() && information.Mode()&0o111 != 0
}

func isDirectory(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && information.IsDir()
}

// companionBrowserAutomationReport pairs the readiness of the runtime that
// actually drives browser automation (ExtensionInputRuntime) with the
// presence of the legacy agent-browser CLI, which is no longer used by
// automation but is still bundled and worth surfacing in `status`.
type companionBrowserAutomationReport struct {
	Extension         browserruntime.RuntimeReadiness
	AgentBrowserPath  string
	AgentBrowserFound bool
}

func companionBrowserAutomationReadiness(chromeExecutablePath string, extensionPathFlagValue string, agentBrowserFlagValue string) companionBrowserAutomationReport {
	resolvedExtensionPath := resolveBrowserExtensionPath(extensionPathFlagValue)
	resolvedAgentBrowserPath := resolveAgentBrowserPath(agentBrowserFlagValue)
	return companionBrowserAutomationReport{
		Extension:         extensionBrowserRuntimeReadiness(chromeExecutablePath, resolvedExtensionPath),
		AgentBrowserPath:  resolvedAgentBrowserPath,
		AgentBrowserFound: isExecutableFile(resolvedAgentBrowserPath),
	}
}
