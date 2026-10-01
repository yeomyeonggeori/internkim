package companyhost

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The macOS half. `brew install internkim` delivers the files and this makes the
// box a host: it creates the two accounts through the directory service, sets
// the helper's ownership and its setuid bit — the one thing a bottle pour cannot
// carry, because extracting a tar as an ordinary user drops the bit and
// Homebrew's own linter forbids even recommending it — writes the nine
// LaunchDaemons from the same declaration the units come from, and then polls
// readiness where ordering used to stand.
//
// Two of Homebrew's rules decide the shape of this file, and both were read out
// of Homebrew's own source rather than assumed. `brew.sh`'s
// check-run-command-as-root refuses every command run as root except
// `as-console-user`, `setup-sandbox`, `--prefix` and `services`. So this may ask
// Homebrew where its prefix is and may start the database and the cache with
// `brew services`, and may do nothing else with it. And `Homebrew::Services::
// System.path` returns /Library/LaunchDaemons with the `system` domain target
// when euid is zero, so the database and the cache land in the same domain as
// the nine daemons here and survive a reboot with nobody logged in.

const (
	// Below 500 so the accounts do not appear at the login window, and far
	// below the 100000 the POSIX helper allocates projected identities from,
	// so a company's people can never collide with the services.
	firstServiceAccountID = 450
	lastServiceAccountID  = 499

	macServiceAccountShell = "/usr/bin/false"
)

// What `brew services` is asked to start, in the order the bundle needs them.
var macDistributionFormulas = map[string]string{
	databaseServiceName: "postgresql@17",
	cacheServiceName:    "redis",
}

type macPlatform struct {
	homebrewPrefix string
}

func (macPlatform) Describe() string {
	return "macOS"
}

func (macPlatform) NameOfItsSupervisor() string {
	return "launchd"
}

func (platform macPlatform) Layout() blueclaw.CompanyHostLayout {
	return blueclaw.MacCompanyHostLayout(platform.homebrewPrefix)
}

// homebrewPrefix asks Homebrew rather than guessing, because the prefix is
// /opt/homebrew on Apple silicon, /usr/local on Intel, and anything at all in a
// prefix somebody chose. `brew --prefix` is one of the two commands Homebrew
// permits as root.
func homebrewPrefix() string {
	if fromEnvironment := strings.TrimSpace(os.Getenv("HOMEBREW_PREFIX")); fromEnvironment != "" {
		return fromEnvironment
	}
	commandOutput, errorValue := exec.Command("brew", "--prefix").Output()
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(commandOutput))
}

// companyHostServiceAccount is one unprivileged account the bundle runs a
// service as. Two exist, and they are separate because the relay outlives the
// agent: it keeps answering when the agent is down, so it does not share the
// agent's identity or its files.
type companyHostServiceAccount struct {
	Name        string
	HomePath    string
	Description string
}

func companyHostServiceAccounts(layout blueclaw.CompanyHostLayout) []companyHostServiceAccount {
	return []companyHostServiceAccount{
		{Name: blueclaw.BlueclawUser, HomePath: layout.AgentHomePath, Description: "internkim agent"},
		{Name: blueclaw.RelayUserName, Description: "internkim relay"},
	}
}

// dscl is the whole of account creation on a Mac, and it is eight commands
// rather than one because the directory service has no adduser. The same eight
// are what blueclaw's POSIX helper runs for a projected person, and the reason
// they are written twice is that the helper is a `package main` in another
// module. A test holds the two orders together.
func (platform macPlatform) EnsureServiceAccounts(machine Machine) error {
	for _, account := range companyHostServiceAccounts(platform.Layout()) {
		if _, errorValue := machine.Output("dscl", []string{".", "-read", "/Users/" + account.Name, "UniqueID"}); errorValue == nil {
			continue
		}
		identityID, errorValue := firstFreeServiceAccountID(machine)
		if errorValue != nil {
			return errorValue
		}
		for _, arguments := range macAccountCommands(account, identityID) {
			if errorValue := machine.Run("dscl", arguments, nil, io.Discard); errorValue != nil {
				return fmt.Errorf("the %s account the company host runs a service as could not be created: %w", account.Name, errorValue)
			}
		}
		if errorValue := machine.Run("dseditgroup", []string{"-o", "edit", "-a", account.Name, "-t", "user", account.Name}, nil, io.Discard); errorValue != nil {
			return fmt.Errorf("the %s account was created and could not be put in its own group: %w", account.Name, errorValue)
		}
	}
	return nil
}

