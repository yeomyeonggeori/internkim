package companyhost

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// Every unit the package installs carries a ConditionPathExists over one of the
// files written here, so a box that has the package and no company sits idle
// rather than restarting into a failure it cannot explain. This is the other
// half of that arrangement: the files that turn those conditions true, and the
// symlink the units read them through, because a unit rendered when the package
// was built cannot name a company id nobody had chosen yet.

const (
	databasePasswordFileName  = "postgres-password"
	relayPrivateKeyFileName   = "buzz-relay-key"
	mediaAccessKeyFileName    = "media-access-key"
	mediaSecretKeyFileName    = "media-secret-key"
	messengerDatabaseFileName = "buzz-database.env"
	messengerRelayKeyFileName = "buzz-relay.env"
	messengerMediaFileName    = "buzz-media.env"
	mediaRootCredentialName   = "buzz-media-root.env"
	messengerBridgeFileName   = "chatd.env"
	hostEnvironmentFileName   = "host.env"
	databaseRoleName          = "internkim"
	messengerPlatform         = "buzz"
	messengerBotUserName      = "internkim"
)

func CurrentConnectionPath() string {
	return filepath.Join(blueclaw.CompanyHostCurrentPath, connectionFileName)
}

func DefaultStateDirectoryPath(companyID string) string {
	return filepath.Join(blueclaw.CompanyHostCompaniesRoot, companyID)
}

type EnvironmentEntry struct {
	Name  string
	Value string
}

// companyFile is one file the install puts on this machine. Owner is empty for
// everything only root reads, which is everything but the relay's: it is the one
// service that runs unprivileged and keeps answering when the agent is down.
type companyFile struct {
	Path     string
	Contents string
	Mode     os.FileMode
	Owner    string
}

type companySecrets struct {
	IdentitySeed     string
	DatabasePassword string
	RelayPrivateKey  string
	MediaAccessKey   string
	MediaSecretKey   string
}

type companyHostSettings struct {
	DirectoryPath    string
	SecretsPath      string
	Connection       Connection
	DatabasePassword string
}

func prepareCompanyDirectory(platform companyHostPlatform, machine Machine, directoryPath string, connection Connection, request Request) (companyHostSettings, error) {
	if errorValue := platform.EnsureServiceAccounts(machine); errorValue != nil {
		return companyHostSettings{}, errorValue
	}
	for _, root := range []string{blueclaw.CompanyHostStateRoot, blueclaw.CompanyHostCompaniesRoot} {
		if errorValue := makePrivateDirectory(root); errorValue != nil {
			return companyHostSettings{}, errorValue
		}
	}
	if errorValue := os.MkdirAll(blueclaw.CompanyHostConfigurationRoot, 0o755); errorValue != nil {
		return companyHostSettings{}, errorValue
	}
	secretsPath, errorValue := PrepareStateDirectory(directoryPath, connection)
	if errorValue != nil {
		return companyHostSettings{}, errorValue
	}
	if errorValue := KeepModelKey(secretsPath, request.ModelKey, request.PromptForModelKey); errorValue != nil {
		return companyHostSettings{}, errorValue
	}
	secrets, errorValue := keepCompanySecrets(secretsPath)
	if errorValue != nil {
		return companyHostSettings{}, errorValue
	}
	files, errorValue := companyHostFiles(platform.Layout(), directoryPath, connection, secrets)
	if errorValue != nil {
		return companyHostSettings{}, errorValue
	}
	for _, file := range files {
		if errorValue := writeCompanyFile(machine, file); errorValue != nil {
			return companyHostSettings{}, errorValue
		}
	}
	if errorValue := pointTheUnitsAtThisCompany(directoryPath); errorValue != nil {
		return companyHostSettings{}, errorValue
	}
	return companyHostSettings{
		DirectoryPath:    directoryPath,
		SecretsPath:      secretsPath,
		Connection:       connection,
		DatabasePassword: secrets.DatabasePassword,
	}, nil
}

