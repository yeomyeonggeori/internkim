package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/blueclawworkspace"
	"gitlab.com/eastriver/internkim/internal/deployops"
	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const pocContainerLinuxGoCachePath = "/tmp/internkim-go-cache-linux-arm64"
const pocContainerRemoteEnvironmentPrefix = "export PATH=\"/opt/homebrew/bin:$PATH\"; "
const pocContainerCapabilityContractFilename = "capability-contract.json"

var pocContainerCapabilityContractComponents = []string{"admind", "capabilityd", "blueclaw", "skills"}

func deployPocContainer(target deployops.Target, components []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	loadEnvironmentFile(filepath.Join(repositoryRootPath, ".env"))
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
	remoteContractPath, errorValue := stagePocContainerCapabilityContract(target, temporaryDirectoryPath)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := runRemote(target, pocContainerCapabilityContractCheckCommand(remoteContractPath, components)); errorValue != nil {
		return errorValue
	}
	for _, component := range components {
		if errorValue := deployPocContainerComponent(target, repositoryRootPath, temporaryDirectoryPath, component); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := syncPocContainerRuntimeScripts(target, repositoryRootPath); errorValue != nil {
		return errorValue
	}
	if errorValue := installPocContainerCapabilityContract(target, remoteContractPath); errorValue != nil {
		return errorValue
	}
	return recreatePocContainer(target)
}

func stagePocContainerCapabilityContract(target deployops.Target, temporaryDirectoryPath string) (string, error) {
	document, errorValue := blueclawruntime.CapabilityContractDocument()
	if errorValue != nil {
		return "", errorValue
	}
	localPath := filepath.Join(temporaryDirectoryPath, pocContainerCapabilityContractFilename)
	if errorValue := os.WriteFile(localPath, []byte(document), 0o600); errorValue != nil {
		return "", errorValue
	}
	remotePath := path.Join("/tmp", pocContainerCapabilityContractFilename)
	if errorValue := scpToTarget(target, localPath, remotePath); errorValue != nil {
		return "", errorValue
	}
	return remotePath, nil
}

func installPocContainerCapabilityContract(target deployops.Target, remoteContractPath string) error {
	command := "install -m 600 " + quoteShellValue(remoteContractPath) + " " + quoteShellValue(pocContainerCapabilityContractFilename) +
		" && rm -f " + quoteShellValue(remoteContractPath)
	return runRemote(target, command)
}

func pocContainerCapabilityContractComponentsIncluded(components []string) bool {
	selectedComponents := map[string]bool{}
	for _, component := range components {
		selectedComponents[component] = true
	}
	for _, component := range pocContainerCapabilityContractComponents {
		if !selectedComponents[component] {
			return false
		}
	}
	return true
}

func pocContainerCapabilityContractCheckCommand(remoteContractPath string, components []string) string {
	if pocContainerCapabilityContractComponentsIncluded(components) {
		return "true"
	}
	requiredComponents := strings.Join(pocContainerCapabilityContractComponents, ",")
	errorMessage := "capability contract changed; deploy coupled components: " + requiredComponents
	return "if [ ! -f " + quoteShellValue(pocContainerCapabilityContractFilename) + "] || " +
		"! cmp -s " + quoteShellValue(remoteContractPath) + " " + quoteShellValue(pocContainerCapabilityContractFilename) + "; then " +
		"rm -f " + quoteShellValue(remoteContractPath) + "; echo " + quoteShellValue(errorMessage) + " >&2; exit 1; fi"
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
	case "mattermostPlugins":
		return syncPocContainerMattermostPlugins(target, repositoryRootPath, temporaryDirectoryPath)
	case "skills":
		return syncPocContainerSkills(target, repositoryRootPath, temporaryDirectoryPath)
	default:
		return fmt.Errorf("poc-container deploy does not support component %q", component)
	}
}

