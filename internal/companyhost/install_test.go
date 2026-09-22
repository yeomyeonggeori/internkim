package companyhost

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type recordedMachine struct {
	runs     [][]string
	answers  map[string]string
	failures map[string]error
	missing  map[string]bool
}

func (machine *recordedMachine) Run(name string, arguments []string, environment []string, output io.Writer) error {
	run := append([]string{name}, arguments...)
	machine.runs = append(machine.runs, append(run, environment...))
	return machine.failures[name]
}

func (machine *recordedMachine) Output(name string, arguments []string) (string, error) {
	if failure, failing := machine.failures[name]; failing {
		return "", failure
	}
	return machine.answers[name], nil
}

func (machine *recordedMachine) CarriesProgram(programName string) error {
	if machine.missing[programName] {
		return fmt.Errorf("%s: not found", programName)
	}
	return nil
}

func (machine *recordedMachine) CarriesFile(path string) error {
	if machine.missing[path] {
		return fmt.Errorf("%s: not there", path)
	}
	return nil
}

func exampleSecrets() companySecrets {
	return companySecrets{
		IdentitySeed:     strings.Repeat("1", 64),
		DatabasePassword: strings.Repeat("2", 64),
		RelayPrivateKey:  strings.Repeat("3", 64),
		MediaAccessKey:   strings.Repeat("4", 64),
		MediaSecretKey:   strings.Repeat("5", 64),
	}
}

func exampleConnection(t *testing.T) Connection {
	t.Helper()
	connection, errorValue := ParseConnection(documentWith(func(map[string]any) {}))
	if errorValue != nil {
		t.Fatalf("parse the example connection: %v", errorValue)
	}
	return connection
}

// This is the arrangement the package rests on: every unit refuses to start
// until a file exists, and `internkim install` is what writes it. A unit whose
// condition nothing satisfies is a service that never runs on a box that reports
// itself installed.
func TestEveryUnitWaitsOnAFileTheInstallOrTheBundleWrites(t *testing.T) {
	connection := exampleConnection(t)
	directoryPath := DefaultStateDirectoryPath(connection.Company.ID)
	files, errorValue := companyHostFiles(directoryPath, connection, exampleSecrets())
	if errorValue != nil {
		t.Fatalf("render the company's files: %v", errorValue)
	}
	written := map[string]bool{
		filepath.Join(directoryPath, secretDirectoryName, agentKeyFileName): true,
	}
	for _, file := range files {
		written[file.Path] = true
	}
	// internkim-prepare writes these two, and its own condition is the agent key
	// the install writes, so nothing in the chain waits on nobody.
	writtenByThePrepareService := map[string]bool{
		blueclaw.CompanyHostRuntimeDocument: true,
		blueclaw.CompanyHostPolicyDocument:  true,
	}

	for _, unit := range blueclaw.CompanyPackageUnits() {
		condition := conditionPathOf(unit.Contents)
		if condition == "" {
			t.Fatalf("%s starts whether or not this box has a company", unit.Name)
		}
		if writtenByThePrepareService[condition] {
			continue
		}
		if !written[insideTheCompanyDirectory(condition, directoryPath)] {
			t.Fatalf("%s waits on %s and nothing in the install writes it", unit.Name, condition)
		}
	}
}

// The units name /var/lib/internkim/current; the install writes into the
// company's own directory and points that link at it.
func insideTheCompanyDirectory(condition string, directoryPath string) string {
	relative, found := strings.CutPrefix(condition, blueclaw.CompanyHostCurrentPath+"/")
	if !found {
		return condition
	}
	return filepath.Join(directoryPath, relative)
}

func conditionPathOf(unitContents string) string {
	for _, line := range strings.Split(unitContents, "\n") {
		if path, found := strings.CutPrefix(line, "ConditionPathExists="); found {
			return strings.TrimSpace(path)
		}
	}
	return ""
}