func keepCompanySecrets(secretsPath string) (companySecrets, error) {
	secrets := companySecrets{}
	for _, kept := range []struct {
		name  string
		value *string
	}{
		{identitySeedFileName, &secrets.IdentitySeed},
		{databasePasswordFileName, &secrets.DatabasePassword},
		{relayPrivateKeyFileName, &secrets.RelayPrivateKey},
		{mediaAccessKeyFileName, &secrets.MediaAccessKey},
		{mediaSecretKeyFileName, &secrets.MediaSecretKey},
	} {
		value, errorValue := KeepSecret(secretsPath, kept.name)
		if errorValue != nil {
			return companySecrets{}, errorValue
		}
		*kept.value = value
	}
	return secrets, nil
}

// companyHostFiles is a pure function of the company and its secrets so a test
// can read what the install would write beside what the units require, without a
// machine to write it on.
func companyHostFiles(layout blueclaw.CompanyHostLayout, directoryPath string, connection Connection, secrets companySecrets) ([]companyFile, error) {
	identity, errorValue := IdentityForSeed(secrets.IdentitySeed)
	if errorValue != nil {
		return nil, errorValue
	}
	secretsPath := filepath.Join(directoryPath, secretDirectoryName)
	environmentFiles := []struct {
		path    string
		owner   string
		entries []EnvironmentEntry
	}{
		{path: filepath.Join(directoryPath, hostEnvironmentFileName), entries: companyEnvironment(layout, secrets.DatabasePassword, connection)},
		{path: filepath.Join(secretsPath, messengerDatabaseFileName), entries: []EnvironmentEntry{
			{"DATABASE_URL", layout.DatabaseURL(databaseRoleName, secrets.DatabasePassword, blueclaw.BuzzRelayDatabaseName)},
		}},
		{path: filepath.Join(secretsPath, messengerRelayKeyFileName), entries: []EnvironmentEntry{
			{"BUZZ_RELAY_PRIVATE_KEY", secrets.RelayPrivateKey},
			{"RELAY_OWNER_PUBKEY", identity.RelayOwnerPublicKey},
		}},
		{path: filepath.Join(secretsPath, messengerMediaFileName), entries: []EnvironmentEntry{
			{"BUZZ_S3_ACCESS_KEY", secrets.MediaAccessKey},
			{"BUZZ_S3_SECRET_KEY", secrets.MediaSecretKey},
		}},
		{path: filepath.Join(secretsPath, mediaRootCredentialName), entries: []EnvironmentEntry{
			{"ROOT_ACCESS_KEY_ID", secrets.MediaAccessKey},
			{"ROOT_SECRET_ACCESS_KEY", secrets.MediaSecretKey},
		}},
		{path: filepath.Join(secretsPath, messengerBridgeFileName), entries: []EnvironmentEntry{
			{"CHATD_BUZZ_PRIVATE_KEY", identity.AgentPrivateKey},
		}},
		{path: blueclaw.RelayEnvironmentFilePath, owner: blueclaw.RelayUserName, entries: relayEnvironment(layout, connection)},
	}
	files := []companyFile{}
	for _, file := range environmentFiles {
		text, errorValue := environmentFileText(file.entries)
		if errorValue != nil {
			return nil, errorValue
		}
		files = append(files, companyFile{Path: file.path, Contents: text, Mode: modeFor(file.owner), Owner: file.owner})
	}
	return append(files, companyFile{
		Path:     blueclaw.RelayAgentKeyPath,
		Contents: connection.AgentKey + "\n",
		Mode:     modeFor(blueclaw.RelayUserName),
		Owner:    blueclaw.RelayUserName,
	}), nil
}

// A file only root reads is 0600. One an unprivileged service reads is 0640 and
// given to that account, which is the narrowest it can be and still be read.
func modeFor(owner string) os.FileMode {
	if owner == "" {
		return 0o600
	}
	return 0o640
}

