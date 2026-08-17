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
	"syscall"
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
	supervisorCommand.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if errorValue := supervisorCommand.Start(); errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(supervisorProcessIDPath(layout), []byte(fmt.Sprintf("%d", supervisorCommand.Process.Pid)), deliveryFileMode); errorValue != nil {
		return errorValue
	}
	return requireSupervisorSurvivedItsStart(layout, supervisorCommand.Process.Pid)
}

// A supervisor that cannot bind its listen address exits in well under a second, and the
// caller would otherwise be told it started and go on to believe whatever else answers there.
// A child that exits stays a zombie until it is reaped, and a zombie answers signal 0, so its
// own parent has to ask through wait rather than through the liveness check everyone else uses.
func requireSupervisorSurvivedItsStart(layout Layout, processID int) error {
	time.Sleep(time.Second)
	var waitStatus syscall.WaitStatus
	exitedProcessID, errorValue := syscall.Wait4(processID, &waitStatus, syscall.WNOHANG, nil)
	if errorValue == nil && exitedProcessID == processID {
		return fmt.Errorf("the supervisor exited immediately: %s", SupervisorLogTail(layout))
	}
	return nil
}

func SupervisorLogTail(layout Layout) string {
	document, errorValue := os.ReadFile(filepath.Join(layout.LogDirectoryPath(), "supervisor.log"))
	if errorValue != nil {
		return "no supervisor log"
	}
	lines := strings.Split(strings.TrimSpace(string(document)), "\n")
	if len(lines) > 5 {
		lines = lines[len(lines)-5:]
	}
	return strings.Join(lines, "; ")
}

func StopSupervisor(layout Layout) error {
	processID := RunningSupervisorProcessID(layout)
	if processID == 0 {
		return nil
	}
	if errorValue := syscall.Kill(-processID, syscall.SIGINT); errorValue != nil && !errors.Is(errorValue, syscall.ESRCH) {
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
	if errorValue := syscall.Kill(processID, syscall.Signal(0)); errorValue != nil {
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
