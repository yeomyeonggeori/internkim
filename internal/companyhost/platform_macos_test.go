package companyhost

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const homebrewPrefixForTest = "/opt/homebrew"

func macPlatformForTest(t *testing.T) macPlatform {
	return macPlatform{homebrewPrefix: homebrewPrefixForTest, launchDaemonRoot: t.TempDir()}
}

// A Mac with no Homebrew has no company host to install, and saying so is the
// whole of what this repository can do about it: `brew install internkim` is how
// the programs arrive, and there is nothing to configure without them.
func TestAMacWithoutHomebrewIsRefusedByName(t *testing.T) {
	_, errorValue := platformFor("darwin", "")
	if errorValue == nil {
		t.Fatal("a Mac with no Homebrew was accepted")
	}
	if !strings.Contains(errorValue.Error(), "brew.sh") {
		t.Fatalf("the refusal does not say where to get Homebrew: %v", errorValue)
	}
}

func TestAMachineThatIsNeitherIsRefusedByName(t *testing.T) {
	_, errorValue := platformFor("windows", "")
	if errorValue == nil {
		t.Fatal("a machine with no install was accepted")
	}
	if !strings.Contains(errorValue.Error(), "windows") {
		t.Fatalf("the refusal does not name the machine: %v", errorValue)
	}
}

// The Hangul face on a Mac is a file the system ships, not Debian's path. A
// preflight that asked for NanumGothic's Debian path would refuse every Mac.
func TestAMacWithTheSystemHangulFaceIsNotToldItsFontIsMissing(t *testing.T) {
	machine := &recordedMachine{missing: map[string]bool{
		"/usr/share/fonts/truetype/nanum/NanumGothic.ttf": true,
	}}
	if errorValue := requireWhatTheCompanyHostRuns(macPlatformForTest(t), machine); errorValue != nil {
		t.Fatalf("a Mac with AppleSDGothicNeo was refused:\n%v", errorValue)
	}
}

func TestAMacWithNoHangulFaceIsToldWhichFileWasLookedFor(t *testing.T) {
	machine := &recordedMachine{missing: map[string]bool{}}
	for _, dependency := range blueclaw.HostDependencies() {
		for _, candidate := range dependency.MacFilePathCandidates {
			machine.missing[candidate] = true
		}
	}
	errorValue := requireWhatTheCompanyHostRuns(macPlatformForTest(t), machine)
	if errorValue == nil {
		t.Fatal("a Mac with no Hangul face was accepted")
	}
	if !strings.Contains(errorValue.Error(), "AppleSDGothicNeo") {
		t.Fatalf("the refusal does not name the file it looked for:\n%s", errorValue)
	}
}

// The refusal on a Mac must not offer an apt command, which is the one thing the
// Debian arm exists to offer.
func TestAMacIsNeverToldToRunAptGet(t *testing.T) {
	machine := &recordedMachine{missing: map[string]bool{blueclaw.HomebrewFormulaProgramDirectory(homebrewPrefixForTest, "jq") + "/jq": true}}
	errorValue := requireWhatTheCompanyHostRuns(macPlatformForTest(t), machine)
	if errorValue == nil {
		t.Fatal("a Mac with no jq was accepted")
	}
	if strings.Contains(errorValue.Error(), "apt-get") {
		t.Fatalf("a Mac was told to run apt-get:\n%s", errorValue)
	}
	if !strings.Contains(errorValue.Error(), "brew install jq") {
		t.Fatalf("the refusal does not name the command that installs it:\n%s", errorValue)
	}
}

// A service account that shares a number with something else shares its
// ownership, and ownership is the whole access boundary on this box.
func TestAServiceAccountTakesANumberNothingElseHas(t *testing.T) {
	taken := []string{}
	for number := firstServiceAccountID; number < firstServiceAccountID+3; number++ {
		taken = append(taken, "_something                       "+strconv.Itoa(number))
	}
	machine := &recordedMachine{answers: map[string]string{"dscl": strings.Join(taken, "\n")}}
	identityID, errorValue := firstFreeServiceAccountID(machine)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if identityID != firstServiceAccountID+3 {
		t.Fatalf("the first free number is %d and %d was chosen", firstServiceAccountID+3, identityID)
	}
}

func TestAFullIdentityRangeIsARefusalRatherThanACollision(t *testing.T) {
	taken := []string{}
	for number := firstServiceAccountID; number <= lastServiceAccountID; number++ {
		taken = append(taken, "_something "+strconv.Itoa(number))
	}
	machine := &recordedMachine{answers: map[string]string{"dscl": strings.Join(taken, "\n")}}
	if _, errorValue := firstFreeServiceAccountID(machine); errorValue == nil {
		t.Fatal("a Mac with no free identity was given one anyway")
	}
}

