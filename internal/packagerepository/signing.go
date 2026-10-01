package packagerepository

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SigningKeyVariable is the name the operating system's vault holds the private
// half of the archive key under, and the name `monkeys run` sets when it hands
// the key to a release. It is the key's one home, and it signs the apt, rpm and
// pacman repositories alike.
//
// What gpg is given is still a path, because a secret this repository hands a
// program is a file that program opens rather than a value in its environment.
// MaterialiseSigningKey is the seam between the two, and it is the one function
// that changes when the key moves to a smartcard.
const SigningKeyVariable = "INTERNKIM_PACKAGE_SIGNING_KEY"

// GPGSigner signs repository metadata and packages with an exported OpenPGP
// secret key, in a homedir it creates and destroys around the run.
//
// Shelling out to gpg rather than signing in-process is what keeps the key's
// eventual home an open question: a key held by a running gpg-agent, on a
// smartcard, or exported to a file are all the same call here, and only
// MaterialiseSigningKey changes.
type GPGSigner struct {
	homeDirectory string
	fingerprint   string
}

// NewGPGSigner imports the secret key at keyPath into a private homedir.
// The caller closes it.
func NewGPGSigner(keyPath string) (*GPGSigner, error) {
	if strings.TrimSpace(keyPath) == "" {
		return nil, fmt.Errorf("no signing key: %s is what holds it", SigningKeyVariable)
	}
	key, errorValue := os.ReadFile(keyPath)
	if errorValue != nil {
		return nil, fmt.Errorf("read the archive signing key: %w", errorValue)
	}
	if _, errorValue := exec.LookPath("gpg"); errorValue != nil {
		return nil, errors.New("gpg is not on PATH; the archive signature is what apt, dnf and pacman check before they install anything")
	}
	homeDirectory, errorValue := makeShortLivedHomeDirectory()
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := os.Chmod(homeDirectory, 0o700); errorValue != nil {
		os.RemoveAll(homeDirectory)
		return nil, errorValue
	}
	signer := &GPGSigner{homeDirectory: homeDirectory}
	if errorValue := signer.importKey(key); errorValue != nil {
		signer.Close()
		return nil, errorValue
	}
	fingerprint, errorValue := signer.readFingerprint()
	if errorValue != nil {
		signer.Close()
		return nil, errorValue
	}
	signer.fingerprint = fingerprint
	return signer, nil
}

// makeShortLivedHomeDirectory keeps the gpg homedir's path short.
//
// gpg-agent's socket is a unix socket inside the homedir, and a unix socket
// path over about 104 bytes cannot be bound. macOS puts the default temporary
// directory under /var/folders/<two>/<28 characters>/T/, which spends most of
// that budget before the homedir is named, and gpg then reports "can't
// connect to the gpg-agent: File name too long" rather than a path problem.
func makeShortLivedHomeDirectory() (string, error) {
	for _, base := range []string{"/tmp", ""} {
		homeDirectory, errorValue := os.MkdirTemp(base, "ikapt-")
		if errorValue == nil {
			return homeDirectory, nil
		}
	}
	return "", errors.New("no writable temporary directory for a gpg homedir")
}

func (signer *GPGSigner) Fingerprint() string {
	return signer.fingerprint
}

func (signer *GPGSigner) Close() {
	signer.run(nil, "--quit-agent")
	exec.Command("gpgconf", "--homedir", signer.homeDirectory, "--kill", "gpg-agent").Run()
	os.RemoveAll(signer.homeDirectory)
}

func (signer *GPGSigner) ClearSign(document []byte) ([]byte, error) {
	return signer.run(document, "--clearsign", "--digest-algo", "SHA256", "--local-user", signer.fingerprint)
}

func (signer *GPGSigner) DetachSign(document []byte) ([]byte, error) {
	return signer.run(document, "--detach-sign", "--armor", "--digest-algo", "SHA256", "--local-user", signer.fingerprint)
}

// DetachSignBinary is the signature rpm keeps in a package header and pacman
// keeps in a .sig file; neither reads the armoured form.
func (signer *GPGSigner) DetachSignBinary(document []byte) ([]byte, error) {
	return signer.run(document, "--detach-sign", "--digest-algo", "SHA256", "--local-user", signer.fingerprint)
}

