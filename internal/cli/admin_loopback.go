package cli

import (
	"encoding/base64"
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
