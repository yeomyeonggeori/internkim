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

func macPlatformForTest() macPlatform {
	return macPlatform{homebrewPrefix: homebrewPrefixForTest}
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
	if errorValue := requireWhatTheCompanyHostRuns(macPlatformForTest(), machine); errorValue != nil {
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
	errorValue := requireWhatTheCompanyHostRuns(macPlatformForTest(), machine)
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
	machine := &recordedMachine{missing: map[string]bool{"jq": true}}
	errorValue := requireWhatTheCompanyHostRuns(macPlatformForTest(), machine)
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
	account := companyHostServiceAccounts(blueclaw.MacCompanyHostLayout(homebrewPrefixForTest))[0]
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
func TestThePasswordReachesPostgreSQLThroughTheEnvironmentOnAMacToo(t *testing.T) {
	machine := &recordedMachine{}
	password := strings.Repeat("7", 64)
	if errorValue := prepareDatabases(macPlatformForTest(), machine, companyHostSettings{DatabasePassword: password}, io.Discard); errorValue != nil {
		t.Fatalf("prepare the databases: %v", errorValue)
	}
	ranTheStatements := false
	for _, run := range machine.runs {
		if run[0] != "sh" {
			continue
		}
		ranTheStatements = true
		for _, argument := range run {
			if strings.Contains(argument, password) && !strings.HasPrefix(argument, databasePreparationVariable+"=") {
				t.Fatalf("the password reached the command line: %v", run)
			}
		}
	}
	if !ranTheStatements {
		t.Fatalf("no statements were run: %v", machine.runs)
	}
}

func TestAMacDatabaseIsPreparedWithoutPgvector(t *testing.T) {
	machine := &recordedMachine{}
	if errorValue := prepareDatabases(macPlatformForTest(), machine, companyHostSettings{DatabasePassword: "secret"}, io.Discard); errorValue != nil {
		t.Fatalf("a Homebrew PostgreSQL without pgvector was not prepared: %v", errorValue)
	}
}

// Homebrew refuses every command run as root except --prefix and services. An
// install that reached for `brew install` would fail at the one step that has
// root and leave the box half done.
func TestTheInstallAsksHomebrewForNothingButServices(t *testing.T) {
	machine := &recordedMachine{}
	if errorValue := macPlatformForTest().StartTheDatabaseAndTheCache(machine); errorValue != nil {
		t.Fatal(errorValue)
	}
	started := 0
	for _, run := range machine.runs {
		if run[0] != "brew" {
			continue
		}
		if run[1] != "services" {
			t.Fatalf("the install runs `brew %s` as root, which Homebrew refuses", run[1])
		}
		started++
	}
	if started != 2 {
		t.Fatalf("the database and the cache are two services and %d were started: %v", started, machine.runs)
	}
}

// Every plist has to reach /Library/LaunchDaemons and be bootstrapped into the
// system domain: a daemon loaded into a user domain stops when that person logs
// out, which on a company host is a messenger that goes away overnight.
func TestEveryDaemonIsWrittenAndBootstrappedIntoTheSystemDomain(t *testing.T) {
	daemonRoot := t.TempDir()
	layout := blueclaw.MacCompanyHostLayout(homebrewPrefixForTest)
	machine := &recordedMachine{}
	if errorValue := writeAndBootstrapLaunchDaemons(daemonRoot, layout, environmentFilesForTest(layout), machine, io.Discard); errorValue != nil {
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
	if errorValue := writeAndBootstrapLaunchDaemons(t.TempDir(), layout, environmentFilesForTest(layout), machine, io.Discard); errorValue != nil {
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

// The refusal has to name commands that exist on the machine reading it.
// `journalctl` on a Mac is advice a person cannot follow.
func TestTheWaitTellsAMacToReadLaunchctlRatherThanJournalctl(t *testing.T) {
	previousBudget := waitForTheServerBudget
	waitForTheServerBudget = 0
	t.Cleanup(func() { waitForTheServerBudget = previousBudget })

	machine := &recordedMachine{failures: map[string]error{"pg_isready": errors.New("exit status 2")}}
	errorValue := waitUntilTheServerAnswers(macPlatformForTest(), machine, io.Discard)
	if errorValue == nil {
		t.Fatal("a Mac whose database never answered reported ready")
	}
	if strings.Contains(errorValue.Error(), "journalctl") || strings.Contains(errorValue.Error(), "systemctl") {
		t.Fatalf("a Mac is told to read systemd's log:\n%s", errorValue)
	}
	for _, named := range []string{"launchctl print", "PostgreSQL is not accepting connections", "Nothing was removed"} {
		if !strings.Contains(errorValue.Error(), named) {
			t.Fatalf("the refusal does not name %q:\n%s", named, errorValue)
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
