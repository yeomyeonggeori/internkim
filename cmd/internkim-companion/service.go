package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

const serviceLabel = "kim.intern.companion"

type commandRunner func(name string, arguments ...string) error

type backgroundService interface {
	Install(executablePath string, runArguments []string) error
	Uninstall() error
	Restart() error
	IsRunning() (bool, error)
	Describe() string
}

func runService(arguments []string, standardOutput *os.File) error {
	if len(arguments) == 0 {
		return errors.New("usage: internkim-companion service <install|uninstall|restart|status> [-- run flags]")
	}
	service, errorValue := platformService(runCommandAttached)
	if errorValue != nil {
		return errorValue
	}
	switch arguments[0] {
	case "install":
		return installService(service, arguments[1:], standardOutput)
	case "uninstall":
		if errorValue := service.Uninstall(); errorValue != nil {
			return errorValue
		}
		fmt.Fprintln(standardOutput, "removed "+service.Describe())
		return nil
	case "restart":
		if errorValue := service.Restart(); errorValue != nil {
			return errorValue
		}
		fmt.Fprintln(standardOutput, "restarted "+service.Describe())
		return nil
	case "status":
		isRunning, errorValue := service.IsRunning()
		if errorValue != nil {
			return errorValue
		}
		fmt.Fprintln(standardOutput, serviceStatusLabel(isRunning)+" ("+service.Describe()+")")
		return nil
	}
	return fmt.Errorf("unknown service command: %s", arguments[0])
}

func installService(service backgroundService, arguments []string, standardOutput *os.File) error {
	flags := flag.NewFlagSet("service install", flag.ContinueOnError)
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return errorValue
	}
	if errorValue := service.Install(executablePath, flags.Args()); errorValue != nil {
		return errorValue
	}
	fmt.Fprintln(standardOutput, "installed "+service.Describe())
	return nil
}

func serviceStatusLabel(isRunning bool) string {
	if isRunning {
		return "running"
	}
	return "not running"
}

func currentExecutablePath() (string, error) {
	executablePath, errorValue := os.Executable()
	if errorValue != nil {
		return "", errorValue
	}
	return filepath.EvalSymlinks(executablePath)
}

func platformService(run commandRunner) (backgroundService, error) {
	homeDirectory, errorValue := os.UserHomeDir()
	if errorValue != nil {
		return nil, errorValue
	}
	switch runtime.GOOS {
	case "darwin":
		currentUser, errorValue := user.Current()
		if errorValue != nil {
			return nil, errorValue
		}
		return launchdService{HomeDirectory: homeDirectory, UserID: currentUser.Uid, Run: run}, nil
	case "linux":
		return systemdUserService{HomeDirectory: homeDirectory, Run: run}, nil
	}
	return nil, fmt.Errorf("background service install is not supported on %s yet; run `internkim-companion run` from a terminal or a scheduled task", runtime.GOOS)
}

func servicePath(homeDirectory string) string {
	return strings.Join([]string{
		filepath.Join(homeDirectory, ".local", "bin"),
		"/opt/homebrew/bin",
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
	}, ":")
}

func runCommandAttached(name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func runCommandQuietly(name string, arguments ...string) error {
	return exec.Command(name, arguments...).Run()
}
