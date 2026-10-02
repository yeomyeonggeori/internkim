package cli

import (
	"os"
	"path/filepath"
	"testing"

	setup "github.com/yeomyeonggeori/internkim/internal/provisioning/steps"
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