// PublicKeyArmoured is the form `rpm --import` and `pacman-key --add` read.
func (signer *GPGSigner) PublicKeyArmoured() ([]byte, error) {
	return signer.run(nil, "--armor", "--export", signer.fingerprint)
}

// PublicKeyring is the binary keyring the install script writes to
// /usr/share/keyrings and a source's Signed-By names. Binary rather than
// armoured because a .pgp suffix on an armoured file is the shape apt warns
// about.
func (signer *GPGSigner) PublicKeyring() ([]byte, error) {
	return signer.run(nil, "--export", signer.fingerprint)
}

func (signer *GPGSigner) importKey(key []byte) error {
	if _, errorValue := signer.run(key, "--import"); errorValue != nil {
		return fmt.Errorf("import the archive signing key: %w", errorValue)
	}
	return nil
}

// readFingerprint names the one secret key in the homedir, so a keyring that
// happens to hold two does not sign with whichever gpg picks.
func (signer *GPGSigner) readFingerprint() (string, error) {
	listed, errorValue := signer.run(nil, "--list-secret-keys", "--with-colons")
	if errorValue != nil {
		return "", errorValue
	}
	var fingerprints []string
	previousWasSecretKey := false
	for _, line := range strings.Split(string(listed), "\n") {
		columns := strings.Split(line, ":")
		if len(columns) < 10 {
			continue
		}
		if columns[0] == "sec" {
			previousWasSecretKey = true
			continue
		}
		if columns[0] == "fpr" && previousWasSecretKey {
			fingerprints = append(fingerprints, columns[9])
			previousWasSecretKey = false
		}
	}
	if len(fingerprints) == 0 {
		return "", errors.New("the signing key file holds no secret key")
	}
	if len(fingerprints) > 1 {
		return "", fmt.Errorf("the signing key file holds %d secret keys; an archive is signed by one", len(fingerprints))
	}
	return fingerprints[0], nil
}

func (signer *GPGSigner) run(input []byte, arguments ...string) ([]byte, error) {
	command := exec.Command("gpg", append([]string{
		"--homedir", signer.homeDirectory,
		"--batch", "--yes", "--no-tty",
		"--pinentry-mode", "loopback", "--passphrase", "",
	}, arguments...)...)
	command.Env = append(os.Environ(), "GNUPGHOME="+signer.homeDirectory)
	if input != nil {
		command.Stdin = bytes.NewReader(input)
	}
	var signed, spoken bytes.Buffer
	command.Stdout = &signed
	command.Stderr = &spoken
	if errorValue := command.Run(); errorValue != nil {
		return nil, fmt.Errorf("gpg %s: %s", arguments[0], strings.TrimSpace(spoken.String()))
	}
	return signed.Bytes(), nil
}

// MaterialiseSigningKey writes what the vault handed this process to a private
// file and answers its path, then takes the value out of this process's
// environment so nothing it starts inherits it. The caller removes the file.
//
// The name is spelled here rather than passed as the constant because
// `tools/verify-environment-declarations` reads the literal at the call site;
// TestTheConfiguredVariableIsTheOneThatIsRead holds the two together.
func MaterialiseSigningKey() (string, func(), error) {
	key := strings.TrimSpace(os.Getenv("INTERNKIM_PACKAGE_SIGNING_KEY"))
	if key == "" {
		return "", func() {}, fmt.Errorf(
			"no archive signing key: run this as `internkim @production release repositories`, which hands it %s "+
				"out of the vault for the length of the command", SigningKeyVariable)
	}
	if errorValue := os.Unsetenv("INTERNKIM_PACKAGE_SIGNING_KEY"); errorValue != nil {
		return "", func() {}, errorValue
	}
	directory, errorValue := makeShortLivedHomeDirectory()
	if errorValue != nil {
		return "", func() {}, errorValue
	}
	remove := func() { os.RemoveAll(directory) }
	keyPath := filepath.Join(directory, "archive-signing-key.asc")
	if errorValue := os.WriteFile(keyPath, []byte(key+"\n"), 0o600); errorValue != nil {
		remove()
		return "", func() {}, errorValue
	}
	return keyPath, remove, nil
}
