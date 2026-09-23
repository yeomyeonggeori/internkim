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
	if _, errorValue := NewGPGSigner(""); errorValue == nil || !strings.Contains(errorValue.Error(), SigningKeyVariable) {
		t.Fatalf("an absent key path gave %v, wanted the name that holds one", errorValue)
	}
	if _, errorValue := NewGPGSigner(filepath.Join(t.TempDir(), "absent.asc")); errorValue == nil {
		t.Fatal("a key path pointing at nothing was accepted")
	}
}

// The private half is what lets anyone install software as root on a customer's
// machine. It has one home, the operating system's vault, and what reaches gpg is a
// file this process wrote and removes rather than a value any child can read out of
// its own environment.
func TestTheSigningKeyReachesGPGAsAPrivateFileAndLeavesNoEnvironmentValue(t *testing.T) {
	t.Setenv(SigningKeyVariable, "  -----BEGIN PGP PRIVATE KEY BLOCK-----  ")

	keyPath, remove, errorValue := MaterialiseSigningKey()
	if errorValue != nil {
		t.Fatalf("materialise the signing key: %v", errorValue)
	}
	defer remove()

	if held := os.Getenv(SigningKeyVariable); held != "" {
		t.Fatalf("%s still holds a value after the key was written, so every child inherits it", SigningKeyVariable)
	}
	information, errorValue := os.Stat(keyPath)
	if errorValue != nil {
		t.Fatalf("stat the materialised key: %v", errorValue)
	}
	if information.Mode().Perm() != 0o600 {
		t.Fatalf("the materialised key is mode %o, wanted 600", information.Mode().Perm())
	}
	written, errorValue := os.ReadFile(keyPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.TrimSpace(string(written)) != "-----BEGIN PGP PRIVATE KEY BLOCK-----" {
		t.Fatalf("the materialised key holds %q", string(written))
	}

	remove()
	if _, errorValue := os.Stat(keyPath); !os.IsNotExist(errorValue) {
		t.Fatalf("the materialised key survived the run: %v", errorValue)
	}
}

func TestASigningKeyTheVaultDoesNotHoldNamesWhatToRun(t *testing.T) {
	t.Setenv(SigningKeyVariable, "")

	_, _, errorValue := MaterialiseSigningKey()
	if errorValue == nil {
		t.Fatal("a release with no signing key was allowed to start")
	}
	for _, named := range []string{"internkim @production release apt", SigningKeyVariable} {
		if !strings.Contains(errorValue.Error(), named) {
			t.Fatalf("the refusal is %q and does not name %q", errorValue.Error(), named)
		}
	}
}

func TestTheConfiguredVariableIsTheOneThatIsRead(t *testing.T) {
	t.Setenv(SigningKeyVariable, "  a-key  ")
	keyPath, remove, errorValue := MaterialiseSigningKey()
	if errorValue != nil {
		t.Fatalf("MaterialiseSigningKey does not read %s: %v", SigningKeyVariable, errorValue)
	}
	remove()
	if keyPath == "" {
		t.Fatalf("MaterialiseSigningKey read %s and wrote nowhere", SigningKeyVariable)
	}
	if SigningKeyVariable != "INTERNKIM_APT_SIGNING_KEY" {
		t.Fatalf("the signing key is held under %s", SigningKeyVariable)
	}
}

// The vault hands the key over as an environment value; this is where it stops
// being one. A child process this release starts — gpg, or anything gpg starts —
// inherits the environment, and the process table is readable by anything
// running as this user.
func TestMaterialisingTheKeyTakesItOutOfTheEnvironment(t *testing.T) {
	t.Setenv(SigningKeyVariable, "the-key-the-vault-handed-over")

	keyPath, remove, errorValue := MaterialiseSigningKey()
	if errorValue != nil {
		t.Fatalf("materialise the signing key: %v", errorValue)
	}

	if leftBehind := os.Getenv(SigningKeyVariable); leftBehind != "" {
		t.Fatalf("%s still holds %q after the key was written to a file", SigningKeyVariable, leftBehind)
	}
	information, errorValue := os.Stat(keyPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if mode := information.Mode().Perm(); mode != 0o600 {
		t.Fatalf("the signing key file is mode %o, not 600", mode)
	}
	remove()
	if _, errorValue := os.Stat(keyPath); !os.IsNotExist(errorValue) {
		t.Fatalf("the signing key file outlived the run: %v", errorValue)
	}
}