// A service account that can log in, or that shows at the login window, is an
// account a person can be tricked into using. Both are one dscl attribute.
func TestAServiceAccountCannotLogInAndDoesNotShowAtTheLoginWindow(t *testing.T) {
	account := blueclaw.CompanyHostServiceAccounts(blueclaw.MacCompanyHostLayout(homebrewPrefixForTest))[0]
	commands := macAccountCommands(account, firstServiceAccountID)
	rendered := ""
	for _, arguments := range commands {
		rendered += strings.Join(arguments, " ") + "\n"
	}
	for _, required := range []string{
		"UserShell " + macServiceAccountShell,
		"IsHidden 1",
		"UniqueID " + strconv.Itoa(firstServiceAccountID),
		"PrimaryGroupID " + strconv.Itoa(firstServiceAccountID),
	} {
		if !strings.Contains(rendered, required) {
			t.Fatalf("the account is created without %q:\n%s", required, rendered)
		}
	}
	if !strings.Contains(rendered, "-create /Groups/"+account.Name) {
		t.Fatalf("the account's group is never created:\n%s", rendered)
	}
}

// The password opens everything the company remembers, and every account on this
// box can read another process's arguments.
func TestThePasswordReachesPostgreSQLOnItsStandardInputOnAMac(t *testing.T) {
	machine := &recordedMachine{}
	password := strings.Repeat("7", 64)
	if errorValue := prepareDatabases(macPlatformForTest(t), machine, companyHostSettings{DatabasePassword: password}, io.Discard); errorValue != nil {
		t.Fatalf("prepare the databases: %v", errorValue)
	}
	if !machine.ranStatementsCarrying(password) {
		t.Fatalf("no statements carrying the password were run: %v", machine.runs)
	}
	for _, run := range machine.runs {
		for _, argument := range run {
			if strings.Contains(argument, password) {
				t.Fatalf("the password reached the command line: %v", run)
			}
		}
	}
}

func TestAMacDatabaseIsPreparedWithoutPgvector(t *testing.T) {
	machine := &recordedMachine{}
	if errorValue := prepareDatabases(macPlatformForTest(t), machine, companyHostSettings{DatabasePassword: "secret"}, io.Discard); errorValue != nil {
		t.Fatalf("the host's own PostgreSQL without pgvector was not prepared: %v", errorValue)
	}
	if !machine.ranStatementsCarrying("DROP EXTENSION IF EXISTS vector CASCADE;") {
		t.Fatalf("the Mac's agent database kept a vector extension it no longer uses: %v", machine.runs)
	}
}

// Run as root, `brew services` installs the formula's own service into the
// system domain running as root, which PostgreSQL refuses, and takes over the
// postgresql@17 and redis a person on this Mac already runs. The install asks
// Homebrew for nothing at all.
func TestTheInstallNeverAsksHomebrewToStartAService(t *testing.T) {
	machine := &recordedMachine{}
	if errorValue := macPlatformForTest(t).StartTheDatabaseAndTheCache(machine); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, run := range machine.runs {
		if run[0] == "brew" || strings.HasSuffix(run[0], "/brew") {
			t.Fatalf("the install runs `%s` as root", strings.Join(run, " "))
		}
	}
}

func TestTheDatabaseAndTheCacheAreWrittenAndBootstrappedAsTheHostsOwnDaemons(t *testing.T) {
	platform := macPlatformForTest(t)
	machine := &recordedMachine{}
	if errorValue := platform.StartTheDatabaseAndTheCache(machine); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, serviceName := range []string{databaseServiceName, cacheServiceName} {
		label := strings.TrimPrefix(platform.SupervisorIdentityFor(serviceName), "system/")
		path := filepath.Join(platform.launchDaemonRoot, label+".plist")
		if _, errorValue := os.Stat(path); errorValue != nil {
			t.Fatalf("no plist was written for the %s at %s", serviceName, path)
		}
		if !machine.ran("launchctl", "bootstrap", "system", path) {
			t.Fatalf("%s was written and never bootstrapped: %v", label, machine.runs)
		}
	}
}

func TestEachDataDirectoryBelongsToTheAccountThatRunsInIt(t *testing.T) {
	machine := &recordedMachine{}
	if errorValue := macPlatformForTest(t).StartTheDatabaseAndTheCache(machine); errorValue != nil {
		t.Fatal(errorValue)
	}
	for path, account := range map[string]string{
		blueclaw.CompanyHostDatabaseDataPath: blueclaw.CompanyHostDatabaseUser,
		blueclaw.CompanyHostCacheDataPath:    blueclaw.CompanyHostCacheUser,
	} {
		if !machine.ran("install", "-d", "-o", account, "-g", account, "-m", "0700", path) {
			t.Fatalf("%s is not made as %s's alone: %v", path, account, machine.runs)
		}
	}
}

