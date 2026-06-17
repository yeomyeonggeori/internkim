package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/deployops"
)

const pocContainerLinuxGoCachePath = "/tmp/internkim-go-cache-linux-arm64"
const pocContainerDarwinGoModuleCachePath = "/tmp/internkim-go-mod-cache-darwin-arm64"

func deployPocContainer(target deployops.Target, components []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	if errorValue := validatePocContainerTarget(target, components); errorValue != nil {
		return errorValue
	}
	temporaryDirectoryPath, errorValue := os.MkdirTemp("", "internkim-poc-container-*")
	if errorValue != nil {
		return errorValue
	}
	defer os.RemoveAll(temporaryDirectoryPath)
	if errorValue := runRemote(target, "mkdir -p tenant/bin"); errorValue != nil {
		return errorValue
	}
	for _, component := range components {
		if errorValue := deployPocContainerComponent(target, repositoryRootPath, temporaryDirectoryPath, component); errorValue != nil {
			return errorValue
		}
	}
	return recreatePocContainer(target)
}

func validatePocContainerTarget(target deployops.Target, components []string) error {
	if strings.TrimSpace(os.Getenv("INTERNKIM_POC_SSH_PASSWORD")) == "" {
		return errors.New("INTERNKIM_POC_SSH_PASSWORD is required for poc-container deploy")
	}
	if strings.TrimSpace(target.SSHHost) == "" {
		return fmt.Errorf("poc-container target %s is missing sshHost", target.ID)
	}
	if strings.TrimSpace(target.SSHUser) == "" {
		return fmt.Errorf("poc-container target %s is missing sshUser", target.ID)
	}
	if strings.TrimSpace(target.Workdir) == "" {
		return fmt.Errorf("poc-container target %s is missing workdir", target.ID)
	}
	if strings.TrimSpace(target.ImageTag) == "" {
		return fmt.Errorf("poc-container target %s is missing imageTag", target.ID)
	}
	if len(components) == 0 {
		return errors.New("poc-container deploy requires at least one component")
	}
	return nil
}

func deployPocContainerComponent(target deployops.Target, repositoryRootPath string, temporaryDirectoryPath string, component string) error {
	switch component {
	case "admind":
		return syncPocContainerBinary(target, repositoryRootPath, temporaryDirectoryPath, "internkim-admind", "./cmd/internkim-admind", false)
	case "capabilityd":
		return syncPocContainerBinary(target, repositoryRootPath, temporaryDirectoryPath, "internkim-capabilityd", "./cmd/internkim-capabilityd", false)
	case "blueclaw":
		return syncPocContainerBlueclaw(target, repositoryRootPath, temporaryDirectoryPath)
	case "web":
		return syncBoardUI(target, repositoryRootPath, temporaryDirectoryPath)
	default:
		return fmt.Errorf("poc-container deploy does not support component %q", component)
	}
}

func syncPocContainerBinary(target deployops.Target, repositoryRootPath string, temporaryDirectoryPath string, binaryName string, packagePath string, isStatic bool) error {
	outputPath := filepath.Join(temporaryDirectoryPath, binaryName)
	if errorValue := buildLinuxBinary(repositoryRootPath, packagePath, outputPath, isStatic); errorValue != nil {
		return errorValue
	}
	return scpToTarget(target, outputPath, path.Join(target.Workdir, "tenant", "bin", binaryName))
}

func syncPocContainerBlueclaw(target deployops.Target, repositoryRootPath string, temporaryDirectoryPath string) error {
	blueclawRootPath := filepath.Join(repositoryRootPath, ".dependency", "blueclaw")
	if errorValue := syncPocContainerBinary(target, blueclawRootPath, temporaryDirectoryPath, "blueclaw", "./cmd/blueclaw", true); errorValue != nil {
		return errorValue
	}
	if errorValue := syncPocContainerBinary(target, blueclawRootPath, temporaryDirectoryPath, "blueclaw-posix-helper", "./cmd/blueclaw-posix-helper", true); errorValue != nil {
		return errorValue
	}
	return syncMigrations(target, repositoryRootPath, temporaryDirectoryPath)
}

func buildLinuxBinary(directoryPath string, packagePath string, outputPath string, isStatic bool) error {
	environment := linuxBuildEnvironment(isStatic)
	return runPocCommand(directoryPath, environment, "go", "build", "-o", outputPath, packagePath)
}

func linuxBuildEnvironment(isStatic bool) []string {
	environment := append([]string{}, os.Environ()...)
	environment = append(environment, "GOOS=linux", "GOARCH=arm64")
	environment = append(environment, "GOCACHE="+pocContainerLinuxGoCachePath)
	environment = append(environment, "GOMODCACHE="+pocContainerDarwinGoModuleCachePath)
	if isStatic {
		environment = append(environment, "CGO_ENABLED=0")
	}
	return environment
}

