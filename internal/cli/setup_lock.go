package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const setupLockFileName = "setup.lock"

type setupLockDocument struct {
	PID           int       `json:"pid"`
	Command       string    `json:"command"`
	SelectedSteps string    `json:"selectedSteps"`
	StartedAt     time.Time `json:"startedAt"`
	TargetHost    string    `json:"targetHost"`
	TargetURL     string    `json:"targetURL"`
}

type setupLockHandle struct {
	path string
}

func acquireSetupLock(stateDir string, document setupLockDocument, wait bool) (*setupLockHandle, error) {
	if stateDir == "" {
		return nil, errors.New("setup state directory is empty")
	}
	lockPath := filepath.Join(stateDir, setupLockFileName)
	document.PID = os.Getpid()
	document.StartedAt = time.Now().UTC()
	for {
		handle, errorValue := tryAcquireSetupLock(lockPath, document)
		if errorValue == nil {
			return handle, nil
		}
		if !errors.Is(errorValue, os.ErrExist) {
			return nil, errorValue
		}
		if removeStaleSetupLock(lockPath) {
			continue
		}
		if !wait {
			return nil, formatSetupLockError(lockPath)
		}
		time.Sleep(time.Second)
	}
}

func tryAcquireSetupLock(lockPath string, document setupLockDocument) (*setupLockHandle, error) {
	if errorValue := os.MkdirAll(filepath.Dir(lockPath), 0o700); errorValue != nil {
		return nil, errorValue
	}
	file, errorValue := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errorValue != nil {
		return nil, errorValue
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if errorValue := encoder.Encode(document); errorValue != nil {
		_ = file.Close()
		_ = os.Remove(lockPath)
		return nil, errorValue
	}
	if errorValue := file.Close(); errorValue != nil {
		_ = os.Remove(lockPath)
		return nil, errorValue
	}
	return &setupLockHandle{path: lockPath}, nil
}

func removeStaleSetupLock(lockPath string) bool {
	document, errorValue := readSetupLockDocument(lockPath)
	if errorValue != nil {
		return false
	}
	if document.PID <= 0 {
		return false
	}
	if processExists(document.PID) {
		return false
	}
	return os.Remove(lockPath) == nil
}

func processExists(pid int) bool {
	errorValue := syscall.Kill(pid, 0)
	return errorValue == nil || errors.Is(errorValue, os.ErrPermission)
}

func readSetupLockDocument(lockPath string) (setupLockDocument, error) {
	var document setupLockDocument
	data, errorValue := os.ReadFile(lockPath)
	if errorValue != nil {
		return document, errorValue
	}
	errorValue = json.Unmarshal(data, &document)
	return document, errorValue
}

func formatSetupLockError(lockPath string) error {
	document, errorValue := readSetupLockDocument(lockPath)
	if errorValue != nil {
		return fmt.Errorf("setup is already running; lock=%s", lockPath)
	}
	return fmt.Errorf(
		"setup is already running; pid=%d command=%q selectedSteps=%q targetHost=%q targetURL=%q startedAt=%s lock=%s",
		document.PID,
		document.Command,
		document.SelectedSteps,
		document.TargetHost,
		document.TargetURL,
		document.StartedAt.Format(time.RFC3339),
		lockPath,
	)
}

func (handle *setupLockHandle) release() {
	if handle == nil || handle.path == "" {
		return
	}
	_ = os.Remove(handle.path)
}

func setupLockSelectedSteps(arguments []string) string {
	parts := []string{}
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "--only", "--from", "--skip":
			if index+1 < len(arguments) {
				parts = append(parts, arguments[index]+"="+arguments[index+1])
				index++
			}
		case "--force", "--force-all":
			parts = append(parts, arguments[index])
		}
	}
	if len(parts) == 0 {
		return "default"
	}
	return strings.Join(parts, " ")
}

func commandLineForSetupLock(arguments []string) string {
	values := append([]string{"internkim", "setup"}, arguments...)
	return strings.Join(values, " ")
}
