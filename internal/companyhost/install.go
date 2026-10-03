package companyhost

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Installing a company host is giving a machine that already carries the
// programs a company to run them for: the Linux package, or the Homebrew
// formula on a Mac. This writes the files every unit names in
// ConditionPathExists, prepares the databases they open, starts them, and then
// waits the way `docker compose up --wait` used to.

// Machine is what the installer may do to the computer it runs on. Starting a
// process goes through this so a test can read the commands instead of running
// them; reading and writing this machine's own files does not, because a test
// that cannot see the file it wrote is testing nothing.
type Machine interface {
	Run(name string, arguments []string, environment []string, output io.Writer) error
	Stream(name string, arguments []string, streams Streams) error
	Output(name string, arguments []string) (string, error)
	CarriesProgram(programName string) error
	CarriesFile(path string) error
}

type Streams struct {
	Input  io.Reader
	Output io.Writer
	Errors io.Writer
}

type Installation struct {
	StateDirectoryPath string
	Connection         Connection
	// Supervisor is what keeps the server running on this machine, so the
	// closing sentence names the one this person has rather than the one the
	// other kind of box has.
	Supervisor string
}

type Request struct {
	Connection         Connection
	StateDirectoryPath string
	ModelKey           string
	PromptForModelKey  func() (string, error)
}

func Install(request Request, machine Machine, progress io.Writer) (Installation, error) {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return Installation{}, errorValue
	}
	return installOn(platform, request, machine, progress)
}

func installOn(platform companyHostPlatform, request Request, machine Machine, progress io.Writer) (Installation, error) {
	connection, errorValue := validatedConnection(request.Connection)
	if errorValue != nil {
		return Installation{}, errorValue
	}
	directoryPath, errorValue := resolveStateDirectoryPath(request.StateDirectoryPath, connection.Company.ID)
	if errorValue != nil {
		return Installation{}, errorValue
	}

	fmt.Fprintln(progress, "1/5 Checking what this computer already has…")
	if errorValue := requireWhatTheCompanyHostRuns(platform, machine); errorValue != nil {
		return Installation{}, errorValue
	}

	fmt.Fprintln(progress, "2/5 Keeping the company's keys and data on this computer…")
	company, errorValue := prepareCompanyDirectory(platform, machine, directoryPath, connection, request)
	if errorValue != nil {
		return Installation{}, errorValue
	}

	fmt.Fprintln(progress, "3/5 Preparing PostgreSQL…")
	if errorValue := prepareDatabases(platform, machine, company, progress); errorValue != nil {
		return Installation{}, errorValue
	}
	if errorValue := rehomeTheMessengerCommunity(platform, machine, connection, progress); errorValue != nil {
		return Installation{}, errorValue
	}

	fmt.Fprintln(progress, "4/5 Installing and starting the services…")
	if errorValue := platform.SuperviseTheBundle(machine, progress); errorValue != nil {
		return Installation{}, errorValue
	}

	fmt.Fprintln(progress, "5/5 Waiting for the server to answer…")
	if errorValue := waitUntilTheServerAnswers(platform, machine, progress); errorValue != nil {
		return Installation{}, errorValue
	}
	return Installation{
		StateDirectoryPath: directoryPath,
		Connection:         connection,
		Supervisor:         platform.NameOfItsSupervisor(),
	}, nil
}

func validatedConnection(connection Connection) (Connection, error) {
	if errorValue := validateConnection(connection); errorValue != nil {
		return Connection{}, errorValue
	}
	return normalizeConnection(connection), nil
}

// RequireAdministrator makes the refusal a sentence rather than a permission
// error from whichever write happened to come first.
func RequireAdministrator() error {
	if os.Geteuid() == 0 {
		return nil
	}
	return fmt.Errorf("installing the company server writes service definitions and a private state directory, so it needs root. Run the same command with sudo")
}

func resolveStateDirectoryPath(requested, companyID string) (string, error) {
	if requested == "" {
		return DefaultStateDirectoryPath(companyID), nil
	}
	return filepath.Abs(requested)
}
