package companyhost

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/host/quickstart"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var AgentImage = ""

const (
	composeFileName            = "compose.yaml"
	messengerDatabaseFileName  = "01-buzz.sql"
	composeEnvironmentFileName = "compose.env"
	composeWaitSeconds         = "240"
	databaseHostPort           = "127.0.0.1:5432"
	databaseUser               = "internkim"
)

type Commands interface {
	Run(name string, arguments []string, environment []string, output io.Writer) error
	Output(name string, arguments []string) (string, error)
}

type EnvironmentEntry struct {
	Name  string
	Value string
}

type Installation struct {
	StateDirectoryPath string
	Connection         Connection
}

type Request struct {
	ConnectionPath     string
	StateDirectoryPath string
	ModelKey           string
	PromptForModelKey  func() (string, error)
}

func Install(request Request, commands Commands, progress io.Writer) (Installation, error) {
	if AgentImage == "" {
		return Installation{}, fmt.Errorf("this build carries no company server image: build one from source with make build-company-host-image build-company-host, or publish one with internkim release host")
	}
	connection, errorValue := ReadConnection(request.ConnectionPath)
	if errorValue != nil {
		return Installation{}, errorValue
	}
	directoryPath, errorValue := resolveStateDirectoryPath(request.StateDirectoryPath, connection.Company.ID)
	if errorValue != nil {
		return Installation{}, errorValue
	}
	fmt.Fprintln(progress, "1/3 Checking Docker…")
	if errorValue := requireLinuxContainers(commands); errorValue != nil {
		return Installation{}, errorValue
	}
	fmt.Fprintln(progress, "2/3 Keeping the company's keys and data on this computer…")
	environment, errorValue := prepareInstallation(directoryPath, connection, request)
	if errorValue != nil {
		return Installation{}, errorValue
	}
	fmt.Fprintln(progress, "3/3 Starting the server and waiting for it to answer…")
	if errorValue := startStack(directoryPath, commands, environment, progress); errorValue != nil {
		return Installation{}, errorValue
	}
	return Installation{StateDirectoryPath: directoryPath, Connection: connection}, nil
}

func resolveStateDirectoryPath(requested, companyID string) (string, error) {
	if requested == "" {
		return DefaultStateDirectoryPath(companyID)
	}
	return filepath.Abs(requested)
}

func requireLinuxContainers(commands Commands) error {
	if _, errorValue := commands.Output("docker", []string{"compose", "version"}); errorValue != nil {
		return fmt.Errorf("install Docker, then run this command again: %w", errorValue)
	}
	operatingSystem, errorValue := commands.Output("docker", []string{"info", "--format", "{{.OSType}}"})
	if errorValue != nil {
		return fmt.Errorf("check that Docker is running and that your account can use it: %w", errorValue)
	}
	if strings.TrimSpace(operatingSystem) != "linux" {
		return fmt.Errorf("start Docker with Linux containers, then run this command again")
	}
	return nil
}

func prepareInstallation(directoryPath string, connection Connection, request Request) ([]EnvironmentEntry, error) {
	privateDirectoryPath, errorValue := PrepareStateDirectory(directoryPath, connection)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := KeepModelKey(privateDirectoryPath, request.ModelKey, request.PromptForModelKey); errorValue != nil {
		return nil, errorValue
	}
	environment, errorValue := composeEnvironment(directoryPath, privateDirectoryPath, connection)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := writePrivateFile(filepath.Join(directoryPath, composeEnvironmentFileName), []byte(composeEnvironmentText(environment))); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := writeStackFile(filepath.Join(directoryPath, composeFileName), quickstart.ComposeFile); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := writeStackFile(filepath.Join(directoryPath, messengerDatabaseFileName), quickstart.MessengerDatabaseFile); errorValue != nil {
		return nil, errorValue
	}
	return environment, nil
}

func writeStackFile(path string, content []byte) error {
	if errorValue := os.WriteFile(path, content, 0o644); errorValue != nil {
		return errorValue
	}
	return os.Chmod(path, 0o644)
}