func syncPocContainerSkills(target deployops.Target, repositoryRootPath string, temporaryDirectoryPath string) error {
	archivePath := filepath.Join(temporaryDirectoryPath, "skills.tar")
	if errorValue := runPocCommand(repositoryRootPath, nil, "tar", "-C", blueclawworkspace.AssetsPath(repositoryRootPath), "-cf", archivePath, "skills"); errorValue != nil {
		return errorValue
	}
	remoteArchivePath := path.Join("/tmp", filepath.Base(archivePath))
	if errorValue := scpToTarget(target, archivePath, remoteArchivePath); errorValue != nil {
		return errorValue
	}
	return runRemote(target, fmt.Sprintf(
		"for tenantWorkspace in workspace/tenant_*; do rm -rf \"$tenantWorkspace/skills\" && tar -C \"$tenantWorkspace\" -xf %s; done && rm -f %s",
		quoteShellValue(remoteArchivePath),
		quoteShellValue(remoteArchivePath),
	))
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

func syncPocContainerMattermostPlugins(target deployops.Target, repositoryRootPath string, temporaryDirectoryPath string) error {
	if errorValue := runPocCommand(repositoryRootPath, nil, "make", "build-mattermost-ephemeral-plugin"); errorValue != nil {
		return errorValue
	}
	archivePath := filepath.Join(temporaryDirectoryPath, "mattermost-plugins.tar")
	if errorValue := runPocCommand(repositoryRootPath, nil, "tar", "-C", filepath.Join(repositoryRootPath, "build"), "-cf", archivePath, "mattermost-plugins"); errorValue != nil {
		return errorValue
	}
	remoteArchivePath := path.Join("/tmp", filepath.Base(archivePath))
	if errorValue := scpToTarget(target, archivePath, remoteArchivePath); errorValue != nil {
		return errorValue
	}
	return runRemote(target, fmt.Sprintf(
		"rm -rf tenant/mattermost-plugins && tar -C tenant -xf %s && rm -f %s",
		quoteShellValue(remoteArchivePath),
		quoteShellValue(remoteArchivePath),
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

func syncPocContainerRuntimeScripts(target deployops.Target, repositoryRootPath string) error {
	pathsToSync := map[string]string{
		filepath.Join("poc", "start-poc.py"):                   "start-poc.py",
		filepath.Join("poc", "restart-tunnel.py"):              "restart-tunnel.py",
		filepath.Join("poc", "refresh_capability_contract.py"): "refresh_capability_contract.py",
		filepath.Join("poc", "tenant", "Dockerfile"):           filepath.Join("tenant", "Dockerfile"),
		filepath.Join("poc", "tenant", "entrypoint.sh"):        filepath.Join("tenant", "entrypoint.sh"),
	}
	for localRelativePath, remoteRelativePath := range pathsToSync {
		localPath := filepath.Join(repositoryRootPath, localRelativePath)
		remotePath := path.Join(target.Workdir, remoteRelativePath)
		if errorValue := scpToTarget(target, localPath, remotePath); errorValue != nil {
			return errorValue
		}
	}
	return runRemote(target, "chmod +x start-poc.py restart-tunnel.py refresh_capability_contract.py tenant/entrypoint.sh")
}

func recreatePocContainer(target deployops.Target) error {
	return runRemote(target, pocContainerRecreateCommand(target))
}

func pocContainerRecreateCommand(target deployops.Target) string {
	overlayPath := "tenant/Dockerfile.deploy-overlay"
	return strings.Join([]string{
		"set -eu",
		"trap " + quoteShellValue("rm -f "+overlayPath) + " EXIT",
		"chmod +x tenant/bin/* start-poc.py restart-tunnel.py refresh_capability_contract.py",
		"rm -f " + quoteShellValue(overlayPath),
		pocContainerTagExistingImageCommand(target),
		pocContainerBuildImageCommand(target, overlayPath),
		pocContainerTenantCountCommand(),
		"TENANT_IMAGE=" + quoteShellValue(target.ImageTag) + " python3 start-poc.py \"$tenant_count\"",
		"[ ! -f cf.env ] || python3 restart-tunnel.py",
	}, "\n")
}

func pocContainerTenantCountCommand() string {
	return "tenant_count=\"$(find config -type d -name 'tenant_[0-9][0-9]*' -prune 2>/dev/null | sed 's#.*/tenant_0*##' | sort -n | tail -1)\"\n" +
		"if [ -z \"$tenant_count\" ]; then tenant_count=10; fi"
}

func pocContainerTagExistingImageCommand(target deployops.Target) string {
	return "if container image inspect " + quoteShellValue(target.ImageTag) + " >/dev/null 2>&1; then " +
		"container image tag " + quoteShellValue(target.ImageTag) + " " + quoteShellValue(pocContainerBaseImageTag(target.ImageTag)) + "; " +
		"fi"
}

func pocContainerBuildImageCommand(target deployops.Target, overlayPath string) string {
	imageTag := quoteShellValue(target.ImageTag)
	baseImageTag := quoteShellValue(pocContainerBaseImageTag(target.ImageTag))
	overlayDocument := pocContainerOverlayDockerfile(pocContainerBaseImageTag(target.ImageTag))
	return "if ! container build --platform linux/arm64 -t " + imageTag + " ./tenant; then\n" +
		"container image inspect " + baseImageTag + " >/dev/null 2>&1\n" +
		"cat > " + quoteShellValue(overlayPath) + " <<'INTERNKIM_OVERLAY_EOF'\n" +
		overlayDocument +
		"INTERNKIM_OVERLAY_EOF\n" +
		"container build --platform linux/arm64 -f " + quoteShellValue(overlayPath) + " -t " + imageTag + " ./tenant\n" +
		"fi"
}

func pocContainerOverlayDockerfile(baseImageTag string) string {
	return "FROM " + baseImageTag + "\n" +
		"COPY --chmod=0755 bin/internkim-capabilityd /usr/local/bin/internkim-capabilityd\n" +
		"COPY --chmod=0755 bin/blueclaw /usr/local/bin/blueclaw\n" +
		"COPY --chmod=4755 bin/blueclaw-posix-helper /usr/local/bin/blueclaw-posix-helper\n" +
		"COPY --chmod=0755 bin/internkim-admind /usr/local/bin/internkim-admind\n" +
		"COPY migrations /opt/blueclaw/migrations\n" +
		"COPY board-ui /opt/internkim/board-ui\n" +
		"COPY mattermost-plugins /opt/internkim/mattermost-plugins\n" +
		"COPY --chmod=0755 entrypoint.sh /usr/local/bin/entrypoint.sh\n"
}

func pocContainerBaseImageTag(imageTag string) string {
	imageTag = strings.TrimSpace(imageTag)
	tagSeparatorIndex := strings.LastIndex(imageTag, ":")
	slashIndex := strings.LastIndex(imageTag, "/")
	if tagSeparatorIndex > slashIndex {
		return imageTag[:tagSeparatorIndex+1] + "base-before-deploy"
	}
	return imageTag + ":base-before-deploy"
}

func scpToTarget(target deployops.Target, localPath string, remotePath string) error {
	destination := pocContainerSSHDestination(target) + ":" + remotePath
	arguments := append(pocContainerSSHBaseArguments(target), localPath, destination)
	return runPocCommand("", pocContainerSSHEnvironment(), "scp", arguments...)
}

func runRemote(target deployops.Target, remoteCommand string) error {
	command := pocContainerRemoteEnvironmentPrefix + "cd " + quoteShellValue(target.Workdir) + " && " + remoteCommand
	arguments := append(pocContainerSSHBaseArguments(target), pocContainerSSHDestination(target), command)
	return runPocCommand("", pocContainerSSHEnvironment(), "ssh", arguments...)
}

func pocContainerSSHBaseArguments(target deployops.Target) []string {
	arguments := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "PreferredAuthentications=password",
		"-o", "PubkeyAuthentication=no",
		"-o", "IdentitiesOnly=yes",
		"-o", "ConnectTimeout=20",
	}
	if target.SSHProxyCommand != "" {
		arguments = append(arguments, "-o", "ProxyCommand="+target.SSHProxyCommand)
	}
	return arguments
}

func pocContainerSSHEnvironment() []string {
	overrides := map[string]string{
		"DISPLAY":             "internkim-poc-ssh",
		"SSH_ASKPASS":         filepath.Join(pocContainerRepositoryRootPath(), "tools", "poc-ssh-askpass"),
		"SSH_ASKPASS_REQUIRE": "force",
	}
	environment := []string{}
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		if _, isOverridden := overrides[key]; !isOverridden {
			environment = append(environment, entry)
		}
	}
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}
	return environment
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
		return []string{"admind", "capabilityd", "blueclaw", "web", "mattermostPlugins", "skills"}, nil
	}
	components, errorValue := normalizePocContainerComponents(strings.Split(value, ","))
	if errorValue != nil {
		return nil, errorValue
	}
	return pocContainerCoupledComponents(components), nil
}