// macAccountCommands is a pure function of the account so a test can read what
// would be created without a directory service to create it in. The user and the
// group take the same number, which is what every macOS service account does.
func macAccountCommands(account companyHostServiceAccount, identityID int) [][]string {
	groupRecord := "/Groups/" + account.Name
	userRecord := "/Users/" + account.Name
	number := strconv.Itoa(identityID)
	homePath := account.HomePath
	if homePath == "" {
		homePath = "/var/empty"
	}
	return [][]string{
		{".", "-create", groupRecord},
		{".", "-create", groupRecord, "PrimaryGroupID", number},
		{".", "-create", userRecord},
		{".", "-create", userRecord, "RealName", account.Description},
		{".", "-create", userRecord, "UserShell", macServiceAccountShell},
		{".", "-create", userRecord, "NFSHomeDirectory", homePath},
		{".", "-create", userRecord, "IsHidden", "1"},
		{".", "-create", userRecord, "PrimaryGroupID", number},
		{".", "-create", userRecord, "UniqueID", number},
	}
}

// A number already in use is a second account silently sharing one identity,
// which on a box whose access boundary is POSIX ownership is the whole boundary
// gone. So the range is scanned and a full one is a refusal.
func firstFreeServiceAccountID(machine Machine) (int, error) {
	taken := map[int]bool{}
	for _, listing := range []struct{ recordType, key string }{
		{"/Users", "UniqueID"},
		{"/Groups", "PrimaryGroupID"},
	} {
		output, errorValue := machine.Output("dscl", []string{".", "-list", listing.recordType, listing.key})
		if errorValue != nil {
			return 0, fmt.Errorf("read what identities this Mac already has: %w", errorValue)
		}
		for _, number := range identityNumbersIn(output) {
			taken[number] = true
		}
	}
	for candidate := firstServiceAccountID; candidate <= lastServiceAccountID; candidate++ {
		if !taken[candidate] {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf(
		"every identity between %d and %d is taken, so a service account here would share a number with something else",
		firstServiceAccountID, lastServiceAccountID)
}

func identityNumbersIn(listing string) []int {
	numbers := []int{}
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		number, errorValue := strconv.Atoi(fields[len(fields)-1])
		if errorValue != nil {
			continue
		}
		numbers = append(numbers, number)
	}
	return numbers
}

func (platform macPlatform) StartTheDatabaseAndTheCache(machine Machine) error {
	formulas := sortedAndUnique([]string{
		macDistributionFormulas[databaseServiceName],
		macDistributionFormulas[cacheServiceName],
	})
	for _, formula := range formulas {
		if errorValue := machine.Run("brew", []string{"services", "start", formula}, nil, io.Discard); errorValue != nil {
			return fmt.Errorf(
				"`brew services start %s` failed. It is one of the two Homebrew commands that may run as root, "+
					"and as root it installs the service into %s so it comes back after a restart: %w",
				formula, blueclaw.CompanyHostLaunchDaemonRoot, errorValue)
		}
	}
	return nil
}

// Homebrew's PostgreSQL runs as the person who installed it and authenticates
// locally, so there is no account to become: psql connects directly. The SQL is
// the same SQL.
func (macPlatform) RunDatabaseStatements(machine Machine, statements string, progress io.Writer) error {
	arguments := []string{
		"-c", `printf '%s' "$` + databasePreparationVariable + `" | psql --set ON_ERROR_STOP=1 --quiet --no-psqlrc --dbname postgres`,
	}
	return machine.Run("sh", arguments, []string{databasePreparationVariable + "=" + statements}, progress)
}

func (platform macPlatform) SuperviseTheBundle(machine Machine, progress io.Writer) error {
	layout := platform.Layout()
	if errorValue := makeTheHelperSetuidRoot(layout, progress); errorValue != nil {
		return errorValue
	}
	return writeAndBootstrapLaunchDaemons(
		blueclaw.CompanyHostLaunchDaemonRoot, layout, readEnvironmentFilesTheServicesName(layout), machine, progress)
}

func writeAndBootstrapLaunchDaemons(daemonRoot string, layout blueclaw.CompanyHostLayout, environmentFiles map[string]string, machine Machine, progress io.Writer) error {
	daemons, errorValue := blueclaw.CompanyHostLaunchDaemons(layout, environmentFiles)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(daemonRoot, 0o755); errorValue != nil {
		return errorValue
	}
	for _, daemon := range daemons {
		path := filepath.Join(daemonRoot, daemon.FileName())
		if errorValue := os.WriteFile(path, []byte(daemon.Contents), 0o644); errorValue != nil {
			return fmt.Errorf("write the %s daemon: %w", daemon.Label, errorValue)
		}
		fmt.Fprintf(progress, "  wrote %s\n", path)
		// bootout first, because bootstrap refuses a label already loaded and
		// an install that runs twice is the ordinary case: a company host is
		// installed again whenever its connection file is downloaded again.
		machine.Run("launchctl", []string{"bootout", "system/" + daemon.Label}, nil, io.Discard)
		if errorValue := machine.Run("launchctl", []string{"bootstrap", "system", path}, nil, progress); errorValue != nil {
			return fmt.Errorf("launchd would not take %s: %w", daemon.Label, errorValue)
		}
	}
	return nil
}

// The setuid bit is what lets the unprivileged agent act as the person who
// asked, and it is the one thing Homebrew cannot deliver: a bottle is a tar
// extracted as an ordinary user and extraction drops the bit, `brew` refuses to
// run as root so the files it writes are owned by a person, and
// `rubocops/caveats.rb` raises on a formula that so much as recommends setuid.
// So the bit is set here, by the step that already has root.
func makeTheHelperSetuidRoot(layout blueclaw.CompanyHostLayout, progress io.Writer) error {
	path := layout.POSIXHelperPath()
	if _, errorValue := os.Stat(path); errorValue != nil {
		return fmt.Errorf(
			"the POSIX helper is not at %s. It is what lets the agent act as the person who asked, "+
				"and without it every terminal command would run as the service account: %w", path, errorValue)
	}
	if errorValue := os.Chown(path, 0, 0); errorValue != nil {
		return fmt.Errorf("give %s to root, which is what its setuid bit is for: %w", path, errorValue)
	}
	if errorValue := os.Chmod(path, os.ModeSetuid|0o755); errorValue != nil {
		return fmt.Errorf("make %s setuid root: %w", path, errorValue)
	}
	fmt.Fprintf(progress, "  %s is setuid root\n", path)
	return nil
}

// launchd reads no environment file, so every value a service's plist carries is
// read here, once, from the files `internkim install` has just written. A file
// that is not there is left out and the renderer decides whether that is a
// failure.
func readEnvironmentFilesTheServicesName(layout blueclaw.CompanyHostLayout) map[string]string {
	contentsByPath := map[string]string{}
	for _, service := range blueclaw.CompanyHostServices(layout) {
		for _, source := range service.Environment {
			if source.FilePath == "" {
				continue
			}
			contents, errorValue := os.ReadFile(source.FilePath)
			if errorValue != nil {
				continue
			}
			contentsByPath[source.FilePath] = string(contents)
		}
	}
	return contentsByPath
}

// On a Mac the one command is brew's; what Homebrew has no formula for is named
// by what it is.
func (macPlatform) HowToInstallTheseByHand(machine Machine, missing []missingPiece) []string {
	formulas := []string{}
	unavailable := []string{}
	for _, piece := range missing {
		if piece.HomebrewFormula != "" {
			formulas = append(formulas, piece.HomebrewFormula)
			continue
		}
		unavailable = append(unavailable, "  "+piece.What+" is missing and Homebrew has nothing that installs it.")
	}
	lines := []string{}
	if len(formulas) > 0 {
		lines = append(lines,
			"  Homebrew carries the rest. Install them, then run this again:",
			"    brew install "+strings.Join(sortedAndUnique(formulas), " "))
	}
	return append(lines, sortedAndUnique(unavailable)...)
}

func (macPlatform) CarriesItInThePackage(blueclaw.HostDependency) bool {
	return false
}

func (macPlatform) WhereToLookFor(dependency blueclaw.HostDependency) []string {
	return dependency.MacFilePathCandidates
}

func (macPlatform) SupervisorIdentityFor(serviceName string) string {
	if formula, isDistributions := macDistributionFormulas[serviceName]; isDistributions {
		return "system/" + blueclaw.CompanyHostLaunchDaemonLabelPrefix + formula
	}
	return "system/" + blueclaw.CompanyHostLaunchDaemonLabel(serviceName)
}

// launchd has no `is-active`. What `launchctl print` carries is the job's state
// and how it last exited, and both matter: a job that is restarting every second
// and a job that ran once and is done are both "not running" to anything
// coarser.
func (macPlatform) SupervisorStateOf(machine Machine, identity string) string {
	printed, errorValue := machine.Output("launchctl", []string{"print", identity})
	if errorValue != nil {
		return "not loaded"
	}
	return strings.Join(launchdStateLines(printed), ", ")
}

func launchdStateLines(printed string) []string {
	wanted := map[string]bool{"state": true, "last exit code": true, "pid": true}
	found := []string{}
	for _, line := range strings.Split(printed, "\n") {
		name, value, isSetting := strings.Cut(strings.TrimSpace(line), "=")
		if !isSetting || !wanted[strings.TrimSpace(name)] {
			continue
		}
		found = append(found, strings.TrimSpace(name)+" "+strings.TrimSpace(value))
	}
	sort.Strings(found)
	if len(found) == 0 {
		return []string{"loaded"}
	}
	return found
}

func (macPlatform) HowToSeeWhyItIsSilent(identity string) []string {
	label := identity[strings.LastIndex(identity, "/")+1:]
	logPath := filepath.Join(blueclaw.CompanyHostLogPath, strings.TrimPrefix(label, blueclaw.CompanyHostLaunchDaemonLabelPrefix)+".log")
	return []string{
		"  what it is doing:  launchctl print " + identity,
		"  why it is not:     tail -n 50 " + logPath,
	}
}