// The plan's claim is that there is one unit renderer, and that the unpackaged
// path runs it at install time where the package runs it at build time. A second
// renderer would be a second definition of the same service.
func TestTheInstallWritesTheUnitsThePackageShips(t *testing.T) {
	unitRoot := t.TempDir()
	if errorValue := installServiceUnits(unitRoot, io.Discard); errorValue != nil {
		t.Fatalf("install the units: %v", errorValue)
	}
	for _, unit := range blueclaw.CompanyPackageUnits() {
		written, errorValue := os.ReadFile(filepath.Join(unitRoot, unit.FileName()))
		if errorValue != nil {
			t.Fatalf("the install wrote no %s: %v", unit.FileName(), errorValue)
		}
		if string(written) != unit.Contents {
			t.Fatalf("%s written by the install is not the one the package ships", unit.FileName())
		}
	}
}

// dpkg owns the units on a packaged box, and `dpkg --verify` re-hashes them.
func TestTheInstallLeavesAUnitSomethingElseAlreadyOwns(t *testing.T) {
	unitRoot := t.TempDir()
	owned := filepath.Join(unitRoot, blueclaw.CompanyPackageUnits()[0].FileName())
	if errorValue := os.WriteFile(owned, []byte("installed by dpkg\n"), 0o644); errorValue != nil {
		t.Fatalf("plant the packaged unit: %v", errorValue)
	}
	if errorValue := installServiceUnits(unitRoot, io.Discard); errorValue != nil {
		t.Fatalf("install the units: %v", errorValue)
	}
	kept, _ := os.ReadFile(owned)
	if string(kept) != "installed by dpkg\n" {
		t.Fatalf("the install rewrote a unit dpkg owns: %q", kept)
	}
}

// The preflight is the unpackaged path's substitute for `Depends:`, so it reads
// the same declaration rather than a list of its own.
func TestThePreflightNamesWhatIsMissingAndTheCommandThatInstallsIt(t *testing.T) {
	machine := &recordedMachine{missing: map[string]bool{"chromium": true, "redis-server": true}}
	errorValue := requireWhatTheCompanyHostRuns(machine)
	if errorValue == nil {
		t.Fatal("a machine with no chromium and no redis was accepted")
	}
	for _, named := range []string{"chromium", "redis-server", "sudo apt-get install"} {
		if !strings.Contains(errorValue.Error(), named) {
			t.Fatalf("the refusal does not name %q:\n%s", named, errorValue)
		}
	}
}

func TestThePreflightPassesAMachineThatCarriesEverything(t *testing.T) {
	if errorValue := requireWhatTheCompanyHostRuns(&recordedMachine{}); errorValue != nil {
		t.Fatalf("a complete machine was refused: %v", errorValue)
	}
}

// `systemctl start` returns when a unit is active, and a process answering
// nothing is active too. A person whose PostgreSQL is not accepting connections
// has to be told that, not that the server did not start.
func TestTheWaitNamesTheServiceThatIsSilentAndWhatToRead(t *testing.T) {
	previousBudget := waitForTheServerBudget
	waitForTheServerBudget = 0
	t.Cleanup(func() { waitForTheServerBudget = previousBudget })

	machine := &recordedMachine{
		failures: map[string]error{"pg_isready": errors.New("exit status 2")},
		answers:  map[string]string{"systemctl": "failed"},
	}
	errorValue := waitUntilTheServerAnswers(machine, io.Discard)
	if errorValue == nil {
		t.Fatal("a server whose database never answered reported ready")
	}
	for _, named := range []string{
		"PostgreSQL is not accepting connections",
		"pg_isready",
		"postgresql.service is failed",
		"journalctl -u postgresql.service",
		"Nothing was removed",
	} {
		if !strings.Contains(errorValue.Error(), named) {
			t.Fatalf("the refusal does not name %q:\n%s", named, errorValue)
		}
	}
}

func TestTheWaitReturnsOnceEveryServiceAnswers(t *testing.T) {
	machine := &recordedMachine{answers: map[string]string{"redis-cli": "PONG\n"}}
	if errorValue := waitUntilTheServerAnswers(machine, io.Discard); errorValue != nil {
		t.Fatalf("a machine whose services all answered was refused: %v", errorValue)
	}
}

