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
	"time"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The macOS half. `brew install internkim` delivers the files and this makes the
// box a host: it creates the service accounts through the directory service,
// starts the host's own database and cache, sets the helper's ownership and its
// setuid bit — the one thing a bottle pour cannot carry, because extracting a
// tar as an ordinary user drops the bit and Homebrew's own linter forbids even
// recommending it — writes the nine LaunchDaemons from the same declaration the
// units come from, and then polls readiness where ordering used to stand.
//
// `brew.sh`'s check-run-command-as-root refuses every command run as root
// except `as-console-user`, `setup-sandbox`, `--prefix` and `services`, so this
// asks Homebrew where its prefix is and nothing else. `brew services` is not
// used either: run as root it installs the formula's own service into
// /Library/LaunchDaemons running as root, which PostgreSQL refuses, and takes
// over the postgresql@17 and redis services the person at this Mac may already
// run. The host starts the formulas' binaries as its own daemons instead, the
// way the Linux package runs its own units.

const (
	// Below 500 so the accounts do not appear at the login window, and far
	// below the 100000 the POSIX helper allocates projected identities from,
	// so a company's people can never collide with the services.
	firstServiceAccountID = 450
	lastServiceAccountID  = 499

	macServiceAccountShell = "/usr/bin/false"
)

