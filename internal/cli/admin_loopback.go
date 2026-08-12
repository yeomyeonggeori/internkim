package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const adminLoopbackBaseURL = "http://127.0.0.1:18080"

func adminLoopbackCommand(method string, path string, body []byte) string {
	request := []string{
		"curl", "--silent", "--show-error", "--fail-with-body",
		"--request", strings.ToUpper(method),
	}
	if len(body) > 0 {
		request = append(request,
			"--header", "'Content-Type: application/json'",
			"--data-binary", "@-",
		)
	}
	request = append(request, adminLoopbackBaseURL+path)
	command := strings.Join(request, " ")
	if len(body) == 0 {
		return command
	}
	return fmt.Sprintf("printf %%s %s | base64 -d | %s", base64.StdEncoding.EncodeToString(body), command)
}

func sshpassBinaryPath() string {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		repositoryRootPath, _ = os.Getwd()
	}
	return filepath.Join(repositoryRootPath, "bin", "sshpass")
}

type deviceAdmin struct {
	connection *sshClient
}

func reachDeviceAdmin(target commandTarget) (deviceAdmin, error) {
	connection, _, errorValue := resolveDeviceSSHConnection(loadConfig(), sshpassBinaryPath(), target)
	if errorValue != nil {
		return deviceAdmin{}, errorValue
	}
	return deviceAdmin{connection: connection}, nil
}

func (admin deviceAdmin) ask(method string, path string, body []byte, answer any) error {
	document, errorValue := admin.connection.runResult(adminLoopbackCommand(method, path, body))
	if errorValue != nil {
		return fmt.Errorf("%s %s failed: %s: %w", method, path, strings.TrimSpace(document), errorValue)
	}
	if answer == nil {
		return nil
	}
	if errorValue := json.Unmarshal([]byte(document), answer); errorValue != nil {
		return fmt.Errorf("%s %s answered something that is not JSON: %s", method, path, strings.TrimSpace(document))
	}
	return nil
}