func TestTheClusterIsMadeOnceAsTheDatabasesAccount(t *testing.T) {
	versionFile := blueclaw.CompanyHostDatabaseDataPath + "/PG_VERSION"
	fresh := &recordedMachine{missing: map[string]bool{versionFile: true}}
	if errorValue := macPlatformForTest(t).StartTheDatabaseAndTheCache(fresh); errorValue != nil {
		t.Fatal(errorValue)
	}
	initdb := blueclaw.MacCompanyHostLayout(homebrewPrefixForTest).DatabaseProgram("initdb")
	if !fresh.ran("sudo", "-u", blueclaw.CompanyHostDatabaseUser, "--") || !fresh.ranProgram(initdb) {
		t.Fatalf("a Mac with no cluster did not make one as %s: %v", blueclaw.CompanyHostDatabaseUser, fresh.runs)
	}
	made := &recordedMachine{}
	if errorValue := macPlatformForTest(t).StartTheDatabaseAndTheCache(made); errorValue != nil {
		t.Fatal(errorValue)
	}
	if made.ranProgram(initdb) {
		t.Fatalf("a cluster that exists was made again: %v", made.runs)
	}
}

// Peer authentication on the socket is the database account's own name, so the
// statements are run as that account, on the socket inside its data directory.
func TestTheStatementsConnectAsTheDatabasesAccountOverItsSocket(t *testing.T) {
	machine := &recordedMachine{}
	if errorValue := macPlatformForTest(t).RunDatabaseStatements(machine, "SELECT 1;", io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	layout := blueclaw.MacCompanyHostLayout(homebrewPrefixForTest)
	if !machine.ran("sudo", "-u", blueclaw.CompanyHostDatabaseUser, "--") {
		t.Fatalf("the statements are not run as %s: %v", blueclaw.CompanyHostDatabaseUser, machine.runs)
	}
	run := strings.Join(machine.runs[0], " ")
	for _, required := range []string{
		layout.DatabaseProgram("psql"),
		"--host " + blueclaw.CompanyHostDatabaseDataPath,
		"--port " + layout.DatabasePort(),
		"--username " + blueclaw.CompanyHostDatabaseUser,
	} {
		if !strings.Contains(run, required) {
			t.Fatalf("psql is run without %q: %s", required, run)
		}
	}
}

func TestEveryServiceAccountIsCreatedOnAMac(t *testing.T) {
	machine := &recordedMachine{}
	if errorValue := macPlatformForTest(t).EnsureServiceAccounts(machineWithNoAccounts{machine}); errorValue != nil {
		t.Fatal(errorValue)
	}
	created := map[string]bool{}
	for _, run := range machine.runs {
		if run[0] == "dscl" && len(run) == 4 && run[2] == "-create" && strings.HasPrefix(run[3], "/Users/") {
			created[strings.TrimPrefix(run[3], "/Users/")] = true
		}
	}
	for _, account := range []string{blueclaw.BlueclawUser, blueclaw.RelayUserName, blueclaw.CompanyHostDatabaseUser, blueclaw.CompanyHostCacheUser} {
		if !created[account] {
			t.Fatalf("the %s account is never created: %v", account, machine.runs)
		}
	}
}

type machineWithNoAccounts struct {
	*recordedMachine
}

func (machine machineWithNoAccounts) Output(name string, arguments []string) (string, error) {
	if name == "dscl" && len(arguments) > 1 && arguments[1] == "-read" {
		return "", errors.New("eDSRecordNotFound")
	}
	return machine.recordedMachine.Output(name, arguments)
}

// Every plist has to reach /Library/LaunchDaemons and be bootstrapped into the
// system domain: a daemon loaded into a user domain stops when that person logs
// out, which on a company host is a messenger that goes away overnight.
func TestEveryDaemonIsWrittenAndBootstrappedIntoTheSystemDomain(t *testing.T) {
	daemonRoot := t.TempDir()
	layout := blueclaw.MacCompanyHostLayout(homebrewPrefixForTest)
	machine := &recordedMachine{}
	if errorValue := writeAndBootstrapLaunchDaemons(daemonRoot, bundleDaemonsForTest(t, layout), machine, io.Discard); errorValue != nil {
		t.Fatalf("write the daemons: %v", errorValue)
	}
	for _, service := range blueclaw.CompanyHostServices(layout) {
		label := blueclaw.CompanyHostLaunchDaemonLabel(service.Name)
		if _, errorValue := os.Stat(filepath.Join(daemonRoot, label+".plist")); errorValue != nil {
			t.Fatalf("no plist was written for %s", service.Name)
		}
		bootstrapped := false
		for _, run := range machine.runs {
			if run[0] == "launchctl" && run[1] == "bootstrap" && run[2] == "system" && strings.HasSuffix(run[3], label+".plist") {
				bootstrapped = true
			}
		}
		if !bootstrapped {
			t.Fatalf("%s was written and never bootstrapped: %v", label, machine.runs)
		}
	}
}

// An install that runs twice is the ordinary case, and `launchctl bootstrap`
// refuses a label already loaded.
func TestASecondInstallUnloadsBeforeItLoads(t *testing.T) {
	layout := blueclaw.MacCompanyHostLayout(homebrewPrefixForTest)
	machine := &recordedMachine{}
	if errorValue := writeAndBootstrapLaunchDaemons(t.TempDir(), bundleDaemonsForTest(t, layout), machine, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	for index, run := range machine.runs {
		if run[0] != "launchctl" || run[1] != "bootstrap" {
			continue
		}
		previous := machine.runs[index-1]
		if previous[0] != "launchctl" || previous[1] != "bootout" {
			t.Fatalf("%v is bootstrapped without being booted out first", run)
		}
	}
}

func bundleDaemonsForTest(t *testing.T, layout blueclaw.CompanyHostLayout) []blueclaw.CompanyHostLaunchDaemon {
	t.Helper()
	daemons, errorValue := blueclaw.CompanyHostLaunchDaemons(layout, environmentFilesForTest(layout))
	if errorValue != nil {
		t.Fatalf("render the daemons: %v", errorValue)
	}
	return daemons
}

// The refusal has to name commands that exist on the machine reading it.
// `journalctl` on a Mac is advice a person cannot follow.
func TestTheWaitTellsAMacToReadLaunchctlRatherThanJournalctl(t *testing.T) {
	previousBudget := waitForTheServerBudget
	waitForTheServerBudget = 0
	t.Cleanup(func() { waitForTheServerBudget = previousBudget })

	readiness := blueclaw.MacCompanyHostLayout(homebrewPrefixForTest).DatabaseProgram("pg_isready")
	machine := &recordedMachine{failures: map[string]error{readiness: errors.New("exit status 2")}}
	errorValue := waitUntilTheServerAnswers(macPlatformForTest(t), machine, io.Discard)
	if errorValue == nil {
		t.Fatal("a Mac whose database never answered reported ready")
	}
	if strings.Contains(errorValue.Error(), "journalctl") || strings.Contains(errorValue.Error(), "systemctl") {
		t.Fatalf("a Mac is told to read systemd's log:\n%s", errorValue)
	}
	for _, named := range []string{
		"launchctl print system/kim.intern." + blueclaw.CompanyHostDatabaseServiceName,
		"tail -n 50 " + blueclaw.CompanyHostLogPath + "/" + blueclaw.CompanyHostDatabaseServiceName + ".log",
		"PostgreSQL is not accepting connections on 127.0.0.1:18432",
		"Nothing was removed",
	} {
		if !strings.Contains(errorValue.Error(), named) {
			t.Fatalf("the refusal does not name %q:\n%s", named, errorValue)
		}
	}
}

// The label the readiness hint names is one launchd actually carries.
func TestTheDataServicesAreNamedByTheLabelsTheirDaemonsCarry(t *testing.T) {
	platform := macPlatformForTest(t)
	daemons, errorValue := blueclaw.CompanyHostDataLaunchDaemons(platform.Layout())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	labels := map[string]bool{}
	for _, daemon := range daemons {
		labels["system/"+daemon.Label] = true
	}
	for _, serviceName := range []string{databaseServiceName, cacheServiceName} {
		if identity := platform.SupervisorIdentityFor(serviceName); !labels[identity] {
			t.Fatalf("the %s is called %s, and launchd carries %v", serviceName, identity, labels)
		}
	}
}

// environmentFilesForTest is what the install has on disk by the time it writes
// the plists: every file a service names, carrying the one value an argument
// refers to.
func environmentFilesForTest(layout blueclaw.CompanyHostLayout) map[string]string {
	contentsByPath := map[string]string{}
	for _, service := range blueclaw.CompanyHostServices(layout) {
		for _, source := range service.Environment {
			if source.FilePath == "" {
				continue
			}
			contentsByPath[source.FilePath] = "MESSENGER_PLATFORM=buzz\nINTERNKIM_APP_URL=https://example.test\nSUPABASE_URL=https://example.supabase.test\nSUPABASE_PUBLISHABLE_KEY=publishable\nRELAY_URL=wss://acme.example.test\n"
		}
	}
	return contentsByPath
}
