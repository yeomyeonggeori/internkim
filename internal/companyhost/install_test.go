package companyhost

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type recordedMachine struct {
	runs     [][]string
	streamed []string
	answers  map[string]string
	printed  map[string]string
	failures map[string]error
	missing  map[string]bool
}

func (machine *recordedMachine) Run(name string, arguments []string, environment []string, output io.Writer) error {
	run := append([]string{name}, arguments...)
	machine.runs = append(machine.runs, append(run, environment...))
	io.WriteString(output, machine.printed[name])
	return machine.failures[name]
}

func (machine *recordedMachine) Stream(name string, arguments []string, streams Streams) error {
	machine.runs = append(machine.runs, append([]string{name}, arguments...))
	if streams.Input != nil {
		input, _ := io.ReadAll(streams.Input)
		machine.streamed = append(machine.streamed, string(input))
	}
	if streams.Output != nil {
		io.WriteString(streams.Output, machine.printed[name])
	}
	return machine.failures[name]
}

func (machine *recordedMachine) ranStatementsCarrying(text string) bool {
	for _, input := range machine.streamed {
		if strings.Contains(input, text) {
			return true
		}
	}
	for _, run := range machine.runs {
		for _, argument := range run {
			if strings.HasPrefix(argument, databasePreparationVariable+"=") && strings.Contains(argument, text) {
				return true
			}
		}
	}
	return false
}

