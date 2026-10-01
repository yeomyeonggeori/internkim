package deviceexport

import (
	"archive/tar"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

// A Jetson leaves the device stack through `recover -action migration-export`,
// which writes the guest's database, the workspace tree, the host's databases,
// the media store and the host's own state into one directory. Convert turns
// that directory into the archive `internkim restore` takes, so moving the
// device is a restore and nothing else.
type Request struct {
	ExportDirectoryPath string
	ConnectionPath      string
	HostPasswdPath      string
	HostGroupPath       string
	ArchivePath         string
	CreatedAt           time.Time
}

type Report struct {
	Manifest        hostbackup.Manifest
	Entries         int
	UnnamedOwners   int
	LeftOut         []string
	CarriedSecrets  []string
	WorkspaceOwners map[string]int
}

const (
	stateRole          = "state"
	relayRole          = "relay"
	administrationRole = "administration"
	workspaceRole      = "workspace"

	guestDatabaseFileName   = "blueclaw.dump"
	hostMessengerFileName   = "host-buzz.dump"
	guestPasswdFileName     = "guest-passwd"
	guestGroupFileName      = "guest-group"
	guestPostgresFileName   = "guest-postgres-version"
	workspaceArchiveName    = "workspace.tar.zst"
	hostStateArchiveName    = "host-state.tar.zst"
	mediaDirectoryName      = "media"
	identityMapArchivedName = ".blueclaw/identity-map.json"

	deviceAdministrationPrefix = "root/.internkim/"
	deviceSecretsPrefix        = "root/.internkim/secrets/"
	deviceChatdPrefix          = "root/.internkim/state/chatd/"
	deviceAccountLinksPath     = "root/.internkim/state/admin/buzz-account-links.json"
	deviceRelayPrefix          = "var/lib/internkim/relay/"
	deviceRosterPath           = "var/lib/blueclaw/delivery/config/policy.json"
)

// What the host keeps under its own secrets, by the device's name for it. The
// relay key sits inside an environment file on the device and is read out of it.
var carriedSecrets = map[string]string{
	"buzz-key-seed":      "buzz-key-seed",
	"openrouter-api-key": "openrouter-key",
}

const (
	deviceRelayEnvironmentName = "buzz-relay-env"
	relayPrivateKeyVariable    = "BUZZ_RELAY_PRIVATE_KEY"
	hostRelayKeyFileName       = "buzz-relay-key"
)

// The device's administration directory, less what the host has no place for or
// writes itself: local models, the old backups, the fleet's environment and
// credentials, the TLS terminator's key. Site credentials stay beside the site
// data they open.
var administrationLeftOut = []string{
	"models", "backups", "env", "tls", "recovery", "state/admin/jobs",
}

var administrationSecretsKept = []string{"secrets/sites", "secrets/google-oauth"}

// Exactly what the product's own backup leaves out of the workspace.
var workspaceLeftOut = []string{"shared/cache", "private/people/*/tmp", ".blueclaw/postgres", ".blueclaw/runtime", ".blueclaw/logs", ".blueclaw/tmp"}

func Convert(request Request) (Report, error) {
	connection, errorValue := companyhost.ReadConnection(request.ConnectionPath)
	if errorValue != nil {
		return Report{}, fmt.Errorf("the connection file: %w", errorValue)
	}
	exported := request.ExportDirectoryPath
	postgresMajor, errorValue := readPostgresMajor(filepath.Join(exported, guestPostgresFileName))
	if errorValue != nil {
		return Report{}, errorValue
	}
	device, errorValue := readDeviceState(filepath.Join(exported, hostStateArchiveName))
	if errorValue != nil {
		return Report{}, errorValue
	}
	guestNames, errorValue := readGuestNames(exported)
	if errorValue != nil {
		return Report{}, errorValue
	}
	hostNames := newAccountNames()
	if errorValue := readAccountFile(request.HostPasswdPath, hostNames.users); errorValue != nil {
		return Report{}, fmt.Errorf("the device's accounts: %w", errorValue)
	}
	if errorValue := readAccountFile(request.HostGroupPath, hostNames.groups); errorValue != nil {
		return Report{}, fmt.Errorf("the device's groups: %w", errorValue)
	}

	writer, errorValue := hostbackup.Create(request.ArchivePath)
	if errorValue != nil {
		return Report{}, errorValue
	}
	report := Report{WorkspaceOwners: map[string]int{}}
	members := []struct {
		name    string
		produce func(io.Writer) error
	}{
		{hostbackup.DatabaseMemberName("blueclaw"), copyFileTo(filepath.Join(exported, guestDatabaseFileName))},
		{hostbackup.DatabaseMemberName("buzz"), copyFileTo(filepath.Join(exported, hostMessengerFileName))},
		{hostbackup.RosterMemberName, func(output io.Writer) error { _, errorValue := output.Write(device.roster); return errorValue }},
		{hostbackup.FilesMemberName, func(output io.Writer) error {
			return writeFiles(output, exported, connection, device, guestNames, hostNames, &report)
		}},
	}
	for _, member := range members {
		if errorValue := writer.AddMember(member.name, member.produce); errorValue != nil {
			writer.Abandon()
			return Report{}, errorValue
		}
	}
	manifest, errorValue := writer.Finish(hostbackup.Manifest{
		CreatedAt:       request.CreatedAt.UTC(),
		CompanyID:       connection.Company.ID,
		PostgreSQLMajor: postgresMajor,
		FileRoots:       []string{stateRole, relayRole, administrationRole, workspaceRole},
	})
	if errorValue != nil {
		return Report{}, errorValue
	}
	report.Manifest = manifest
	report.CarriedSecrets = device.carriedSecretNames()
	return report, nil
}

func readPostgresMajor(path string) (int, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return 0, errorValue
	}
	major, errorValue := strconv.Atoi(strings.TrimSpace(string(document)))
	if errorValue != nil {
		return 0, fmt.Errorf("%s does not hold a PostgreSQL major version: %w", path, errorValue)
	}
	return major, nil
}

