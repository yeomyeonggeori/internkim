// Package repositorytest holds what the tests of every repository format share:
// a throwaway signing key that says in its own user ID that it is not the
// archive key.
package repositorytest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// ShortLivedHomeDirectory gives gpg a homedir whose gpg-agent socket path fits
// in the 104 bytes a unix socket gets, which t.TempDir() on macOS does not.
func ShortLivedHomeDirectory(t *testing.T) string {
	t.Helper()
	homeDirectory, errorValue := os.MkdirTemp("/tmp", "ikapt-test-")
	if errorValue != nil {
		t.Fatalf("make a gpg homedir: %v", errorValue)
	}
	t.Cleanup(func() {
		exec.Command("gpgconf", "--homedir", homeDirectory, "--kill", "gpg-agent").Run()
		os.RemoveAll(homeDirectory)
	})
	if errorValue := os.Chmod(homeDirectory, 0o700); errorValue != nil {
		t.Fatalf("tighten the homedir: %v", errorValue)
	}
	return homeDirectory
}

// KeyPath generates a key that says in its own user ID that it is
// not the archive key. It exists so the signing path can be exercised; the
// real key's home is a decision this test must not pre-empt.
func KeyPath(t *testing.T) string {
	t.Helper()
	if _, errorValue := exec.LookPath("gpg"); errorValue != nil {
		t.Skip("gpg is not on PATH")
	}
	homeDirectory := ShortLivedHomeDirectory(t)
	generate := exec.Command("gpg",
		"--homedir", homeDirectory, "--batch", "--yes", "--no-tty",
		"--pinentry-mode", "loopback", "--passphrase", "",
		"--quick-generate-key", "InternKim TEST KEY DO NOT TRUST <test-key@invalid.internkim.test>",
		"default", "default", "never")
	if output, errorValue := generate.CombinedOutput(); errorValue != nil {
		t.Skipf("gpg could not generate a key here: %s", output)
	}
	exported := exec.Command("gpg", "--homedir", homeDirectory, "--batch", "--no-tty",
		"--pinentry-mode", "loopback", "--passphrase", "", "--armor", "--export-secret-keys")
	key, errorValue := exported.Output()
	if errorValue != nil || len(key) == 0 {
		t.Skipf("gpg exported no secret key: %v", errorValue)
	}
	keyPath := filepath.Join(t.TempDir(), "apt-archive-signing-key.asc")
	if errorValue := os.WriteFile(keyPath, key, 0o600); errorValue != nil {
		t.Fatalf("write the throwaway key: %v", errorValue)
	}
	exec.Command("gpgconf", "--homedir", homeDirectory, "--kill", "gpg-agent").Run()
	return keyPath
}