func composeEnvironment(directoryPath, privateDirectoryPath string, connection Connection) ([]EnvironmentEntry, error) {
	seed, errorValue := KeepSecret(privateDirectoryPath, identitySeedFileName)
	if errorValue != nil {
		return nil, errorValue
	}
	identity, errorValue := IdentityForSeed(seed)
	if errorValue != nil {
		return nil, errorValue
	}
	password, errorValue := KeepSecret(privateDirectoryPath, "postgres-password")
	if errorValue != nil {
		return nil, errorValue
	}
	relayKey, errorValue := KeepSecret(privateDirectoryPath, "buzz-relay-key")
	if errorValue != nil {
		return nil, errorValue
	}
	mediaAccessKey, errorValue := KeepSecret(privateDirectoryPath, "media-access-key")
	if errorValue != nil {
		return nil, errorValue
	}
	mediaSecretKey, errorValue := KeepSecret(privateDirectoryPath, "media-secret-key")
	if errorValue != nil {
		return nil, errorValue
	}
	databaseAddress := fmt.Sprintf("postgres://%s:%s@%s", databaseUser, password, databaseHostPort)
	agentDatabaseURL := databaseAddress + "/" + blueclaw.BlueclawDatabaseName + "?sslmode=disable"
	messengerDatabaseURL := databaseAddress + "/" + blueclaw.BuzzRelayDatabaseName + "?sslmode=disable"
	if errorValue := writePrivateFile(filepath.Join(privateDirectoryPath, "buzz-database.env"), []byte("DATABASE_URL="+messengerDatabaseURL+"\n")); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := writePrivateFile(filepath.Join(privateDirectoryPath, "buzz-relay.env"), []byte("BUZZ_RELAY_PRIVATE_KEY="+relayKey+"\n")); errorValue != nil {
		return nil, errorValue
	}
	return []EnvironmentEntry{
		{"COMPOSE_PROJECT_NAME", "internkim-" + connection.Company.ID},
		{"HOST_STATE_DIRECTORY", directoryPath},
		{"HOST_IMAGE", AgentImage},
		{"BUZZ_IMAGE", quickstart.MessengerImage()},
		{"SUPABASE_URL", connection.CentralPlane.ProjectURL},
		{"SUPABASE_PUBLISHABLE_KEY", connection.CentralPlane.PublishableKey},
		{"INTERNKIM_APP_URL", connection.AppURL},
		{"GATEWAY_URL", connection.GatewayURL},
		{"DATABASE_URL", agentDatabaseURL},
		{"BUZZ_DATABASE_URL", messengerDatabaseURL},
		{"BUZZ_RELAY_PRIVATE_KEY", relayKey},
		{"RELAY_OWNER_PUBKEY", identity.RelayOwnerPublicKey},
		{"CHATD_BUZZ_PRIVATE_KEY", identity.AgentPrivateKey},
		{"MEDIA_ACCESS_KEY", mediaAccessKey},
		{"MEDIA_SECRET_KEY", mediaSecretKey},
	}, nil
}

func composeEnvironmentText(environment []EnvironmentEntry) string {
	var rendered strings.Builder
	for _, entry := range environment {
		rendered.WriteString(entry.Name + "='" + strings.ReplaceAll(entry.Value, "'", "\\'") + "'\n")
	}
	return rendered.String()
}

func startStack(directoryPath string, commands Commands, environment []EnvironmentEntry, progress io.Writer) error {
	arguments := []string{
		"compose",
		"--env-file", filepath.Join(directoryPath, composeEnvironmentFileName),
		"--file", filepath.Join(directoryPath, composeFileName),
		"up", "--detach", "--wait", "--wait-timeout", composeWaitSeconds,
	}
	return commands.Run("docker", arguments, processEnvironment(environment), progress)
}

func processEnvironment(environment []EnvironmentEntry) []string {
	values := os.Environ()
	for _, entry := range environment {
		values = append(values, entry.Name+"="+entry.Value)
	}
	return values
}
