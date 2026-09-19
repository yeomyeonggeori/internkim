package cli

import (
	"os"
	"path/filepath"
	"testing"

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
		t.Fatal("console password was read from .local/secrets; .env is its only home")
	}
	if len(resolved) != 24 {
		t.Fatalf("generated console password is %d characters, wanted 24", len(resolved))
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
