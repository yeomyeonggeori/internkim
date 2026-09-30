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

// Verify fails the test unless signature is a valid detached signature of
// document by the armoured or binary public key, checked by gpg in a keyring
// that holds nothing else.
func Verify(t *testing.T, publicKey []byte, signature []byte, document []byte) {
	t.Helper()
	directory := ShortLivedHomeDirectory(t)
	paths := map[string][]byte{"key": publicKey, "signature": signature, "document": document}
	for name, contents := range paths {
		if errorValue := os.WriteFile(filepath.Join(directory, name), contents, 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	command := exec.Command("gpg", "--homedir", directory, "--batch", "--no-tty", "--no-default-keyring",
		"--keyring", filepath.Join(directory, "keyring.kbx"), "--import", filepath.Join(directory, "key"))
	if output, errorValue := command.CombinedOutput(); errorValue != nil {
		t.Fatalf("import the public key: %s", output)
	}
	command = exec.Command("gpg", "--homedir", directory, "--batch", "--no-tty", "--no-default-keyring",
		"--keyring", filepath.Join(directory, "keyring.kbx"), "--verify",
		filepath.Join(directory, "signature"), filepath.Join(directory, "document"))
	if output, errorValue := command.CombinedOutput(); errorValue != nil {
		t.Fatalf("the signature does not verify:\n%s", output)
	}
}
