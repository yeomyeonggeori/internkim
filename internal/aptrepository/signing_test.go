package aptrepository

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// shortLivedHomeDirectory gives gpg a homedir whose gpg-agent socket path fits
// in the 104 bytes a unix socket gets, which t.TempDir() on macOS does not.
func shortLivedHomeDirectory(t *testing.T) string {
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

// throwawaySigningKey generates a key that says in its own user ID that it is
// not the archive key. It exists so the signing path can be exercised; the
// real key's home is a decision this test must not pre-empt.
func throwawaySigningKey(t *testing.T) string {
	t.Helper()
	if _, errorValue := exec.LookPath("gpg"); errorValue != nil {
		t.Skip("gpg is not on PATH")
	}
	homeDirectory := shortLivedHomeDirectory(t)
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

func TestTheSignatureVerifiesAgainstThePublishedKeyring(t *testing.T) {
	signer, errorValue := NewGPGSigner(throwawaySigningKey(t))
	if errorValue != nil {
		t.Fatalf("open the signer: %v", errorValue)
	}
	defer signer.Close()

	release := []byte("Origin: InternKim\nSuite: stable\n")
	inRelease, errorValue := signer.ClearSign(release)
	if errorValue != nil {
		t.Fatalf("clear-sign: %v", errorValue)
	}
	if !bytes.Contains(inRelease, []byte("BEGIN PGP SIGNED MESSAGE")) {
		t.Fatalf("InRelease is not clear-signed:\n%s", inRelease)
	}
	keyring, errorValue := signer.PublicKeyring()
	if errorValue != nil {
		t.Fatalf("export the keyring: %v", errorValue)
	}

	verificationHome := shortLivedHomeDirectory(t)
	keyringPath := filepath.Join(verificationHome, "keyring.pgp")
	if errorValue := os.WriteFile(keyringPath, keyring, 0o644); errorValue != nil {
		t.Fatalf("write the keyring: %v", errorValue)
	}
	signaturePath := filepath.Join(verificationHome, "InRelease")
	if errorValue := os.WriteFile(signaturePath, inRelease, 0o644); errorValue != nil {
		t.Fatalf("write InRelease: %v", errorValue)
	}
	verify := exec.Command("gpg", "--homedir", verificationHome, "--batch", "--no-tty",
		"--no-default-keyring", "--keyring", keyringPath, "--verify", signaturePath)
	output, errorValue := verify.CombinedOutput()
	exec.Command("gpgconf", "--homedir", verificationHome, "--kill", "gpg-agent").Run()
	if errorValue != nil {
		t.Fatalf("the published keyring does not verify the signature this signer made:\n%s", output)
	}
}

func TestASigningKeyWithNoFileIsNamedRatherThanGuessedAt(t *testing.T) {
	if _, errorValue := NewGPGSigner(""); errorValue == nil || !strings.Contains(errorValue.Error(), SigningKeyPathVariable) {
		t.Fatalf("an absent key path gave %v, wanted the variable that names one", errorValue)
	}
	if _, errorValue := NewGPGSigner(filepath.Join(t.TempDir(), "absent.asc")); errorValue == nil {
		t.Fatal("a key path pointing at nothing was accepted")
	}
}

// The private half is what lets anyone install software as root on a
// customer's machine, so it has one home and the tree is not it.
func TestTheDefaultSigningKeyPathIsOutsideTheTree(t *testing.T) {
	path := DefaultSigningKeyPath("/repository")
	if path != "/repository/.local/secrets/apt-archive-signing-key.asc" {
		t.Fatalf("the default key path is %s", path)
	}
	if SigningKeyPathVariable != "INTERNKIM_APT_SIGNING_KEY_PATH" {
		t.Fatalf("the signing key is configured by %s, which is not a path", SigningKeyPathVariable)
	}
}

func TestTheConfiguredVariableIsTheOneThatIsRead(t *testing.T) {
	t.Setenv(SigningKeyPathVariable, "  /somewhere/archive-key.asc  ")
	if read := SigningKeyPath(); read != "/somewhere/archive-key.asc" {
		t.Fatalf("SigningKeyPath read %q; it does not read %s", read, SigningKeyPathVariable)
	}
}
