package companyhost

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordedCommands struct {
	operatingSystem string
	runs            [][]string
}

func (commands *recordedCommands) Run(name string, arguments []string, environment []string, output io.Writer) error {
	commands.runs = append(commands.runs, append([]string{name}, arguments...))
	return nil
}

func (commands *recordedCommands) Output(name string, arguments []string) (string, error) {
	if len(arguments) > 0 && arguments[0] == "info" {
		return commands.operatingSystem + "\n", nil
	}
	return "Docker Compose version v2.0.0\n", nil
}

func installationRequest(t *testing.T) (Request, string) {
	t.Helper()
	root := t.TempDir()
	connectionPath := filepath.Join(root, "internkim-host.json")
	if errorValue := os.WriteFile(connectionPath, documentWith(func(map[string]any) {}), 0o600); errorValue != nil {
		t.Fatalf("write the connection file: %v", errorValue)
	}
	return Request{
		ConnectionPath:     connectionPath,
		StateDirectoryPath: filepath.Join(root, "state"),
		ModelKey:           "model-key",
	}, filepath.Join(root, "state")
}

func withAgentImage(t *testing.T, reference string) {
	t.Helper()
	previous := AgentImage
	AgentImage = reference
	t.Cleanup(func() { AgentImage = previous })
}

func readStateFiles(t *testing.T, directoryPath string) map[string]string {
	t.Helper()
	files := map[string]string{}
	errorValue := filepath.Walk(directoryPath, func(path string, information os.FileInfo, walkError error) error {
		if walkError != nil || information.IsDir() {
			return walkError
		}
		content, readError := os.ReadFile(path)
		if readError != nil {
			return readError
		}
		relative, _ := filepath.Rel(directoryPath, path)
		files[relative] = string(content)
		return nil
	})
	if errorValue != nil {
		t.Fatalf("read the installation directory: %v", errorValue)
	}
	return files
}

func TestASecondInstallKeepsEveryKeyAndSettingTheFirstOneMade(t *testing.T) {
	withAgentImage(t, "registry.example.test/company-host:one")
	request, directoryPath := installationRequest(t)
	commands := &recordedCommands{operatingSystem: "linux"}
	if _, errorValue := Install(request, commands, io.Discard); errorValue != nil {
		t.Fatalf("first install: %v", errorValue)
	}
	first := readStateFiles(t, directoryPath)
	if _, errorValue := Install(request, commands, io.Discard); errorValue != nil {
		t.Fatalf("second install: %v", errorValue)
	}
	second := readStateFiles(t, directoryPath)
	for name, content := range first {
		if second[name] != content {
			t.Fatalf("a second install rewrote %s", name)
		}
	}
	if len(second) != len(first) {
		t.Fatalf("a second install changed the installation directory: %v then %v", first, second)
	}
}

func TestTheInstallationKeepsEverySecretUnreadableToOtherAccounts(t *testing.T) {
	withAgentImage(t, "registry.example.test/company-host:one")
	request, directoryPath := installationRequest(t)
	if _, errorValue := Install(request, &recordedCommands{operatingSystem: "linux"}, io.Discard); errorValue != nil {
		t.Fatalf("install: %v", errorValue)
	}
	for _, name := range []string{"", "secrets"} {
		information, errorValue := os.Stat(filepath.Join(directoryPath, name))
		if errorValue != nil {
			t.Fatalf("read the directory mode: %v", errorValue)
		}
		if information.Mode().Perm() != 0o700 {
			t.Fatalf("%q is %v, not 0700", name, information.Mode().Perm())
		}
	}
	private := []string{
		"connection.json", "compose.env",
		filepath.Join("secrets", "agent-key"), filepath.Join("secrets", "openrouter-key"),
		filepath.Join("secrets", "buzz-key-seed"), filepath.Join("secrets", "postgres-password"),
		filepath.Join("secrets", "buzz-relay-key"), filepath.Join("secrets", "media-access-key"),
		filepath.Join("secrets", "media-secret-key"), filepath.Join("secrets", "buzz-database.env"),
		filepath.Join("secrets", "buzz-relay.env"),
	}
	for _, name := range private {
		information, errorValue := os.Stat(filepath.Join(directoryPath, name))
		if errorValue != nil {
			t.Fatalf("read %s: %v", name, errorValue)
		}
		if information.Mode().Perm() != 0o600 {
			t.Fatalf("%s is %v, not 0600", name, information.Mode().Perm())
		}
	}
}

