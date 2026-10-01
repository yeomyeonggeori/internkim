package devplane

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The local record answers on the port the Supabase CLI always uses, and the
// guest reaches it by the same number over the reverse forward.
const localRecordPort = 54321

const guestAdminPort = 18080

type Options struct {
	RepositoryRootPath string
	ExecutablePath     string
	TestArguments      []string
}

type Logger interface {
	Info(message string)
}

type CommandPlan struct {
	DirectoryPath string
	Name          string
	Arguments     []string
	Environment   []string
}

type Service struct {
	options            Options
	runID              string
	stateRootPath      string
	virtualMachineName string
	adminHostPort      int
	companyAppPort     int
}

func NewService(options Options) (Service, error) {
	if strings.TrimSpace(options.RepositoryRootPath) == "" || strings.TrimSpace(options.ExecutablePath) == "" {
		return Service{}, errors.New("dev plane needs the repository root and the internkim executable")
	}
	adminHostPort, errorValue := availableLoopbackPort()
	if errorValue != nil {
		return Service{}, errorValue
	}
	companyAppPort, errorValue := availableLoopbackPort()
	if errorValue != nil {
		return Service{}, errorValue
	}
	runID := generateRunID()
	return Service{
		options:            options,
		runID:              runID,
		stateRootPath:      filepath.Join(options.RepositoryRootPath, ".local", "dev-plane", "runs", runID),
		virtualMachineName: "internkim-dev-plane-" + runID,
		adminHostPort:      adminHostPort,
		companyAppPort:     companyAppPort,
	}, nil
}

func (service Service) Run(contextValue context.Context, logger Logger) error {
	service.logRunContext(logger)
	service.reapOrphanedGuests(contextValue, logger)
	if errorValue := service.writeLabConfiguration(); errorValue != nil {
		return errorValue
	}
	errorValue := service.runPlans(contextValue, logger, service.planePlans())
	cleanupError := service.cleanUp(contextValue, logger)
	if errorValue != nil {
		if cleanupError != nil {
			return fmt.Errorf("%w; cleanup failed: %v", errorValue, cleanupError)
		}
		return errorValue
	}
	return cleanupError
}

func (service Service) planePlans() []CommandPlan {
	return []CommandPlan{
		service.command(filepath.Join(service.options.RepositoryRootPath, "tools", "start-dev-plane-app"),
			"--state-root", service.stateRootPath,
			"--app-port", strconv.Itoa(service.companyAppPort),
			"--admin-port", strconv.Itoa(service.adminHostPort),
		),
		service.command(filepath.Join(service.options.RepositoryRootPath, "tools", "prepare-company-plane")),
		service.command(filepath.Join(service.options.RepositoryRootPath, "tools", "prepare-container-kernel")),
		service.labCommand("vm-up"),
		service.shellPlan(service.checkSharedWorkspaceCommand()),
		service.shellPlan(service.startTunnelCommand()),
		service.labCommand("vm-ssh", "bash /mnt/shared/workspace/lab/scripts/provision-blueclaw-dev-session.sh admin /mnt/shared 1"),
		service.planeTestPlan(),
	}
}

func (service Service) planeTestPlan() CommandPlan {
	arguments := []string{
		"sudo", "bash", "/mnt/shared/workspace/lab/scripts/scenario-company-plane.sh",
		"/mnt/shared/workspace/.artifacts/dev-plane/" + service.runID,
		strconv.Itoa(service.companyAppPort),
	}
	arguments = append(arguments, service.options.TestArguments...)
	return service.labCommand("vm-ssh", quoteShellArguments(arguments))
}

func (service Service) cleanUp(contextValue context.Context, logger Logger) error {
	cleanupContext, cancel := cleanupContextFrom(contextValue)
	defer cancel()
	var cleanupErrors []string
	for _, plan := range []CommandPlan{
		service.shellPlan(service.stopTunnelCommand()),
		service.shellPlan(service.stopAppCommand()),
		service.shellPlan(service.removeGuestCommand()),
	} {
		if errorValue := service.runPlan(cleanupContext, logger, plan); errorValue != nil {
			cleanupErrors = append(cleanupErrors, errorValue.Error())
		}
	}
	if len(cleanupErrors) > 0 {
		return errors.New(strings.Join(cleanupErrors, "; "))
	}
	return nil
}

