package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/machost"
	blueclaw "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const (
	macHostDefaultListenAddress  = "127.0.0.1:18080"
	macHostWorkspaceMinimumBytes = 8 * 1024 * 1024 * 1024
)

func runMac() {
	if errorValue := runMacArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runMacArguments(arguments []string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("internkim mac runs the guest under vfkit, which is macOS only; this host is %s", runtime.GOOS)
	}
	subcommand := "status"
	if len(arguments) > 0 && arguments[0] != "" && arguments[0][0] != '-' {
		subcommand = arguments[0]
		arguments = arguments[1:]
	}
	switch subcommand {
	case "install":
		return runMacInstall(arguments)
	case "start":
		return runMacStart(arguments)
	case "stop":
		return runMacStop(arguments)
	case "status":
		return runMacStatus(arguments)
	case "verify":
		return runMacVerify(arguments)
	}
	return fmt.Errorf("unknown mac subcommand: %s", subcommand)
}

func runMacInstall(arguments []string) error {
	flagSet := flag.NewFlagSet("install", flag.ContinueOnError)
	installRootPath := flagSet.String("install-root", "", "where the guest's files live")
	listenAddress := flagSet.String("listen", macHostDefaultListenAddress, "where the host answers for the guest")
	modelName := flagSet.String("model", blueclaw.BlueclawDefaultModelName, "model the agent asks for")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	layout, errorValue := macLayout(*installRootPath)
	if errorValue != nil {
		return errorValue
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}

	fmt.Printf("  installing into %s\n", layout.InstallRootPath)
	if errorValue := machost.Install(machost.InstallRequest{
		Layout:                   layout,
		RepositoryRootPath:       repositoryRootPath,
		ArtifactDirectoryPath:    filepath.Join(repositoryRootPath, blueclaw.BlueclawRuntimeArtifactPath),
		PayloadDirectoryPath:     filepath.Join(repositoryRootPath, blueclaw.BlueclawPayloadArtifactPath),
		HostHTTPListenAddress:    *listenAddress,
		ModelName:                *modelName,
		WorkspaceMinimumBytes:    macHostWorkspaceMinimumBytes,
		ReadCodesignEntitlements: machost.ReadCodesignEntitlements,
	}); errorValue != nil {
		return errorValue
	}
	fmt.Println("  installed")
	return nil
}

func runMacStart(arguments []string) error {
	layout, errorValue := macLayoutFromFlags("start", arguments)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := machost.StartSupervisor(layout); errorValue != nil {
		return errorValue
	}
	fmt.Printf("  supervisor started as process %d\n", machost.RunningSupervisorProcessID(layout))
	return nil
}

func runMacStop(arguments []string) error {
	layout, errorValue := macLayoutFromFlags("stop", arguments)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := machost.StopSupervisor(layout); errorValue != nil {
		return errorValue
	}
	fmt.Println("  supervisor stopped")
	return nil
}

func runMacStatus(arguments []string) error {
	layout, errorValue := macLayoutFromFlags("status", arguments)
	if errorValue != nil {
		return errorValue
	}
	processID := machost.RunningSupervisorProcessID(layout)
	if processID == 0 {
		fmt.Println("  supervisor stopped")
		return nil
	}
	fmt.Printf("  supervisor running as process %d\n", processID)
	return nil
}

func runMacVerify(arguments []string) error {
	flagSet := flag.NewFlagSet("verify", flag.ContinueOnError)
	installRootPath := flagSet.String("install-root", "", "where the guest's files live")
	listenAddress := flagSet.String("listen", macHostDefaultListenAddress, "where the host answers for the guest")
	timeout := flagSet.Duration("timeout", 3*time.Minute, "how long the guest has to answer")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	layout, errorValue := macLayout(*installRootPath)
	if errorValue != nil {
		return errorValue
	}

	document, errorValue := machost.WaitForGuestHealth(*listenAddress, *timeout)
	if errorValue != nil {
		printMacGuestConsoleTail(layout)
		return errorValue
	}
	fmt.Printf("  the guest answers: %s\n", document)
	fmt.Println("  this says the guest booted and serves, not that it can do company work")
	return nil
}

func macLayoutFromFlags(name string, arguments []string) (machost.Layout, error) {
	flagSet := flag.NewFlagSet(name, flag.ContinueOnError)
	installRootPath := flagSet.String("install-root", "", "where the guest's files live")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return machost.Layout{}, errorValue
	}
	return macLayout(*installRootPath)
}

func macLayout(installRootPath string) (machost.Layout, error) {
	if installRootPath == "" {
		homeDirectory, errorValue := os.UserHomeDir()
		if errorValue != nil {
			return machost.Layout{}, errorValue
		}
		installRootPath = filepath.Join(homeDirectory, ".internkim", "blueclaw")
	}
	return machost.NewLayout(installRootPath, macHostRuntimeRootPath()), nil
}

// The runtime directory holds vfkit's sockets, and macOS caps a socket path at 104 bytes,
// so it lives beside /tmp rather than under the install root, which is far too long.
func macHostRuntimeRootPath() string {
	return fmt.Sprintf("/tmp/bc-%d", os.Getuid())
}

// vfkit writes the guest console to a file, and it is the only thing that says why a guest
// that never answered died — the supervisor's own log only sees a process that stopped.
func printMacGuestConsoleTail(layout machost.Layout) {
	consolePaths, _ := filepath.Glob(filepath.Join(layout.LogDirectoryPath(), "*", "console.log"))
	for _, consolePath := range consolePaths {
		document, errorValue := os.ReadFile(consolePath)
		if errorValue != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(document)), "\n")
		if len(lines) > 20 {
			lines = lines[len(lines)-20:]
		}
		fmt.Printf("  %s:\n%s\n", consolePath, strings.Join(lines, "\n"))
	}
}
