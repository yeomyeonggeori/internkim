package machost

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const supervisorProcessIDFileName = "supervisor.pid"

func StartSupervisor(layout Layout) error {
	if processID := RunningSupervisorProcessID(layout); processID != 0 {
		return fmt.Errorf("a supervisor is already running as process %d", processID)
	}
	logFile, errorValue := os.OpenFile(filepath.Join(layout.LogDirectoryPath(), "supervisor.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, deliveryFileMode)
	if errorValue != nil {
		return errorValue
	}
	defer logFile.Close()

	supervisorCommand := exec.Command(layout.SupervisorBinaryPath(), "-runtime", layout.RuntimeConfigurationPath())
	supervisorCommand.Stdout = logFile
	supervisorCommand.Stderr = logFile
	if errorValue := supervisorCommand.Start(); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(supervisorProcessIDPath(layout), []byte(fmt.Sprintf("%d", supervisorCommand.Process.Pid)), deliveryFileMode)
}

func StopSupervisor(layout Layout) error {
	processID := RunningSupervisorProcessID(layout)
	if processID == 0 {
		return nil
	}
	process, errorValue := os.FindProcess(processID)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := process.Signal(os.Interrupt); errorValue != nil && !errors.Is(errorValue, os.ErrProcessDone) {
		return errorValue
	}
	return os.Remove(supervisorProcessIDPath(layout))
}

func RunningSupervisorProcessID(layout Layout) int {
	document, errorValue := os.ReadFile(supervisorProcessIDPath(layout))
	if errorValue != nil {
		return 0
	}
	processID := 0
	if _, errorValue := fmt.Sscanf(strings.TrimSpace(string(document)), "%d", &processID); errorValue != nil {
		return 0
	}
	process, errorValue := os.FindProcess(processID)
	if errorValue != nil {
		return 0
	}
	if errorValue := process.Signal(nil); errorValue != nil {
		return 0
	}
	return processID
}

func supervisorProcessIDPath(layout Layout) string {
	return filepath.Join(layout.InstallRootPath, supervisorProcessIDFileName)
}

func WaitForGuestHealth(hostHTTPListenAddress string, deadline time.Duration) (string, error) {
	healthURL := "http://" + hostHTTPListenAddress + "/admin/api/health"
	lastDetail := "no answer yet"
	for attemptDeadline := time.Now().Add(deadline); time.Now().Before(attemptDeadline); time.Sleep(2 * time.Second) {
		document, errorValue := readGuestHealth(healthURL)
		if errorValue != nil {
			lastDetail = errorValue.Error()
			continue
		}
		lastDetail = document
		if guestHealthIsOK(document) {
			return document, nil
		}
	}
	return lastDetail, fmt.Errorf("the guest did not answer %s within %s: %s", healthURL, deadline, lastDetail)
}

func readGuestHealth(healthURL string) (string, error) {
	healthClient := &http.Client{Timeout: 5 * time.Second}
	response, errorValue := healthClient.Get(healthURL)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	document, errorValue := io.ReadAll(io.LimitReader(response.Body, 8192))
	if errorValue != nil {
		return "", errorValue
	}
	return string(document), nil
}

func guestHealthIsOK(document string) bool {
	health := struct {
		Status string `json:"status"`
	}{}
	if errorValue := json.Unmarshal([]byte(document), &health); errorValue != nil {
		return false
	}
	return health.Status == "ok"
}