// companyEnvironment is what every unit of the bundle reads: the addresses only
// this company knows, and the database the agent opens.
func companyEnvironment(layout blueclaw.CompanyHostLayout, password string, connection Connection) []EnvironmentEntry {
	return []EnvironmentEntry{
		{"DATABASE_URL", layout.DatabaseURL(databaseRoleName, password, blueclaw.BlueclawDatabaseName)},
		{"SUPABASE_URL", connection.CentralPlane.ProjectURL},
		{"SUPABASE_PUBLISHABLE_KEY", connection.CentralPlane.PublishableKey},
		{"INTERNKIM_APP_URL", connection.AppURL},
		{"GATEWAY_URL", connection.GatewayURL},
		{"MESSENGER_PLATFORM", messengerPlatform},
		{"CHATD_BOT_USER_NAME", messengerBotUserName},
	}
}

// The relay reads neither the company directory nor the agent's environment
// file, because it runs unprivileged and outlives the agent. Its settings and
// its own copy of the agent key sit in the configuration directory its unit
// already names.
func relayEnvironment(layout blueclaw.CompanyHostLayout, connection Connection) []EnvironmentEntry {
	return []EnvironmentEntry{
		{"SUPABASE_URL", connection.CentralPlane.ProjectURL},
		{"SUPABASE_PUBLISHABLE_KEY", connection.CentralPlane.PublishableKey},
		{"INTERNKIM_APP_URL", connection.AppURL},
		{"GATEWAY_URL", connection.GatewayURL},
		{"MESSENGER_PLATFORM", messengerPlatform},
		{"AGENT_API_KEY_PATH", blueclaw.RelayAgentKeyPath},
		{"CHATD_BASE_URL", blueclaw.CompanyHostChatdEndpoint},
		{"ADMIND_BASE_URL", "http://" + blueclaw.CompanyHostAdmindListenAddress},
		{"ADMIND_SOCKET_PATH", layout.AdmindSocketPath()},
		{"BLUECLAW_ACP_SOCKET_PATH", layout.ACPSocketPath()},
		{"WORKSPACE_ROOT_PATH", blueclaw.CompanyHostWorkspacePath},
	}
}

// environmentFileText is systemd's EnvironmentFile format, which is not a shell
// script: a value runs to the end of the line, so a newline in one would
// silently become a second setting.
func environmentFileText(entries []EnvironmentEntry) (string, error) {
	var rendered strings.Builder
	for _, entry := range entries {
		if strings.ContainsAny(entry.Value, "\r\n") {
			return "", fmt.Errorf("%s cannot be written to a systemd environment file: its value spans more than one line", entry.Name)
		}
		rendered.WriteString(entry.Name + "=" + entry.Value + "\n")
	}
	return rendered.String(), nil
}

func writeCompanyFile(machine Machine, file companyFile) error {
	if errorValue := os.WriteFile(file.Path, []byte(file.Contents), file.Mode); errorValue != nil {
		return errorValue
	}
	if errorValue := os.Chmod(file.Path, file.Mode); errorValue != nil {
		return errorValue
	}
	if file.Owner == "" {
		return nil
	}
	if errorValue := machine.Run("chown", []string{file.Owner + ":" + file.Owner, file.Path}, nil, io.Discard); errorValue != nil {
		return fmt.Errorf("give %s to the %s account that reads it: %w", file.Path, file.Owner, errorValue)
	}
	return nil
}

// The units name /var/lib/internkim/current because a unit rendered when the
// package was built cannot name a company id. Replacing the link is how a box
// moves from one company to another without a single unit changing.
func pointTheUnitsAtThisCompany(directoryPath string) error {
	existing, errorValue := os.Readlink(blueclaw.CompanyHostCurrentPath)
	if errorValue == nil && existing == directoryPath {
		return nil
	}
	if errorValue == nil {
		if errorValue := os.Remove(blueclaw.CompanyHostCurrentPath); errorValue != nil {
			return errorValue
		}
	}
	return os.Symlink(directoryPath, blueclaw.CompanyHostCurrentPath)
}

func makePrivateDirectory(path string) error {
	if errorValue := os.MkdirAll(path, blueclaw.CompanyHostStateRootMode); errorValue != nil {
		return errorValue
	}
	return os.Chmod(path, blueclaw.CompanyHostStateRootMode)
}
