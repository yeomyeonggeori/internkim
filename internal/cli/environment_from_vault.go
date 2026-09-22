package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// The names this repository keeps in the operating system's vault are listed in
// its monkeys manifest, and `monkeys` hands a value to the command it starts
// rather than to whoever asks — a value read back through a pipe comes out
// redacted, on purpose. So the only way for the CLI to hold one is to be that
// command: it re-executes itself under `monkeys run` and marks the environment
// so the second run knows the values already arrived.
//
// Everything here declines rather than fails. No manifest, no monkeys, no
// profile, or a profile the vault cannot fill yet, and the CLI runs exactly as
// it would have.
//
// The profile is never named here. `monkeys` reads the first profile the
// manifest declares when a command names none, so `run` and `remember` are
// given none and the manifest stays the one place the name is written.
const (
	vaultManifestName          = ".monkeys"
	vaultManifestNamespaceMark = "+"
	vaultManifestProfileMark   = "@"
	// Set on the re-executed run, and read as a literal at the one place that
	// reads it so the declaration scan can see the read.
	vaultInjectedMarker = "INTERNKIM_ENVIRONMENT_FROM_VAULT"
)

func reExecuteWithVaultEnvironment() {
	monkeysPath, shouldRun := plannedVaultRun()
	if !shouldRun {
		return
	}
	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return
	}
	arguments := append([]string{
		filepath.Base(monkeysPath), "run", executablePath,
	}, os.Args[1:]...)
	errorValue = syscall.Exec(monkeysPath, arguments, append(os.Environ(), vaultInjectedMarker+"=1"))
	fmt.Fprintf(os.Stderr, "internkim: could not run through the vault: %v\n", errorValue)
}

func plannedVaultRun() (string, bool) {
	if os.Getenv("INTERNKIM_ENVIRONMENT_FROM_VAULT") != "" {
		return "", false
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return "", false
	}
	manifest, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, vaultManifestName))
	if errorValue != nil {
		return "", false
	}
	profile := vaultManifestProfile(string(manifest))
	if len(vaultManifestNames(string(manifest), profile)) == 0 {
		return "", false
	}
	monkeysPath := vaultCommandPath()
	if monkeysPath == "" {
		fmt.Fprintf(os.Stderr, "internkim: %s lists a @%s profile but monkeys is not installed, "+
			"so the vault cannot supply it\n", vaultManifestName, profile)
		return "", false
	}
	missing := vaultProfileGap(monkeysPath, repositoryRootPath, profile)
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "internkim: the vault has no %s yet, so the environment is whatever was "+
			"already set; a human types each into `monkeys remember @%s <name>`\n",
			strings.Join(missing, ", "), profile)
		return "", false
	}
	return monkeysPath, true
}

func vaultCommandPath() string {
	if path, errorValue := exec.LookPath("monkeys"); errorValue == nil {
		return path
	}
	homeDirectoryPath, errorValue := os.UserHomeDir()
	if errorValue != nil {
		return ""
	}
	installedPath := filepath.Join(homeDirectoryPath, ".local", "bin", "monkeys")
	if _, errorValue := os.Stat(installedPath); errorValue != nil {
		return ""
	}
	return installedPath
}

func vaultProfileGap(monkeysPath, repositoryRootPath, profile string) []string {
	command := exec.Command(monkeysPath, "doctor", "--short")
	command.Dir = repositoryRootPath
	output, _ := command.Output()
	return vaultMissingNames(string(output), profile)
}

// `monkeys doctor --short` prints one `missing @profile: A, B` line per profile
// that lacks a secret, and nothing at all when none does.
func vaultMissingNames(doctorOutput, profile string) []string {
	prefix := "missing " + vaultManifestProfileMark + profile + ":"
	for _, line := range strings.Split(doctorOutput, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		return splitAndTrim(strings.TrimPrefix(line, prefix), ",")
	}
	return nil
}

// The first profile the manifest declares is the one a command that names none
// runs under, so it is the one this repository means everywhere.
func vaultManifestProfile(manifest string) string {
	for _, line := range strings.Split(manifest, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, vaultManifestProfileMark) {
			continue
		}
		opened := splitAndTrim(strings.TrimPrefix(line, vaultManifestProfileMark), ",")
		if len(opened) > 0 {
			return opened[0]
		}
	}
	return ""
}

// A manifest names the project on its first line, opens a profile with an `@`
// line that may list several, and carries a bare key per secret. A `KEY=value`
// line is a value that is not secret, which `monkeys run` reads from the file
// rather than the vault, so the vault is never asked for one.
func vaultManifestNames(manifest, profile string) []string {
	names := []string{}
	profileIsOpen := false
	for _, line := range strings.Split(manifest, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, vaultManifestNamespaceMark) {
			continue
		}
		if strings.HasPrefix(line, vaultManifestProfileMark) {
			openedProfiles := splitAndTrim(strings.TrimPrefix(line, vaultManifestProfileMark), ",")
			profileIsOpen = slices.Contains(openedProfiles, profile)
			continue
		}
		if profileIsOpen && !strings.Contains(line, "=") {
			names = append(names, line)
		}
	}
	return names
}

func splitAndTrim(value, separator string) []string {
	parts := []string{}
	for _, part := range strings.Split(value, separator) {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// A credential the CLI generates has the same home as one it reads, so it goes
// back into the vault rather than into a file beside the checkout.
func rememberInVault(name, value string) error {
	monkeysPath := vaultCommandPath()
	if monkeysPath == "" {
		return fmt.Errorf("monkeys is not installed, and the vault is where %s lives", name)
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	command := exec.Command(monkeysPath, "remember", name)
	command.Dir = repositoryRootPath
	command.Stdin = strings.NewReader(value)
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("monkeys remember %s: %w: %s", name, errorValue, strings.TrimSpace(string(output)))
	}
	return nil
}