func (machine *recordedMachine) Output(name string, arguments []string) (string, error) {
	machine.runs = append(machine.runs, append([]string{name}, arguments...))
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
	files, errorValue := companyHostFiles(blueclaw.LinuxCompanyHostLayout(), directoryPath, connection, exampleSecrets())
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
		blueclaw.LinuxCompanyHostLayout().RuntimeDocumentPath(): true,
		blueclaw.LinuxCompanyHostLayout().PolicyDocumentPath():  true,
	}

	for _, unit := range blueclaw.CompanyHostSystemdUnits(blueclaw.LinuxCompanyHostLayout()) {
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

// The preflight stands in for the package's dependencies on a binary run without it, so it reads
// the same declaration rather than a list of its own.
func TestThePreflightNamesWhatIsMissingAndTheCommandThatInstallsIt(t *testing.T) {
	machine := &recordedMachine{missing: map[string]bool{"jq": true, "redis-server": true, "valkey-server": true}}
	errorValue := requireWhatTheCompanyHostRuns(linuxPlatform{}, machine)
	if errorValue == nil {
		t.Fatal("a machine with no jq and no redis was accepted")
	}
	for _, named := range []string{"jq", "redis-server", "sudo apt-get install"} {
		if !strings.Contains(errorValue.Error(), named) {
			t.Fatalf("the refusal does not name %q:\n%s", named, errorValue)
		}
	}
}

func TestThePreflightDoesNotAskADebianMachineForWhatThePackageCarries(t *testing.T) {
	machine := &recordedMachine{missing: map[string]bool{
		"/usr/share/fonts/truetype/nanum/NanumGothic.ttf": true,
	}}
	if errorValue := requireWhatTheCompanyHostRuns(linuxPlatform{}, machine); errorValue != nil {
		t.Fatalf("a machine without its own Nanum font was refused for what the package brings: %v", errorValue)
	}
}

func TestThePreflightAsksNoNativeHostForAPythonOfItsOwn(t *testing.T) {
	machine := &recordedMachine{missing: map[string]bool{"python3": true}}
	for _, platform := range []companyHostPlatform{linuxPlatform{}, macPlatformForTest(t)} {
		if errorValue := requireWhatTheCompanyHostRuns(platform, machine); errorValue != nil {
			t.Fatalf("a host was refused for a python3 the package's install step brings:\n%v", errorValue)
		}
	}
}

func TestThePreflightPassesAMachineThatCarriesEverything(t *testing.T) {
	if errorValue := requireWhatTheCompanyHostRuns(linuxPlatform{}, &recordedMachine{}); errorValue != nil {
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
	errorValue := waitUntilTheServerAnswers(linuxPlatform{}, machine, io.Discard)
	if errorValue == nil {
		t.Fatal("a server whose database never answered reported ready")
	}
	for _, named := range []string{
		"PostgreSQL is not accepting connections",
		"pg_isready",
		"internkim-postgresql.service is failed",
		"journalctl -u internkim-postgresql.service",
		"Nothing was removed",
	} {
		if !strings.Contains(errorValue.Error(), named) {
			t.Fatalf("the refusal does not name %q:\n%s", named, errorValue)
		}
	}
}

func TestTheWaitReturnsOnceEveryServiceAnswers(t *testing.T) {
	machine := &recordedMachine{answers: map[string]string{blueclaw.LinuxCompanyHostLayout().DataServicePath(): "PONG\n"}}
	if errorValue := waitUntilTheServerAnswers(linuxPlatform{}, machine, io.Discard); errorValue != nil {
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
	if errorValue := prepareDatabases(linuxPlatform{}, machine, companyHostSettings{DatabasePassword: password}, io.Discard); errorValue != nil {
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

func TestTheRetiredVectorExtensionIsDroppedFromTheAgentsDatabaseByTheClustersSuperuser(t *testing.T) {
	machine := &recordedMachine{}
	if errorValue := prepareDatabases(linuxPlatform{}, machine, companyHostSettings{DatabasePassword: "secret"}, io.Discard); errorValue != nil {
		t.Fatalf("prepare the databases: %v", errorValue)
	}
	if !machine.ranStatementsCarrying("\\connect blueclaw\nSET client_min_messages = warning;\nDROP EXTENSION IF EXISTS vector CASCADE;") {
		t.Fatalf("nothing dropped the vector extension from the agent's database, so its backups keep needing pgvector to restore: %v", machine.runs)
	}
	if machine.ranStatementsCarrying("CREATE EXTENSION") {
		t.Fatal("the install created an extension, and every one the schemas need is created by their own migrations")
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
	files, errorValue := companyHostFiles(blueclaw.LinuxCompanyHostLayout(), DefaultStateDirectoryPath(connection.Company.ID), connection, exampleSecrets())
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

func TestTheBundleIsRestartedWithoutTheUnitsItIsBoundTo(t *testing.T) {
	machine := &recordedMachine{}
	platform := linuxPlatform{}
	if errorValue := platform.SuperviseTheBundle(machine, io.Discard); errorValue != nil {
		t.Fatalf("supervise the bundle: %v", errorValue)
	}
	for _, run := range machine.runs {
		if run[0] != "systemctl" || run[1] != "restart" {
			continue
		}
		for _, argument := range run[2:] {
			if argument == blueclaw.CompanyHostDatabaseServiceName+".service" || argument == blueclaw.CompanyHostCacheServiceName+".service" {
				t.Fatalf("%v restarts %s in the same transaction as the units bound to it", run, argument)
			}
		}
		return
	}
	t.Fatal("the bundle was never restarted")
}

func TestAnInstallRefusesAConnectionThePlaneHandedOverBeforeTouchingTheMachine(t *testing.T) {
	connection := exampleConnection(t)
	connection.GatewayURL = "https://gateway.example.com"
	machine := &recordedMachine{}
	_, errorValue := installOn(linuxPlatform{}, Request{Connection: connection, StateDirectoryPath: t.TempDir()}, machine, io.Discard)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "gatewayURL") {
		t.Fatalf("an install given a gateway that is not a websocket answered %v", errorValue)
	}
	if len(machine.runs) != 0 {
		t.Fatalf("a refused connection still ran %v", machine.runs)
	}
}

func TestAnInstallTrimsTheTrailingSlashOffEveryAddress(t *testing.T) {
	connection, errorValue := validatedConnection(Connection{
		SchemaVersion: connectionSchemaVersion,
		AppURL:        "https://company.example.com/",
		Company:       Company{ID: "00000000-0000-4000-8000-000000000001", Name: "Example Co", Slug: "example"},
		CentralPlane:  CentralPlane{ProjectURL: "https://project.supabase.co/", PublishableKey: "publishable"},
		GatewayURL:    "wss://gateway.example.com/",
		AgentKey:      strings.Repeat("a", 64),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, address := range []string{connection.AppURL, connection.CentralPlane.ProjectURL, connection.GatewayURL} {
		if strings.HasSuffix(address, "/") {
			t.Fatalf("%s kept its trailing slash, so every path joined onto it has two", address)
		}
	}
}

func (machine *recordedMachine) ran(words ...string) bool {
	for _, run := range machine.runs {
		if len(run) >= len(words) && strings.Join(run[:len(words)], "\x00") == strings.Join(words, "\x00") {
			return true
		}
	}
	return false
}

func (machine *recordedMachine) ranProgram(program string) bool {
	for _, run := range machine.runs {
		for _, argument := range run {
			if argument == program {
				return true
			}
		}
	}
	return false
}