func TestTheWaitKeepsTheBudgetTheComposeStackHad(t *testing.T) {
	if waitForTheServerBudget != 240*time.Second {
		t.Fatalf("the wait budget is %v and the stack it replaces waited 240s", waitForTheServerBudget)
	}
}

// Every account on this box can read another process's command line, and the
// database password opens everything the company remembers.
func TestThePasswordReachesPostgreSQLThroughTheEnvironmentAndNotACommandLine(t *testing.T) {
	machine := &recordedMachine{}
	password := strings.Repeat("2", 64)
	if errorValue := prepareDatabases(machine, companyHostSettings{DatabasePassword: password}, io.Discard); errorValue != nil {
		t.Fatalf("prepare the databases: %v", errorValue)
	}
	prepared := false
	for _, run := range machine.runs {
		if run[0] != "runuser" {
			continue
		}
		prepared = true
		for _, argument := range run {
			if strings.Contains(argument, password) && !strings.HasPrefix(argument, databasePreparationVariable+"=") {
				t.Fatalf("the password reached the command line of %v", run[0])
			}
		}
	}
	if !prepared {
		t.Fatal("nothing created the role and the databases")
	}
}

func TestTheDatabasePreparationRunsTwiceWithoutFailing(t *testing.T) {
	statements := databasePreparationStatements("secret")
	for _, expected := range []string{
		"IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'internkim')",
		"ALTER ROLE internkim WITH LOGIN PASSWORD 'secret'",
		"WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'blueclaw')",
		"WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'buzz')",
	} {
		if !strings.Contains(statements, expected) {
			t.Fatalf("the preparation does not carry %q:\n%s", expected, statements)
		}
	}
}

func TestAPasswordCarryingAnApostropheIsQuotedRatherThanEndingTheStatement(t *testing.T) {
	statements := databasePreparationStatements("it's")
	if !strings.Contains(statements, "PASSWORD 'it''s'") {
		t.Fatalf("an apostrophe was not doubled:\n%s", statements)
	}
}

// systemd reads an environment file line by line and takes a value to the end of
// it, so a newline would become a second setting rather than part of one.
func TestAnEnvironmentValueSpanningTwoLinesIsRefusedRatherThanWritten(t *testing.T) {
	_, errorValue := environmentFileText([]EnvironmentEntry{{"GATEWAY_URL", "wss://gateway\nDATABASE_URL=elsewhere"}})
	if errorValue == nil {
		t.Fatal("a value carrying a newline was written to a systemd environment file")
	}
}

func TestACompanyInstallsWhereTheUnitsReadIt(t *testing.T) {
	path := DefaultStateDirectoryPath("00000000-0000-4000-8000-000000000001")
	if filepath.Dir(path) != blueclaw.CompanyHostCompaniesRoot {
		t.Fatalf("a company installs into %s and the units read %s", path, blueclaw.CompanyHostCompaniesRoot)
	}
}

// The relay is the one service that runs unprivileged and outlives the agent, so
// the two files it reads are the two the install hands to its account.
func TestTheRelayCanReadTheTwoFilesItsUnitNames(t *testing.T) {
	connection := exampleConnection(t)
	files, errorValue := companyHostFiles(DefaultStateDirectoryPath(connection.Company.ID), connection, exampleSecrets())
	if errorValue != nil {
		t.Fatalf("render the company's files: %v", errorValue)
	}
	owners := map[string]string{}
	for _, file := range files {
		owners[file.Path] = file.Owner
	}
	for _, path := range []string{blueclaw.RelayEnvironmentFilePath, blueclaw.RelayAgentKeyPath} {
		if owners[path] != blueclaw.RelayUserName {
			t.Fatalf("%s is owned by %q and the relay runs as %s", path, owners[path], blueclaw.RelayUserName)
		}
	}
}
