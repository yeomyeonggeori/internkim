package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/packagerepository"
	"gitlab.com/eastriver/internkim/internal/deployops"
	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
)

func writeDecoyLocalSecret(t *testing.T, name string, contents string) {
	t.Helper()
	workingDirectory := t.TempDir()
	secretsDirectory := filepath.Join(workingDirectory, ".local", "secrets")
	if errorValue := os.MkdirAll(secretsDirectory, 0o700); errorValue != nil {
		t.Fatalf("create decoy secrets directory: %v", errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(secretsDirectory, name), []byte(contents), 0o600); errorValue != nil {
		t.Fatalf("write decoy secret: %v", errorValue)
	}
	t.Chdir(workingDirectory)
}

func TestConsolePasswordComesFromTheEnvironmentAlone(t *testing.T) {
	writeDecoyLocalSecret(t, "console-password", "password-from-the-file")
	t.Setenv("INTERNKIM_CONSOLE_PASSWORD", "password-from-the-environment")

	resolvedForFlash, errorValue := resolveConsolePassword()
	if errorValue != nil {
		t.Fatalf("resolve console password: %v", errorValue)
	}
	_, resolvedForSetup := resolveSetupSSHCredentials(setup.BoardJetsonOrinNano, "", "")

	if resolvedForFlash != "password-from-the-environment" {
		t.Fatalf("flash read %q, wanted the environment value", resolvedForFlash)
	}
	if resolvedForSetup != resolvedForFlash {
		t.Fatalf("setup read %q and flash read %q; both must resolve the same password", resolvedForSetup, resolvedForFlash)
	}
}

func TestConsolePasswordWithoutAnEnvironmentValueIsGeneratedRatherThanReadFromAFile(t *testing.T) {
	writeDecoyLocalSecret(t, "console-password", "password-from-the-file")
	t.Setenv("INTERNKIM_CONSOLE_PASSWORD", "")

	resolved, errorValue := resolveConsolePassword()
	if errorValue != nil {
		t.Fatalf("resolve console password: %v", errorValue)
	}
	if resolved == "password-from-the-file" {
		t.Fatal("console password was read from .local/secrets; the vault is its only home")
	}
	if len(resolved) != 24 {
		t.Fatalf("generated console password is %d characters, wanted 24", len(resolved))
	}
}

// The archive signing key publishes software every company host installs as root, and
// its home is the operating system's vault. A file left beside the other local secrets
// is the shape it used to fall back to, and a fallback means two homes.
func TestTheArchiveSigningKeyComesFromTheVaultAlone(t *testing.T) {
	writeDecoyLocalSecret(t, "apt-archive-signing-key.asc", "key-from-the-file")
	t.Setenv(packagerepository.SigningKeyVariable, "")

	if _, _, errorValue := packagerepository.MaterialiseSigningKey(); errorValue == nil {
		t.Fatal("a release with nothing in the vault found a signing key anyway")
	}

	t.Setenv(packagerepository.SigningKeyVariable, "key-from-the-vault")
	keyPath, remove, errorValue := packagerepository.MaterialiseSigningKey()
	if errorValue != nil {
		t.Fatalf("materialise the signing key: %v", errorValue)
	}
	defer remove()
	written, errorValue := os.ReadFile(keyPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.TrimSpace(string(written)) != "key-from-the-vault" {
		t.Fatalf("the signing key resolved to %q", strings.TrimSpace(string(written)))
	}
}

// The Access service token authenticates every unattended deploy. It had a
// second home, a file under `.local/secrets`, and the two halves drifted
// without anything saying so: the stale one authenticated against nothing and
// the deploy quietly fell back to the browser login nobody was there to answer.
func TestTheAccessServiceTokenComesFromTheVaultAlone(t *testing.T) {
	writeDecoyLocalSecret(t, "cloudflare-access-service-token.json",
		`{"clientID":"id-from-the-file.access","clientSecret":"secret-from-the-file"}`)
	t.Setenv("INTERNKIM_CF_ACCESS_CLIENT_ID", "")
	t.Setenv("INTERNKIM_CF_ACCESS_CLIENT_SECRET", "")

	request, errorValue := http.NewRequest(http.MethodGet, "https://example.test/admin/", nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	deployops.AttachCloudflareAccess(request)
	if request.Header.Get("CF-Access-Client-Id") != "" {
		t.Fatalf("the service token resolved to %q from a file beside the checkout",
			request.Header.Get("CF-Access-Client-Id"))
	}
}

func TestReleaseDownloadTokenComesFromTheEnvironmentAlone(t *testing.T) {
	writeDecoyLocalSecret(t, "release-download-token", "token-from-the-file")
	t.Setenv("INTERNKIM_RELEASE_DOWNLOAD_TOKEN", "")

	if token := releaseDownloadToken(); token != "" {
		t.Fatalf("release download token resolved to %q with no environment value", token)
	}

	t.Setenv("INTERNKIM_RELEASE_DOWNLOAD_TOKEN", "token-from-the-environment")
	if token := releaseDownloadToken(); token != "token-from-the-environment" {
		t.Fatalf("release download token resolved to %q, wanted the environment value", token)
	}
}

func TestReleaseDownloadTokenIsStagedForTheDeviceOnlyFromTheEnvironment(t *testing.T) {
	writeDecoyLocalSecret(t, "release-download-token", "token-from-the-file")
	t.Setenv("INTERNKIM_RELEASE_DOWNLOAD_TOKEN", "")

	path, cleanup, errorValue := releaseDownloadTokenSourcePath()
	if cleanup != nil {
		defer cleanup()
	}
	if errorValue != nil {
		t.Fatalf("resolve release download token source: %v", errorValue)
	}
	if path != "" {
		t.Fatalf("staged %q for the device with no environment value", path)
	}
}

func TestBuzzKeySeedComesFromTheEnvironmentAlone(t *testing.T) {
	writeDecoyLocalSecret(t, "buzz-key-seed", "seed-from-the-file")
	t.Setenv("INTERNKIM_BUZZ_KEY_SEED", "")

	seed, errorValue := buildBuzzKeySeedCallback()(false)
	if errorValue != nil {
		t.Fatalf("resolve buzz key seed: %v", errorValue)
	}
	if seed != "" {
		t.Fatalf("buzz key seed resolved to %q with no environment value; a wrong seed derives wrong identities", seed)
	}
}

// The CLI's environment came from a dotenv file at the repository root until
// the values moved into the operating system's vault. Reading one again, from
// the working directory or the one above it, is how the fallback that carried
// the migration comes back and gives every one of these credentials a second
// home.
func TestTheCLIEnvironmentComesFromTheVaultAlone(t *testing.T) {
	workingDirectory := t.TempDir()
	decoy := "INTERNKIM_REGISTER_SECRET=secret-from-the-file\nCLOUDFLARE_DOMAIN=domain-from-the-file\n"
	if errorValue := os.WriteFile(filepath.Join(workingDirectory, ".env"), []byte(decoy), 0o600); errorValue != nil {
		t.Fatalf("write the decoy environment file: %v", errorValue)
	}
	t.Chdir(workingDirectory)
	t.Setenv("INTERNKIM_REGISTER_SECRET", "")
	t.Setenv("CLOUDFLARE_DOMAIN", "")

	configuration := loadConfig()

	if configuration.RegisterSecret != "" {
		t.Fatalf("the register secret resolved to %q from a file beside the checkout", configuration.RegisterSecret)
	}
	if configuration.CFDomain == "domain-from-the-file" {
		t.Fatal("the fleet domain was read from a file beside the checkout")
	}
}