func TestTheDockerCommandLineCarriesNoSecret(t *testing.T) {
	withAgentImage(t, "registry.example.test/company-host:one")
	request, directoryPath := installationRequest(t)
	commands := &recordedCommands{operatingSystem: "linux"}
	if _, errorValue := Install(request, commands, io.Discard); errorValue != nil {
		t.Fatalf("install: %v", errorValue)
	}
	password, errorValue := os.ReadFile(filepath.Join(directoryPath, "secrets", "postgres-password"))
	if errorValue != nil {
		t.Fatalf("read the generated password: %v", errorValue)
	}
	for _, run := range commands.runs {
		if strings.Contains(strings.Join(run, " "), strings.TrimSpace(string(password))) {
			t.Fatalf("a secret reached the command line: %v", run)
		}
	}
}

func TestInstallRefusesADirectoryThatBelongsToAnotherCompany(t *testing.T) {
	withAgentImage(t, "registry.example.test/company-host:one")
	request, _ := installationRequest(t)
	if _, errorValue := Install(request, &recordedCommands{operatingSystem: "linux"}, io.Discard); errorValue != nil {
		t.Fatalf("install: %v", errorValue)
	}
	other := documentWith(func(document map[string]any) {
		document["company"] = map[string]any{"id": "00000000-0000-4000-8000-000000000009", "name": "Other Co", "slug": "other"}
	})
	if errorValue := os.WriteFile(request.ConnectionPath, other, 0o600); errorValue != nil {
		t.Fatalf("write the other company's connection file: %v", errorValue)
	}
	_, errorValue := Install(request, &recordedCommands{operatingSystem: "linux"}, io.Discard)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "another company") {
		t.Fatalf("installing over another company's directory returned %v", errorValue)
	}
}

func TestInstallRefusesWhenTheBuildCarriesNoPublishedServerImage(t *testing.T) {
	withAgentImage(t, "")
	request, _ := installationRequest(t)
	_, errorValue := Install(request, &recordedCommands{operatingSystem: "linux"}, io.Discard)
	if errorValue == nil {
		t.Fatal("an unpublished build installed anyway")
	}
	for _, lever := range []string{"make build-company-host-image build-company-host", "internkim release host"} {
		if !strings.Contains(errorValue.Error(), lever) {
			t.Fatalf("the refusal does not name %q: %v", lever, errorValue)
		}
	}
}

func TestInstallRefusesDockerWithoutLinuxContainers(t *testing.T) {
	withAgentImage(t, "registry.example.test/company-host:one")
	request, _ := installationRequest(t)
	_, errorValue := Install(request, &recordedCommands{operatingSystem: "windows"}, io.Discard)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "Linux containers") {
		t.Fatalf("Windows containers were accepted: %v", errorValue)
	}
}

func TestComposeEnvironmentQuotesDollarAndApostropheValues(t *testing.T) {
	rendered := composeEnvironmentText([]EnvironmentEntry{{"DOLLAR", "value$HOME"}, {"APOSTROPHE", "Kim's key"}})
	if rendered != "DOLLAR='value$HOME'\nAPOSTROPHE='Kim\\'s key'\n" {
		t.Fatalf("compose environment rendered as %q", rendered)
	}
}

type failingCommands struct {
	dockerOutput string
	failure      error
}

func (commands *failingCommands) Run(name string, arguments []string, environment []string, output io.Writer) error {
	fmt.Fprint(output, commands.dockerOutput)
	return commands.failure
}

func (commands *failingCommands) Output(name string, arguments []string) (string, error) {
	if len(arguments) > 0 && arguments[0] == "info" {
		return "linux\n", nil
	}
	return "v2\n", nil
}

func TestAServerThatWillNotStartNamesTheImageItRunsAndWhatToDoNext(t *testing.T) {
	withAgentImage(t, "registry.example.test/company-host:20260922T000000Z-abc123")
	request, directoryPath := installationRequest(t)
	dockerOutput := "Error response from daemon: manifest unknown\n"
	commands := &failingCommands{dockerOutput: dockerOutput, failure: errors.New("exit status 18")}
	var progress bytes.Buffer

	_, errorValue := Install(request, commands, &progress)

	if errorValue == nil {
		t.Fatal("a server that never started reported success")
	}
	if !strings.Contains(progress.String(), dockerOutput) {
		t.Fatalf("docker's own output never reached the customer: %q", progress.String())
	}
	for _, named := range []string{
		"registry.example.test/company-host:20260922T000000Z-abc123",
		"exit status 18",
		"curl -fsSL https://intern.kim/install.sh | sh -s -- host",
		"make build-company-host-image build-company-host",
		directoryPath,
	} {
		if !strings.Contains(errorValue.Error(), named) {
			t.Fatalf("the failure does not name %q:\n%s", named, errorValue)
		}
	}
}