func copyFileTo(path string) func(io.Writer) error {
	return func(output io.Writer) error {
		file, errorValue := os.Open(path)
		if errorValue != nil {
			return errorValue
		}
		defer file.Close()
		_, errorValue = io.Copy(output, file)
		return errorValue
	}
}

// deviceState is the little the archive needs from the device's own state
// before the files are written: the roster and the keys the host keeps beside
// its connection.
type deviceState struct {
	roster  []byte
	secrets map[string][]byte
}

func (device deviceState) carriedSecretNames() []string {
	names := []string{}
	for name := range device.secrets {
		names = append(names, name)
	}
	return names
}

func readDeviceState(archivePath string) (deviceState, error) {
	device := deviceState{secrets: map[string][]byte{}}
	errorValue := eachEntry(archivePath, func(header *tar.Header, reader io.Reader) error {
		name := strings.TrimPrefix(header.Name, "./")
		if header.Typeflag != tar.TypeReg {
			return nil
		}
		switch {
		case name == deviceRosterPath:
			document, errorValue := io.ReadAll(reader)
			device.roster = document
			return errorValue
		case name == deviceSecretsPrefix+deviceRelayEnvironmentName:
			key, errorValue := environmentValue(reader, relayPrivateKeyVariable)
			if errorValue != nil {
				return errorValue
			}
			device.secrets[hostRelayKeyFileName] = []byte(key + "\n")
		case strings.HasPrefix(name, deviceSecretsPrefix):
			hostName, isCarried := carriedSecrets[strings.TrimPrefix(name, deviceSecretsPrefix)]
			if !isCarried {
				return nil
			}
			document, errorValue := io.ReadAll(reader)
			device.secrets[hostName] = append(bytes.TrimSpace(document), '\n')
			return errorValue
		}
		return nil
	})
	if errorValue != nil {
		return deviceState{}, errorValue
	}
	if len(device.roster) == 0 {
		return deviceState{}, fmt.Errorf("%s holds no %s, so the people's accounts could not be recreated", archivePath, deviceRosterPath)
	}
	if _, hasSeed := device.secrets["buzz-key-seed"]; !hasSeed {
		return deviceState{}, fmt.Errorf("%s holds no messenger seed; without it every person's messenger key changes", archivePath)
	}
	return device, nil
}

func environmentValue(reader io.Reader, variable string) (string, error) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		name, value, isAssignment := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if isAssignment && strings.TrimPrefix(name, "export ") == variable {
			return strings.Trim(value, `"'`), nil
		}
	}
	if errorValue := scanner.Err(); errorValue != nil {
		return "", errorValue
	}
	return "", fmt.Errorf("the device's relay environment names no %s", variable)
}

func readGuestNames(exported string) (accountNames, error) {
	names := newAccountNames()
	if errorValue := readAccountFile(filepath.Join(exported, guestPasswdFileName), names.users); errorValue != nil {
		return accountNames{}, errorValue
	}
	if errorValue := readAccountFile(filepath.Join(exported, guestGroupFileName), names.groups); errorValue != nil {
		return accountNames{}, errorValue
	}
	found := false
	errorValue := eachEntry(filepath.Join(exported, workspaceArchiveName), func(header *tar.Header, reader io.Reader) error {
		if strings.TrimPrefix(header.Name, "./") != identityMapArchivedName {
			return nil
		}
		document, errorValue := io.ReadAll(reader)
		if errorValue != nil {
			return errorValue
		}
		found = true
		return names.addIdentityMap(document)
	})
	if errorValue != nil {
		return accountNames{}, errorValue
	}
	if !found {
		return accountNames{}, fmt.Errorf("the workspace holds no %s, so its files' owners have no names", identityMapArchivedName)
	}
	return names, nil
}

func eachEntry(archivePath string, visit func(*tar.Header, io.Reader) error) error {
	file, errorValue := os.Open(archivePath)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	decoder, errorValue := zstd.NewReader(file)
	if errorValue != nil {
		return errorValue
	}
	defer decoder.Close()
	reader := tar.NewReader(decoder)
	for {
		header, errorValue := reader.Next()
		if errors.Is(errorValue, io.EOF) {
			return nil
		}
		if errorValue != nil {
			return fmt.Errorf("reading %s: %w", archivePath, errorValue)
		}
		if errorValue := visit(header, reader); errorValue != nil {
			return fmt.Errorf("%s in %s: %w", header.Name, archivePath, errorValue)
		}
	}
}

func isUnder(relative string, patterns []string) bool {
	for _, pattern := range patterns {
		segments := strings.Count(pattern, "/") + 1
		parts := strings.SplitN(relative, "/", segments+1)
		if len(parts) < segments {
			continue
		}
		if isMatched, _ := path.Match(pattern, strings.Join(parts[:segments], "/")); isMatched {
			return true
		}
	}
	return false
}