func cleanupContextFrom(contextValue context.Context) (context.Context, context.CancelFunc) {
	if contextValue.Err() == nil {
		return context.WithTimeout(contextValue, 5*time.Minute)
	}
	return context.WithTimeout(context.Background(), 5*time.Minute)
}

func (service Service) reapOrphanedGuests(contextValue context.Context, logger Logger) {
	reapContext, cancel := context.WithTimeout(contextValue, time.Minute)
	defer cancel()
	if errorValue := service.runPlan(reapContext, logger, service.shellPlan(service.reapOrphanedGuestsCommand())); errorValue != nil {
		logger.Info("orphaned guest reap skipped: " + errorValue.Error())
	}
}

func (service Service) logRunContext(logger Logger) {
	logger.Info("dev plane run: " + service.runID)
	logger.Info("state: " + service.stateRootPath)
	logger.Info("evidence: " + filepath.Join(service.options.RepositoryRootPath, ".artifacts", "dev-plane", service.runID))
	logger.Info("guest: " + service.virtualMachineName)
	logger.Info("cleanup: " + service.removeGuestCommand() + "; rm -rf " + quoteShell(service.stateRootPath))
}

func (service Service) writeLabConfiguration() error {
	if errorValue := os.MkdirAll(service.stateRootPath, 0o700); errorValue != nil {
		return errorValue
	}
	document, errorValue := json.MarshalIndent(map[string]any{
		"host": map[string]any{"mode": "single-mac"},
		"vm": map[string]any{
			"container": map[string]any{
				"binaryPath": "container",
				"name":       service.virtualMachineName,
				"image":      "ubuntu:24.04",
				"cpuCount":   6,
				"memoryMiB":  8192,
			},
			"sharedWorkspacePath": service.options.RepositoryRootPath,
			"mountDirectoryPath":  "/mnt/shared",
			"sshUsername":         "admin",
			"sshPassword":         "admin",
		},
	}, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(service.labConfigurationPath(), document, 0o600)
}

func (service Service) runPlans(contextValue context.Context, logger Logger, plans []CommandPlan) error {
	for _, plan := range plans {
		if errorValue := service.runPlan(contextValue, logger, plan); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service Service) runPlan(contextValue context.Context, logger Logger, plan CommandPlan) error {
	logger.Info("$ " + strings.Join(append([]string{plan.Name}, plan.Arguments...), " "))
	command := exec.CommandContext(contextValue, plan.Name, plan.Arguments...)
	command.Dir = plan.DirectoryPath
	command.Env = plan.Environment
	outputPipe, errorValue := command.StdoutPipe()
	if errorValue != nil {
		return errorValue
	}
	errorPipe, errorValue := command.StderrPipe()
	if errorValue != nil {
		return errorValue
	}
	if errorValue := command.Start(); errorValue != nil {
		return errorValue
	}
	done := make(chan struct{}, 2)
	go scanPlanOutput(outputPipe, logger, done)
	go scanPlanOutput(errorPipe, logger, done)
	<-done
	<-done
	return command.Wait()
}

func scanPlanOutput(pipe interface{ Read([]byte) (int, error) }, logger Logger, done chan struct{}) {
	defer func() { done <- struct{}{} }()
	reader := bufio.NewReaderSize(pipe, 1024*64)
	for {
		fragment, errorValue := reader.ReadSlice('\n')
		if text := strings.TrimSpace(string(fragment)); text != "" {
			logger.Info(text)
		}
		if errorValue == nil || errors.Is(errorValue, bufio.ErrBufferFull) {
			continue
		}
		return
	}
}

func (service Service) labCommand(subcommand string, arguments ...string) CommandPlan {
	commandArguments := append([]string{"lab", subcommand, "--config", service.labConfigurationPath()}, arguments...)
	return service.command(service.options.ExecutablePath, commandArguments...)
}

func (service Service) command(name string, arguments ...string) CommandPlan {
	return CommandPlan{
		DirectoryPath: service.options.RepositoryRootPath,
		Name:          name,
		Arguments:     arguments,
		Environment:   os.Environ(),
	}
}

func (service Service) shellPlan(command string) CommandPlan {
	return service.command("/bin/sh", "-c", command)
}

func (service Service) labInvocation(subcommand string) string {
	return quoteShell(service.options.ExecutablePath) + " lab " + subcommand + " --config " + quoteShell(service.labConfigurationPath())
}

func (service Service) checkSharedWorkspaceCommand() string {
	command := service.labInvocation("vm-ssh") + " " + quoteShell("test -d /mnt/shared/workspace")
	return "for attempt in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do " + command + " && exit 0; sleep 2; done; " + command
}

func (service Service) startTunnelCommand() string {
	pidPath := quoteShell(service.tunnelPIDPath())
	logPath := quoteShell(filepath.Join(service.stateRootPath, "tunnel.log"))
	adminForward := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", service.adminHostPort, guestAdminPort)
	forwardHealthCheck := fmt.Sprintf("nc -z 127.0.0.1 %d", service.adminHostPort)
	appReverseForward := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", service.companyAppPort, service.companyAppPort)
	recordReverseForward := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", localRecordPort, localRecordPort)
	return strings.Join([]string{
		"host=$(" + service.labInvocation("vm-ip") + ")",
		"test -n \"$host\"",
		"for attempt in 1 2 3 4 5 6 7 8 9 10; do rm -f " + pidPath + "; (nohup sshpass -p admin ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password -o PubkeyAuthentication=no -o ExitOnForwardFailure=yes -N -L " + quoteShell(adminForward) + " -R " + quoteShell(appReverseForward) + " -R " + quoteShell(recordReverseForward) + " admin@\"$host\" > " + logPath + " 2>&1 < /dev/null & echo $! > " + pidPath + "); sleep 1; if [ -s " + pidPath + " ] && kill -0 \"$(cat " + pidPath + ")\" 2>/dev/null && " + forwardHealthCheck + "; then exit 0; fi; sleep 2; done; cat " + logPath + " 2>/dev/null || true; exit 1",
	}, " && ")
}

func (service Service) stopTunnelCommand() string {
	return stopProcessCommand(service.tunnelPIDPath())
}

func (service Service) stopAppCommand() string {
	return stopProcessCommand(filepath.Join(service.stateRootPath, "app.pid"))
}

func stopProcessCommand(pidPath string) string {
	quotedPath := quoteShell(pidPath)
	return "if [ -s " + quotedPath + " ]; then kill \"$(cat " + quotedPath + ")\" 2>/dev/null || true; fi; rm -f " + quotedPath
}

func (service Service) reapOrphanedGuestsCommand() string {
	listOrphans := `container ls -a 2>/dev/null | awk '$1 ~ /^internkim-dev-plane-/ && $5 == "stopped" { print $1 }'`
	return "for orphan in $(" + listOrphans + "); do if [ \"$orphan\" != " + quoteShell(service.virtualMachineName) + " ]; then container rm \"$orphan\" >/dev/null 2>&1 || true; fi; done"
}

func (service Service) removeGuestCommand() string {
	name := quoteShell(service.virtualMachineName)
	return "container stop " + name + " >/dev/null 2>&1 || true; container rm " + name + " >/dev/null 2>&1 || true"
}

func (service Service) labConfigurationPath() string {
	return filepath.Join(service.stateRootPath, "config.json")
}

func (service Service) tunnelPIDPath() string {
	return filepath.Join(service.stateRootPath, "tunnel.pid")
}

func availableLoopbackPort() (int, error) {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		return 0, errorValue
	}
	defer listener.Close()
	networkAddress, isTCP := listener.Addr().(*net.TCPAddr)
	if !isTCP {
		return 0, errors.New("loopback listener did not return a TCP address")
	}
	return networkAddress.Port, nil
}

func generateRunID() string {
	randomBytes := make([]byte, 4)
	if _, errorValue := rand.Read(randomBytes); errorValue != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return strings.ToLower(time.Now().UTC().Format("20060102t150405")) + "-" + hex.EncodeToString(randomBytes)
}

func quoteShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func quoteShellArguments(arguments []string) string {
	quotedArguments := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		quotedArguments = append(quotedArguments, quoteShell(argument))
	}
	return strings.Join(quotedArguments, " ")
}