func syncBoardUI(target deployops.Target, repositoryRootPath string, temporaryDirectoryPath string) error {
	if errorValue := runPocCommand(filepath.Join(repositoryRootPath, "web"), nil, "bun", "run", "build:board"); errorValue != nil {
		return errorValue
	}
	archivePath := filepath.Join(temporaryDirectoryPath, "board-ui.tar")
	if errorValue := runPocCommand(repositoryRootPath, nil, "tar", "-C", filepath.Join(repositoryRootPath, "build"), "-cf", archivePath, "board-ui"); errorValue != nil {
		return errorValue
	}
	remoteArchivePath := path.Join("/tmp", filepath.Base(archivePath))
	if errorValue := scpToTarget(target, archivePath, remoteArchivePath); errorValue != nil {
		return errorValue
	}
	return runRemote(target, fmt.Sprintf(
		"rm -rf tenant/board-ui && tar -C tenant -xf %s && rm -f %s && ([ ! -f %s ] || install -m644 %s tenant/board-ui/logo.png)",
		quoteShellValue(remoteArchivePath),
		quoteShellValue(remoteArchivePath),
		quoteShellValue(path.Join(target.Workdir, "internkim-logo.png")),
		quoteShellValue(path.Join(target.Workdir, "internkim-logo.png")),
	))
}

func syncMigrations(target deployops.Target, repositoryRootPath string, temporaryDirectoryPath string) error {
	archivePath := filepath.Join(temporaryDirectoryPath, "migrations.tar")
	blueclawRootPath := filepath.Join(repositoryRootPath, ".dependency", "blueclaw")
	if errorValue := runPocCommand(repositoryRootPath, nil, "tar", "-C", blueclawRootPath, "-cf", archivePath, "migrations"); errorValue != nil {
		return errorValue
	}
	remoteArchivePath := path.Join("/tmp", filepath.Base(archivePath))
	if errorValue := scpToTarget(target, archivePath, remoteArchivePath); errorValue != nil {
		return errorValue
	}
	return runRemote(target, fmt.Sprintf(
		"rm -rf tenant/migrations && tar -C tenant -xf %s && rm -f %s",
		quoteShellValue(remoteArchivePath),
		quoteShellValue(remoteArchivePath),
	))
}

func recreatePocContainer(target deployops.Target) error {
	return runRemote(target, fmt.Sprintf(
		"chmod +x tenant/bin/* && docker build -q -t %s ./tenant && docker compose -f %s up -d --force-recreate",
		quoteShellValue(target.ImageTag),
		quoteShellValue(pocContainerComposeFile(target)),
	))
}

func pocContainerComposeFile(target deployops.Target) string {
	composeFile := strings.TrimSpace(target.ComposeFile)
	if composeFile == "" {
		return "tenants.generated.yml"
	}
	return composeFile
}

func scpToTarget(target deployops.Target, localPath string, remotePath string) error {
	destination := pocContainerSSHDestination(target) + ":" + remotePath
	arguments := append(pocContainerSSHBaseArguments("scp"), localPath, destination)
	return runPocCommand("", nil, filepath.Join(pocContainerRepositoryRootPath(), "bin", "sshpass"), arguments...)
}

func runRemote(target deployops.Target, remoteCommand string) error {
	command := "cd " + quoteShellValue(target.Workdir) + " && " + remoteCommand
	arguments := append(pocContainerSSHBaseArguments("ssh"), pocContainerSSHDestination(target), command)
	return runPocCommand("", nil, filepath.Join(pocContainerRepositoryRootPath(), "bin", "sshpass"), arguments...)
}

func pocContainerSSHBaseArguments(commandName string) []string {
	return []string{
		"-p", os.Getenv("INTERNKIM_POC_SSH_PASSWORD"),
		commandName,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "PreferredAuthentications=password",
		"-o", "PubkeyAuthentication=no",
		"-o", "IdentitiesOnly=yes",
	}
}

func pocContainerSSHDestination(target deployops.Target) string {
	return target.SSHUser + "@" + target.SSHHost
}

func runPocCommand(directoryPath string, environment []string, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	if directoryPath != "" {
		command.Dir = directoryPath
	}
	if environment != nil {
		command.Env = environment
	}
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Run(); errorValue != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(redactPocCommandArguments(arguments), " "), errorValue)
	}
	return nil
}

func redactPocCommandArguments(arguments []string) []string {
	redactedArguments := append([]string{}, arguments...)
	for index, argument := range redactedArguments {
		if argument == "-p" && index+1 < len(redactedArguments) {
			redactedArguments[index+1] = "<redacted>"
		}
	}
	return redactedArguments
}

func selectedPocContainerComponents(arguments []string) ([]string, error) {
	value := strings.TrimSpace(commandArgumentValue(arguments, "--components", ""))
	if value == "" {
		return []string{"admind", "capabilityd", "blueclaw", "web"}, nil
	}
	return normalizePocContainerComponents(strings.Split(value, ","))
}

func normalizePocContainerComponents(componentNames []string) ([]string, error) {
	components := []string{}
	seenComponents := map[string]bool{}
	for _, componentName := range componentNames {
		component, errorValue := normalizePocContainerComponent(componentName)
		if errorValue != nil {
			return nil, errorValue
		}
		if component != "" && !seenComponents[component] {
			seenComponents[component] = true
			components = append(components, component)
		}
	}
	if len(components) == 0 {
		return nil, errors.New("--components did not name any poc-container components")
	}
	return components, nil
}

func normalizePocContainerComponent(componentName string) (string, error) {
	switch strings.TrimSpace(componentName) {
	case "":
		return "", nil
	case "admind", "capabilityd", "blueclaw", "web":
		return strings.TrimSpace(componentName), nil
	case "adminWeb", "admin-web":
		return "web", nil
	case "blueclawPayload":
		return "blueclaw", nil
	default:
		return "", fmt.Errorf("poc-container deploy does not support component %q", componentName)
	}
}

func pocContainerRepositoryRootPath() string {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return "."
	}
	return repositoryRootPath
}
