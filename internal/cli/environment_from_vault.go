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
// The profile is chosen by a leading `@profile` argument, the way `monkeys`
// itself takes one, and is otherwise the first the manifest declares. monkeys
// puts `test` first so that the default is the harmless one, and production is
// something a person says: `internkim @production deploy`. The re-executed run
// is always told the profile, because monkeys hands the command it starts no
// word of which one it read.
const (
	vaultManifestName          = ".monkeys"
	vaultManifestNamespaceMark = "+"
	vaultManifestProfileMark   = "@"
	// Set on the re-executed run to the profile it was handed, and read as a
	// literal at the one place that reads it so the declaration scan can see
	// the read.
	vaultInjectedMarker = "INTERNKIM_ENVIRONMENT_FROM_VAULT"
)

func splitVaultProfileArgument(arguments []string) (string, []string) {
	if len(arguments) == 0 || !strings.HasPrefix(arguments[0], vaultManifestProfileMark) {
		return "", arguments
	}
	return strings.TrimPrefix(arguments[0], vaultManifestProfileMark), arguments[1:]
}

func reExecuteWithVaultEnvironment(requestedProfile string) {
	plan, shouldRun := plannedVaultRun(requestedProfile)
	if !shouldRun {
		return
	}
	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return
	}
	arguments := append([]string{
		filepath.Base(plan.monkeysPath), "run", vaultManifestProfileMark + plan.profile, executablePath,
	}, os.Args[1:]...)
	errorValue = syscall.Exec(plan.monkeysPath, arguments, append(os.Environ(), vaultInjectedMarker+"="+plan.profile))
	fmt.Fprintf(os.Stderr, "internkim: could not run through the vault: %v\n", errorValue)
}

type vaultRun struct {
	monkeysPath string
	profile     string
}

func plannedVaultRun(requestedProfile string) (vaultRun, bool) {
	if os.Getenv("INTERNKIM_ENVIRONMENT_FROM_VAULT") != "" {
		return vaultRun{}, false
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return vaultRun{}, false
	}
	manifest, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, vaultManifestName))
	if errorValue != nil {
		return vaultRun{}, false
	}
	profile, errorValue := chosenVaultProfile(string(manifest), requestedProfile)
	if errorValue != nil {
		fmt.Fprintf(os.Stderr, "internkim: %v\n", errorValue)
		os.Exit(2)
	}
	if len(vaultManifestNames(string(manifest), profile)) == 0 {
		return vaultRun{}, false
	}
	monkeysPath := vaultCommandPath()
	if monkeysPath == "" {
		fmt.Fprintf(os.Stderr, "internkim: %s lists a @%s profile but monkeys is not installed, "+
			"so the vault cannot supply it\n", vaultManifestName, profile)
		return vaultRun{}, false
	}
	missing := vaultProfileGap(monkeysPath, repositoryRootPath, profile)
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "internkim: the vault has no %s yet, so the environment is whatever was "+
			"already set; a human types each into `monkeys remember @%s <name>`\n",
			strings.Join(missing, ", "), profile)
		return vaultRun{}, false
	}
	return vaultRun{monkeysPath: monkeysPath, profile: profile}, true
}

func chosenVaultProfile(manifest, requestedProfile string) (string, error) {
	declared := vaultManifestProfiles(manifest)
	if requestedProfile == "" {
		if len(declared) == 0 {
			return "", nil
		}
		return declared[0], nil
	}
	if !slices.Contains(declared, requestedProfile) {
		return "", fmt.Errorf("%s declares no @%s profile; it declares @%s",
			vaultManifestName, requestedProfile, strings.Join(declared, ", @"))
	}
	return requestedProfile, nil
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

func vaultManifestProfiles(manifest string) []string {
	declared := []string{}
	for _, line := range strings.Split(manifest, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, vaultManifestProfileMark) {
			continue
		}
		for _, profile := range splitAndTrim(strings.TrimPrefix(line, vaultManifestProfileMark), ",") {
			if !slices.Contains(declared, profile) {
				declared = append(declared, profile)
			}
		}
	}
	return declared
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
// back into the vault, into the profile this run was handed.
func rememberInVault(name, value string) error {
	monkeysPath := vaultCommandPath()
	if monkeysPath == "" {
		return fmt.Errorf("monkeys is not installed, and the vault is where %s lives", name)
	}
	if os.Getenv("INTERNKIM_ENVIRONMENT_FROM_VAULT") == "" {
		return fmt.Errorf("this run was not handed a vault profile, so there is no profile to keep %s in; "+
			"run it as `internkim @<profile> …`", name)
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	command := exec.Command(monkeysPath, "remember", vaultManifestProfileMark+os.Getenv("INTERNKIM_ENVIRONMENT_FROM_VAULT"), name)
	command.Dir = repositoryRootPath
	command.Stdin = strings.NewReader(value)
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("monkeys remember %s: %w: %s", name, errorValue, strings.TrimSpace(string(output)))
	}
	return nil
}
