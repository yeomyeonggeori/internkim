package browser

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserStartupFailureCarriesProcessDiagnostics(t *testing.T) {
	directory := t.TempDir()
	executablePath := filepath.Join(directory, "browser")
	if errorValue := os.WriteFile(executablePath, []byte("#!/bin/sh\nprintf 'resource bundle could not be loaded\\n' >&2\nexit 23\n"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	memberDirectory := filepath.Join(directory, "members", "sample")
	_, errorValue := LaunchDeviceBrowserProcess(context.Background(), DeviceBrowserLaunch{
		ExecutablePath: executablePath, Port: availableBrowserPort(t), MemberDirectory: memberDirectory,
		ProfileDirectory: filepath.Join(memberDirectory, "profile"), CacheDirectory: filepath.Join(memberDirectory, "cache"),
		LogPath: filepath.Join(memberDirectory, "moli.log"),
	})
	if errorValue == nil {
		t.Fatal("expected a browser startup failure")
	}
	for _, expected := range []string{"exit status 23", "resource bundle could not be loaded"} {
		if !strings.Contains(errorValue.Error(), expected) {
			t.Fatalf("startup failure omitted %q: %v", expected, errorValue)
		}
	}
}

func TestBrowserDiagnosticsKeepOnlyTheBoundedTail(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "browser.log")
	content := strings.Repeat("x", deviceBrowserDiagnosticBytes) + "last diagnostic"
	if errorValue := os.WriteFile(logPath, []byte(content), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	diagnostics, errorValue := deviceBrowserDiagnostics(logPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(diagnostics) != deviceBrowserDiagnosticBytes || !strings.HasSuffix(diagnostics, "last diagnostic") {
		t.Fatalf("expected a bounded diagnostic tail, got %d bytes", len(diagnostics))
	}
}

func availableBrowserPort(t *testing.T) int {
	t.Helper()
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