func pocContainerCoupledComponents(components []string) []string {
	hasBlueclaw := false
	hasSkills := false
	hasWeb := false
	hasMattermostPlugins := false
	for _, component := range components {
		hasBlueclaw = hasBlueclaw || component == "blueclaw"
		hasSkills = hasSkills || component == "skills"
		hasWeb = hasWeb || component == "web"
		hasMattermostPlugins = hasMattermostPlugins || component == "mattermostPlugins"
	}
	if hasBlueclaw && !hasSkills {
		components = append(components, "skills")
	}
	if hasWeb && !hasMattermostPlugins {
		components = append(components, "mattermostPlugins")
	}
	return components
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
	case "admind", "capabilityd", "blueclaw", "web", "mattermostPlugins", "skills":
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

func loadEnvironmentFile(filePath string) {
	document, errorValue := os.ReadFile(filePath)
	if errorValue != nil {
		return
	}
	for _, line := range strings.Split(string(document), "\n") {
		key, value, found := parseEnvironmentLine(line)
		if found && os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func parseEnvironmentLine(line string) (string, string, bool) {
	trimmedLine := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
	if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
		return "", "", false
	}
	separatorIndex := strings.Index(trimmedLine, "=")
	if separatorIndex <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(trimmedLine[:separatorIndex])
	value := strings.Trim(strings.TrimSpace(trimmedLine[separatorIndex+1:]), "\"'")
	return key, value, true
}