type macPlatform struct {
	homebrewPrefix   string
	launchDaemonRoot string
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

// dscl is the whole of account creation on a Mac, and it is eight commands
// rather than one because the directory service has no adduser. The same eight
// are what blueclaw's POSIX helper runs for a projected person, and the reason
// they are written twice is that the helper is a `package main` in another
// module. A test holds the two orders together.
func (platform macPlatform) EnsureServiceAccounts(machine Machine) error {
	for _, account := range blueclaw.CompanyHostServiceAccounts(platform.Layout()) {
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
func macAccountCommands(account blueclaw.CompanyHostServiceAccount, identityID int) [][]string {
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

// The database and the cache are this host's own daemons, as their own accounts,
// in their own directories. The cluster is made once, as the account that will
// run it, because PostgreSQL refuses to run as root and refuses a data
// directory it does not own. The install goes on to prepare the database, so
// this returns once it answers.
func (platform macPlatform) StartTheDatabaseAndTheCache(machine Machine) error {
	layout := platform.Layout()
	if errorValue := makeTheDataDirectories(machine); errorValue != nil {
		return errorValue
	}
	if errorValue := makeTheClusterOnce(layout, machine); errorValue != nil {
		return errorValue
	}
	daemons, errorValue := blueclaw.CompanyHostDataLaunchDaemons(layout)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := writeAndBootstrapLaunchDaemons(platform.launchDaemonRoot, daemons, machine, io.Discard); errorValue != nil {
		return errorValue
	}
	return waitForOne(platform, machine, databaseProbe(layout), time.Now().Add(waitForTheServerBudget), io.Discard)
}

// Each data directory is the account's that runs in it. The log directory is
// made here too: launchd opens a daemon's log before it starts the daemon and
// creates no directory to do it, and the data daemons start before the bundle's
// preparation step would have made it.
func makeTheDataDirectories(machine Machine) error {
	directories := []struct {
		path  string
		owner string
		mode  os.FileMode
	}{
		{blueclaw.CompanyHostDatabaseDataPath, blueclaw.CompanyHostDatabaseUser, 0o700},
		{blueclaw.CompanyHostCacheDataPath, blueclaw.CompanyHostCacheUser, 0o700},
		{blueclaw.CompanyHostLogPath, blueclaw.BlueclawUser, blueclaw.CompanyHostLogDirectoryMode},
	}
	for _, directory := range directories {
		arguments := []string{"-d", "-o", directory.owner, "-g", directory.owner, "-m", fmt.Sprintf("%04o", directory.mode), directory.path}
		if errorValue := machine.Run("install", arguments, nil, io.Discard); errorValue != nil {
			return fmt.Errorf("make %s, owned by %s: %w", directory.path, directory.owner, errorValue)
		}
	}
	return nil
}

func makeTheClusterOnce(layout blueclaw.CompanyHostLayout, machine Machine) error {
	if machine.CarriesFile(blueclaw.CompanyHostDatabaseDataPath+"/PG_VERSION") == nil {
		return nil
	}
	initialization := blueclaw.CompanyHostDatabaseInitialization(layout)
	if _, errorValue := machine.Output("sudo", asAnAccountOnAMac(blueclaw.CompanyHostDatabaseUser, initialization...)); errorValue != nil {
		return fmt.Errorf("the company's database could not be made in %s: %w", blueclaw.CompanyHostDatabaseDataPath, errorValue)
	}
	return nil
}

// sudo is how root becomes another account on a Mac, which has no runuser. The
// account's own programs refuse a working directory they cannot read, and the
// directory sudo was run from is usually a person's home.
func asAnAccountOnAMac(account string, command ...string) []string {
	return append([]string{"-u", account, "--", "/bin/sh", "-c", `cd / && exec "$@"`, "sh"}, command...)
}

// The superuser is the database account's own name and signs in by peer on the
// socket in its data directory, so the way in is to become that account. The
// statements reach psql on its standard input, never on a command line, which
// every account on this box can read.
func (platform macPlatform) RunDatabaseStatements(machine Machine, statements string, progress io.Writer) error {
	layout := platform.Layout()
	arguments := asAnAccountOnAMac(blueclaw.CompanyHostDatabaseUser,
		layout.DatabaseProgram("psql"),
		"--host", layout.DatabaseSocketDirectory,
		"--port", layout.DatabasePort(),
		"--username", blueclaw.CompanyHostDatabaseUser,
		"--set", "ON_ERROR_STOP=1", "--quiet", "--no-psqlrc", "--dbname", "postgres")
	return machine.Stream("sudo", arguments, Streams{Input: strings.NewReader(statements), Output: progress, Errors: progress})
}

func (platform macPlatform) SuperviseTheBundle(machine Machine, progress io.Writer) error {
	layout := platform.Layout()
	if errorValue := makeTheHelperSetuidRoot(layout, progress); errorValue != nil {
		return errorValue
	}
	daemons, errorValue := blueclaw.CompanyHostLaunchDaemons(layout, readEnvironmentFilesTheServicesName(layout))
	if errorValue != nil {
		return errorValue
	}
	return writeAndBootstrapLaunchDaemons(platform.launchDaemonRoot, daemons, machine, progress)
}

func writeAndBootstrapLaunchDaemons(daemonRoot string, daemons []blueclaw.CompanyHostLaunchDaemon, machine Machine, progress io.Writer) error {
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

// The keg carries what the Linux packages carry, except what the Mac already
// keeps in a place of its own, such as the system's Hangul face.
func (macPlatform) CarriesItInThePackage(dependency blueclaw.HostDependency) bool {
	return dependency.WhatThePackageCarriesInstead != "" && len(dependency.MacFilePathCandidates) == 0
}

func (macPlatform) WhereToLookFor(dependency blueclaw.HostDependency) []string {
	return dependency.MacFilePathCandidates
}

// A formula's programs are in its opt directory whether it is linked or not,
// and that is where the host starts them from.
func (platform macPlatform) WhereItKeepsTheProgram(dependency blueclaw.HostDependency, programName string) string {
	if dependency.HomebrewFormula == "" {
		return ""
	}
	return blueclaw.HomebrewFormulaProgramDirectory(platform.homebrewPrefix, dependency.HomebrewFormula) + "/" + programName
}

func (macPlatform) SupervisorIdentityFor(serviceName string) string {
	if name, isDataService := companyHostDataServiceNames[serviceName]; isDataService {
		serviceName = name
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
