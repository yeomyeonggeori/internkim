package browser

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const deviceBrowserStartTimeout = 20 * time.Second
const deviceBrowserPortReleaseTimeout = 5 * time.Second
const deviceBrowserStopTimeout = 5 * time.Second
const deviceBrowserPollInterval = 100 * time.Millisecond
const deviceBrowserDiagnosticBytes = 4096

type deviceBrowserProcess struct {
	command   *exec.Cmd
	exited    chan struct{}
	exitError error
}

func (process *deviceBrowserProcess) Exited() <-chan struct{} {
	return process.exited
}

func (process *deviceBrowserProcess) Stop() {
	_ = process.command.Process.Signal(syscall.SIGTERM)
	select {
	case <-process.exited:
	case <-time.After(deviceBrowserStopTimeout):
		_ = process.command.Process.Kill()
		<-process.exited
	}
}

func LaunchDeviceBrowserProcess(ctx context.Context, launch DeviceBrowserLaunch) (RunningDeviceBrowser, error) {
	owner, errorValue := deviceBrowserOwnerOf(launch.UserName)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := prepareDeviceBrowserDirectories(launch, owner); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := waitForFreePort(ctx, launch.Port); errorValue != nil {
		return nil, errorValue
	}
	process, errorValue := startDeviceBrowserProcess(launch, owner)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := waitForDevtools(ctx, launch.Port, process.exited); errorValue != nil {
		process.Stop()
		return nil, deviceBrowserStartupError(launch, process, errorValue)
	}
	return process, nil
}

func prepareDeviceBrowserDirectories(launch DeviceBrowserLaunch, owner *deviceBrowserOwner) error {
	membersDirectory := filepath.Dir(launch.MemberDirectory)
	if errorValue := os.MkdirAll(membersDirectory, 0o711); errorValue != nil {
		return fmt.Errorf("the device browser directory %s could not be made: %w", membersDirectory, errorValue)
	}
	for _, directory := range []string{launch.MemberDirectory, launch.ProfileDirectory, launch.CacheDirectory} {
		if errorValue := os.MkdirAll(directory, 0o700); errorValue != nil {
			return fmt.Errorf("the device browser directory %s could not be made: %w", directory, errorValue)
		}
		if errorValue := owner.own(directory); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func startDeviceBrowserProcess(launch DeviceBrowserLaunch, owner *deviceBrowserOwner) (*deviceBrowserProcess, error) {
	logFile, errorValue := os.OpenFile(launch.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if errorValue != nil {
		return nil, fmt.Errorf("the device browser log %s could not be opened: %w", launch.LogPath, errorValue)
	}
	if errorValue := owner.own(launch.LogPath); errorValue != nil {
		logFile.Close()
		return nil, errorValue
	}
	command := exec.Command(launch.ExecutablePath, "serve",
		"--host", "127.0.0.1", "--port", strconv.Itoa(launch.Port), "--layout", "--resource",
		"--profile-dir", launch.ProfileDirectory, "--http-cache-dir", launch.CacheDirectory)
	command.Env = append(os.Environ(), "HOME="+launch.MemberDirectory)
	command.Stdout = logFile
	command.Stderr = logFile
	command.SysProcAttr = deviceBrowserProcessAttributes(owner)
	if errorValue := command.Start(); errorValue != nil {
		logFile.Close()
		return nil, fmt.Errorf("the device browser %s could not start: %w", launch.ExecutablePath, errorValue)
	}
	process := &deviceBrowserProcess{command: command, exited: make(chan struct{})}
	go func() {
		process.exitError = command.Wait()
		logFile.Close()
		close(process.exited)
	}()
	return process, nil
}

func deviceBrowserStartupError(launch DeviceBrowserLaunch, process *deviceBrowserProcess, startupError error) error {
	diagnostics, logError := deviceBrowserDiagnostics(launch.LogPath)
	if logError != nil {
		diagnostics = "browser diagnostics could not be read: " + logError.Error()
	}
	if diagnostics == "" {
		diagnostics = "the browser wrote no startup diagnostics"
	}
	if process.exitError != nil {
		diagnostics = process.exitError.Error() + "; " + diagnostics
	}
	return fmt.Errorf("the device browser on port %d did not start: %w; %s", launch.Port, startupError, diagnostics)
}

func deviceBrowserDiagnostics(logPath string) (string, error) {
	file, errorValue := os.Open(logPath)
	if errorValue != nil {
		return "", errorValue
	}
	defer file.Close()
	information, errorValue := file.Stat()
	if errorValue != nil {
		return "", errorValue
	}
	buffer := make([]byte, min(information.Size(), deviceBrowserDiagnosticBytes))
	if _, errorValue := file.ReadAt(buffer, information.Size()-int64(len(buffer))); errorValue != nil {
		return "", errorValue
	}
	return strings.TrimSpace(string(buffer)), nil
}

func waitForFreePort(ctx context.Context, port int) error {
	deadline := time.Now().Add(deviceBrowserPortReleaseTimeout)
	for {
		listener, errorValue := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if errorValue == nil {
			return listener.Close()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("port %d is still taken by another process", port)
		}
		if errorValue := sleepOrDone(ctx, deviceBrowserPollInterval); errorValue != nil {
			return errorValue
		}
	}
}

func waitForDevtools(ctx context.Context, port int, exited <-chan struct{}) error {
	versionURL := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	deadline := time.Now().Add(deviceBrowserStartTimeout)
	for {
		if devtoolsAnswers(ctx, versionURL) {
			return nil
		}
		select {
		case <-exited:
			return errors.New("the browser exited while starting")
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s did not answer within %s", versionURL, deviceBrowserStartTimeout)
		}
		if errorValue := sleepOrDone(ctx, deviceBrowserPollInterval); errorValue != nil {
			return errorValue
		}
	}
}

func devtoolsAnswers(ctx context.Context, versionURL string) bool {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, nil)
	if errorValue != nil {
		return false
	}
	response, errorValue := deviceBrowserHTTPClient.Do(request)
	if errorValue != nil {
		return false
	}
	response.Body.Close()
	return response.StatusCode == http.StatusOK
}

func sleepOrDone(ctx context.Context, delay time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}
