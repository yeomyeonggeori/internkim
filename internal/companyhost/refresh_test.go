package companyhost

import (
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func refreshedFiles(t *testing.T) map[string]companyFile {
	t.Helper()
	connection := exampleConnection(t)
	files, errorValue := companySettingsFiles(blueclaw.LinuxCompanyHostLayout(), DefaultStateDirectoryPath(connection.Company.ID), connection, exampleSecrets())
	if errorValue != nil {
		t.Fatalf("render the company's settings: %v", errorValue)
	}
	byPath := map[string]companyFile{}
	for _, file := range files {
		byPath[file.Path] = file
	}
	return byPath
}

func inTheCompanyDirectory(t *testing.T, unitPath string) string {
	t.Helper()
	return strings.Replace(unitPath, blueclaw.CompanyHostCurrentPath, DefaultStateDirectoryPath(exampleConnection(t).Company.ID), 1)
}

// systemd.exec(5): "Settings from these files override settings made with
// Environment=", whatever order the two are written in.
func environmentOfUnit(unit string, files map[string]companyFile) map[string]string {
	environment := map[string]string{}
	filePaths := []string{}
	for _, line := range strings.Split(unit, "\n") {
		if assignment, isSetting := strings.CutPrefix(line, "Environment="); isSetting {
			name, value, _ := strings.Cut(assignment, "=")
			environment[name] = value
		}
		if filePath, isFile := strings.CutPrefix(line, "EnvironmentFile="); isFile {
			filePaths = append(filePaths, strings.TrimPrefix(filePath, "-"))
		}
	}
	for _, filePath := range filePaths {
		for _, line := range strings.Split(files[filePath].Contents, "\n") {
			if name, value, isAssignment := strings.Cut(line, "="); isAssignment {
				environment[name] = value
			}
		}
	}
	return environment
}

func TestARefreshPointsTheRelayAtTheSocketsThisReleaseShips(t *testing.T) {
	files := refreshedFiles(t)
	if _, isRewritten := files[blueclaw.RelayEnvironmentFilePath]; !isRewritten {
		t.Fatalf("a refresh leaves %s as the last release wrote it", blueclaw.RelayEnvironmentFilePath)
	}
	layout := blueclaw.LinuxCompanyHostLayout()
	relayUnit := ""
	for _, unit := range blueclaw.CompanyPackageUnits() {
		if unit.Name == blueclaw.RelayServiceName {
			relayUnit = unit.Contents
		}
	}
	environment := environmentOfUnit(relayUnit, files)
	sockets := map[string]string{
		"ADMIND_SOCKET_PATH":       commandArgument(t, layout, blueclaw.AdmindServiceName, "-listen-socket"),
		"BLUECLAW_ACP_SOCKET_PATH": commandArgument(t, layout, blueclaw.BlueclawServiceName, "-acp-socket"),
	}
	for name, daemonPath := range sockets {
		if environment[name] != daemonPath {
			t.Errorf("after a refresh the relay reads %s=%s and the daemon listens at %s", name, environment[name], daemonPath)
		}
	}
}

func TestARefreshRewritesEverySettingsFileTheUnitsRead(t *testing.T) {
	files := refreshedFiles(t)
	for _, unit := range blueclaw.CompanyPackageUnits() {
		for _, line := range strings.Split(unit.Contents, "\n") {
			filePath, isFile := strings.CutPrefix(line, "EnvironmentFile=")
			if !isFile || strings.HasPrefix(filePath, "-"+blueclaw.CompanyHostConfigurationRoot) {
				continue
			}
			filePath = strings.TrimPrefix(filePath, "-")
			if _, isRewritten := files[inTheCompanyDirectory(t, filePath)]; !isRewritten {
				t.Errorf("%s reads %s, which a refresh leaves as the last release wrote it", unit.Name, filePath)
			}
		}
	}
}

func TestARefreshLeavesTheSessionTheBoxRenews(t *testing.T) {
	files := refreshedFiles(t)
	for _, credentialPath := range []string{blueclaw.RelayAgentKeyPath, blueclaw.CompanyHostAgentKeyPath} {
		if _, isRewritten := files[inTheCompanyDirectory(t, credentialPath)]; isRewritten {
			t.Errorf("a refresh writes %s back to the key the connection file carried, over the session the box renewed", credentialPath)
		}
	}
}
