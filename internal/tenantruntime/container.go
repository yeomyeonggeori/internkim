package tenantruntime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/botassets"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const (
	DefaultContainerWorkDirectoryPath = "~/.internkim/container-poc"
	DefaultContainerTenantImageName   = "internkim-poc-tenant:latest"
	DefaultContainerTenantModelName   = blueclaw.BlueclawDefaultModelName
	DefaultContainerMattermostURL     = "https://poc-0.intern.kim"
	DefaultContainerFlowHostBaseLabel = "poc0"
	DefaultContainerCloudflareZone    = "intern.kim"
	DefaultContainerTunnelName        = "internkim-poc-0"
	containerMattermostNameDisplay    = "nickname_full_name"
	containerMattermostDefaultLocale  = "ko"
)

type ContainerCommandInvocation struct {
	ExecutableName string
	Arguments      []string
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
	Environment    []string
}

type ContainerCommandExecutor interface {
	LookPath(name string) (string, error)
	CombinedOutput(invocation ContainerCommandInvocation) ([]byte, error)
	Run(invocation ContainerCommandInvocation) error
}

type OperatingSystemContainerCommandExecutor struct{}

func (executor OperatingSystemContainerCommandExecutor) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (executor OperatingSystemContainerCommandExecutor) CombinedOutput(invocation ContainerCommandInvocation) ([]byte, error) {
	command := exec.Command(invocation.ExecutableName, invocation.Arguments...)
	command.Env = containerCommandEnvironment(invocation.Environment)
	command.Stdin = invocation.Stdin
	return command.CombinedOutput()
}

func (executor OperatingSystemContainerCommandExecutor) Run(invocation ContainerCommandInvocation) error {
	command := exec.Command(invocation.ExecutableName, invocation.Arguments...)
	command.Env = containerCommandEnvironment(invocation.Environment)
	command.Stdin = invocation.Stdin
	command.Stdout = invocation.Stdout
	command.Stderr = invocation.Stderr
	return command.Run()
}

type ContainerRuntime struct {
	CommandExecutor ContainerCommandExecutor
	Output          io.Writer
	ErrorOutput     io.Writer
}

type ContainerTenant struct {
	Index         int
	TenantID      string
	RuntimeID     string
	TeamName      string
	DatabaseName  string
	AgentUsername string
	ContainerName string
	DisplayName   string
}

type ContainerInfraUpOptions struct {
	WorkDirectoryPath   string
	TenantCount         int
	OpenRouterKeyPath   string
	MattermostPublicURL string
}

type ContainerInfraDownOptions struct {
	WorkDirectoryPath string
	Purge             bool
}

type ContainerTenantAddOptions struct {
	WorkDirectoryPath     string
	TenantID              string
	ModelName             string
	ImageName             string
	OpenRouterKeyPath     string
	AdminEmail            string
	MattermostPublicURL   string
	MattermostLocalURL    string
	FlowHostBaseLabel     string
	CloudflareEnvironment string
	CloudflareAPIBaseURL  string
	CloudflareAPIToken    string
	CloudflareAccountID   string
	CloudflareZoneID      string
	CloudflareZoneName    string
	CloudflareTunnelName  string
	CloudflareTunnelID    string
}

type ContainerTenantRemoveOptions struct {
	WorkDirectoryPath string
	TenantID          string
	PurgeData         bool
	ImageName         string
	OpenRouterKeyPath string
}

type ContainerResetOptions struct {
	WorkDirectoryPath      string
	Purge                  bool
	ImageName              string
	OpenRouterKeyPath      string
	ConfirmSuperAdmin      bool
	SuperAdminPasswordPath string
	MattermostLocalURL     string
	FlowHostBaseLabel      string
	CloudflareEnvironment  string
	CloudflareAPIBaseURL   string
	CloudflareAPIToken     string
	CloudflareAccountID    string
	CloudflareZoneID       string
	CloudflareZoneName     string
	CloudflareTunnelName   string
	CloudflareTunnelID     string
}

type ContainerUpOptions struct {
	WorkDirectoryPath string
	Count             int
	ModelName         string
	ImageName         string
	OpenRouterKeyPath string
}

type ContainerStatusOptions struct {
	WorkDirectoryPath string
	ImageName         string
	OpenRouterKeyPath string
}

type ContainerInfraStatus struct {
	WorkDirectoryPath string `json:"workdir"`
	InfraComposePath  string `json:"infraComposePath"`
}

type ContainerTenantStatus struct {
	TenantID              string `json:"tenantID"`
	RuntimeID             string `json:"runtimeID"`
	ContainerName         string `json:"containerName"`
	ContainerState        string `json:"containerState"`
	MattermostTeamName    string `json:"mattermostTeamName"`
	MattermostTeamURL     string `json:"mattermostTeamURL"`
	FlowURL               string `json:"flowURL,omitempty"`
	CompanyAdminEmail     string `json:"companyAdminEmail,omitempty"`
	CompanyAdminPassword  string `json:"companyAdminPassword,omitempty"`
	MattermostTeamPresent bool   `json:"mattermostTeamPresent"`
	DatabaseName          string `json:"databaseName"`
	DatabasePresent       bool   `json:"databasePresent"`
	ConfigPath            string `json:"configPath"`
	SecretPath            string `json:"secretPath"`
}

type ContainerRuntimeStatus struct {
	WorkDirectoryPath string                  `json:"workdir"`
	Tenants           []ContainerTenantStatus `json:"tenants"`
}

type ContainerTenantRemovalStatus struct {
	TenantID         string `json:"tenantID"`
	RuntimeID        string `json:"runtimeID"`
	PurgedDatabase   bool   `json:"purgedDatabase"`
	RemovedConfig    bool   `json:"removedConfig"`
	RemovedSecrets   bool   `json:"removedSecrets"`
	UpdatedCompose   bool   `json:"updatedCompose"`
	RemovedCompose   bool   `json:"removedCompose"`
	MattermostTeam   string `json:"mattermostTeam"`
	MattermostAgent  string `json:"mattermostAgent"`
	ContainerService string `json:"containerService"`
	RemovedFlowRoute bool   `json:"removedFlowRoute"`
}

type ContainerResetStatus struct {
	WorkDirectoryPath  string                         `json:"workdir"`
	Purge              bool                           `json:"purge"`
	SuperAdminVerified bool                           `json:"superAdminVerified"`
	RemovedTenants     []ContainerTenantRemovalStatus `json:"removedTenants"`
	InfraDown          bool                           `json:"infraDown"`
	RemovedState       []string                       `json:"removedState,omitempty"`
}

var containerTenantPattern = regexp.MustCompile(`^tenant_?([0-9]{2,})$`)
var containerRuntimeIDPattern = regexp.MustCompile(`^tenant_[0-9]{2,}$`)
var containerDatabasePasswordPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{32,128}$`)

type tenantPostgresDatabase interface {
	ExecutePostgres(ctx context.Context, statement string) error
}

type containerTenantPostgresDatabase struct {
	runtime           ContainerRuntime
	workDirectoryPath string
}

type containerStoredPassword struct {
	Value string
	IsNew bool
}

func (runtime ContainerRuntime) InfraUp(options ContainerInfraUpOptions) (ContainerInfraStatus, error) {
	normalizedOptions, errorValue := normalizeContainerInfraUpOptions(options)
	if errorValue != nil {
		return ContainerInfraStatus{}, errorValue
	}
	if errorValue := ensureContainerOpenRouterKey(normalizedOptions.OpenRouterKeyPath); errorValue != nil {
		return ContainerInfraStatus{}, errorValue
	}
	if errorValue := writeContainerInfraFiles(normalizedOptions.WorkDirectoryPath, normalizedOptions.MattermostPublicURL); errorValue != nil {
		return ContainerInfraStatus{}, errorValue
	}
	invocation := runtime.dockerComposeInvocation(containerInfraComposePath(normalizedOptions.WorkDirectoryPath), []string{"up", "-d", "--wait"})
	invocation.Environment = []string{"TENANT_COUNT=" + strconv.Itoa(normalizedOptions.TenantCount)}
	if errorValue := runtime.executor().Run(invocation); errorValue != nil {
		return ContainerInfraStatus{}, fmt.Errorf("container infra up failed: %w", errorValue)
	}
	if errorValue := runtime.lockMattermostDatabase(normalizedOptions.WorkDirectoryPath); errorValue != nil {
		return ContainerInfraStatus{}, errorValue
	}
	return ContainerInfraStatus{
		WorkDirectoryPath: normalizedOptions.WorkDirectoryPath,
		InfraComposePath:  containerInfraComposePath(normalizedOptions.WorkDirectoryPath),
	}, nil
}

func (runtime ContainerRuntime) InfraDown(options ContainerInfraDownOptions) (ContainerInfraStatus, error) {
	workDirectoryPath, errorValue := ResolveContainerWorkDirectoryPath(options.WorkDirectoryPath)
	if errorValue != nil {
		return ContainerInfraStatus{}, errorValue
	}
	arguments := []string{"down"}
	if options.Purge {
		arguments = append(arguments, "--volumes")
	}
	if errorValue := runtime.executor().Run(runtime.dockerComposeInvocation(containerInfraComposePath(workDirectoryPath), arguments)); errorValue != nil {
		return ContainerInfraStatus{}, fmt.Errorf("container infra down failed: %w", errorValue)
	}
	return ContainerInfraStatus{WorkDirectoryPath: workDirectoryPath, InfraComposePath: containerInfraComposePath(workDirectoryPath)}, nil
}

func (runtime ContainerRuntime) AddTenant(options ContainerTenantAddOptions) (ContainerTenantStatus, error) {
	normalizedOptions, tenant, errorValue := normalizeContainerTenantAddOptions(options)
	if errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	if errorValue := ensureContainerOpenRouterKey(normalizedOptions.OpenRouterKeyPath); errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	if errorValue := writeContainerInfraFiles(normalizedOptions.WorkDirectoryPath, normalizedOptions.MattermostPublicURL); errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	databasePassword, errorValue := ensureTenantDatabasePassword(normalizedOptions.WorkDirectoryPath, tenant)
	if errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	if errorValue := runtime.ensureTenantDatabase(normalizedOptions.WorkDirectoryPath, tenant, databasePassword); errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	operatorAdminPassword, errorValue := ensureContainerMattermostAdminPassword(normalizedOptions.WorkDirectoryPath)
	if errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	companyAdminPassword, errorValue := ensureTenantCompanyAdminPassword(normalizedOptions.WorkDirectoryPath, tenant)
	if errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	token, errorValue := runtime.ensureTenantMattermost(normalizedOptions, tenant, operatorAdminPassword, companyAdminPassword)
	if errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	if errorValue := writeContainerTenantFiles(normalizedOptions, tenant, token, databasePassword, operatorAdminPassword.Value); errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	if errorValue := runtime.renderTenantsCompose(normalizedOptions.WorkDirectoryPath, normalizedOptions.ImageName, normalizedOptions.OpenRouterKeyPath); errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	if errorValue := syncContainerTenantFlowRoute(context.Background(), normalizedOptions, tenant); errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	if errorValue := runtime.executor().Run(runtime.dockerComposeInvocation(containerTenantsComposePath(normalizedOptions.WorkDirectoryPath), []string{"up", "-d", "--force-recreate", tenant.RuntimeID})); errorValue != nil {
		return ContainerTenantStatus{}, fmt.Errorf("tenant container start failed for %s: %w", tenant.RuntimeID, errorValue)
	}
	status, errorValue := runtime.statusForTenant(normalizedOptions.WorkDirectoryPath, tenant)
	if errorValue != nil {
		return ContainerTenantStatus{}, errorValue
	}
	status.FlowURL = "https://" + containerTenantFlowHostname(normalizedOptions, tenant)
	status.CompanyAdminEmail = normalizedOptions.AdminEmail
	status.CompanyAdminPassword = companyAdminPassword.Value
	return status, nil
}

func (runtime ContainerRuntime) RemoveTenant(options ContainerTenantRemoveOptions) (ContainerTenantRemovalStatus, error) {
	normalizedOptions, tenant, errorValue := normalizeContainerTenantRemoveOptions(options)
	if errorValue != nil {
		return ContainerTenantRemovalStatus{}, errorValue
	}
	status := ContainerTenantRemovalStatus{
		TenantID:         tenant.TenantID,
		RuntimeID:        tenant.RuntimeID,
		MattermostTeam:   tenant.TeamName,
		MattermostAgent:  tenant.AgentUsername,
		ContainerService: tenant.RuntimeID,
	}
	_ = runtime.executor().Run(runtime.dockerComposeInvocation(containerTenantsComposePath(normalizedOptions.WorkDirectoryPath), []string{"rm", "--stop", "--force", tenant.RuntimeID}))
	_ = runtime.runMattermostCommand(normalizedOptions.WorkDirectoryPath, []string{"team", "delete", tenant.TeamName, "--confirm"})
	_ = runtime.runMattermostCommand(normalizedOptions.WorkDirectoryPath, []string{"user", "delete", tenant.AgentUsername, "--confirm"})
	if normalizedOptions.PurgeData {
		if errorValue := runtime.dropTenantDatabase(normalizedOptions.WorkDirectoryPath, tenant); errorValue != nil {
			return status, errorValue
		}
		status.PurgedDatabase = true
	}
	status.RemovedConfig = removeContainerPath(containerTenantConfigPath(normalizedOptions.WorkDirectoryPath, tenant))
	status.RemovedSecrets = removeContainerPath(containerTenantSecretPath(normalizedOptions.WorkDirectoryPath, tenant))
	if errorValue := runtime.renderTenantsCompose(normalizedOptions.WorkDirectoryPath, normalizedOptions.ImageName, normalizedOptions.OpenRouterKeyPath); errorValue != nil {
		return status, errorValue
	}
	status.UpdatedCompose = true
	return status, nil
}

func (runtime ContainerRuntime) Reset(options ContainerResetOptions) (ContainerResetStatus, error) {
	normalizedOptions, errorValue := normalizeContainerResetOptions(options)
	if errorValue != nil {
		return ContainerResetStatus{}, errorValue
	}
	if errorValue := verifyContainerResetSuperAdmin(normalizedOptions); errorValue != nil {
		return ContainerResetStatus{}, errorValue
	}
	tenants, errorValue := containerTenantsFromConfig(normalizedOptions.WorkDirectoryPath)
	if errorValue != nil {
		return ContainerResetStatus{}, errorValue
	}
	status := ContainerResetStatus{WorkDirectoryPath: normalizedOptions.WorkDirectoryPath, Purge: normalizedOptions.Purge, SuperAdminVerified: true}
	for _, tenant := range tenants {
		flowRemoved, routeError := removeContainerTenantFlowRoute(context.Background(), normalizedOptions, tenant)
		if routeError != nil {
			return status, routeError
		}
		removalStatus, errorValue := runtime.RemoveTenant(ContainerTenantRemoveOptions{
			WorkDirectoryPath: normalizedOptions.WorkDirectoryPath,
			TenantID:          tenant.TenantID,
			PurgeData:         true,
			ImageName:         normalizedOptions.ImageName,
			OpenRouterKeyPath: normalizedOptions.OpenRouterKeyPath,
		})
		if errorValue != nil {
			return status, errorValue
		}
		removalStatus.RemovedFlowRoute = flowRemoved
		status.RemovedTenants = append(status.RemovedTenants, removalStatus)
	}
	if normalizedOptions.Purge {
		if _, errorValue := runtime.InfraDown(ContainerInfraDownOptions{WorkDirectoryPath: normalizedOptions.WorkDirectoryPath, Purge: true}); errorValue != nil {
			return status, errorValue
		}
		status.InfraDown = true
		status.RemovedState = removeContainerGeneratedState(normalizedOptions.WorkDirectoryPath)
	}
	return status, nil
}

func (runtime ContainerRuntime) Up(options ContainerUpOptions) ([]ContainerTenantStatus, error) {
	normalizedOptions, errorValue := normalizeContainerUpOptions(options)
	if errorValue != nil {
		return nil, errorValue
	}
	if _, errorValue := runtime.InfraUp(ContainerInfraUpOptions{
		WorkDirectoryPath: normalizedOptions.WorkDirectoryPath,
		TenantCount:       normalizedOptions.Count,
		OpenRouterKeyPath: normalizedOptions.OpenRouterKeyPath,
	}); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := runtime.BuildTenantImageIfMissing(normalizedOptions.WorkDirectoryPath, normalizedOptions.ImageName); errorValue != nil {
		return nil, errorValue
	}
	statuses := make([]ContainerTenantStatus, 0, normalizedOptions.Count)
	for index := 1; index <= normalizedOptions.Count; index++ {
		tenant, errorValue := ContainerTenantForIndex(index)
		if errorValue != nil {
			return nil, errorValue
		}
		status, errorValue := runtime.AddTenant(ContainerTenantAddOptions{
			WorkDirectoryPath: normalizedOptions.WorkDirectoryPath,
			TenantID:          tenant.TenantID,
			ModelName:         normalizedOptions.ModelName,
			ImageName:         normalizedOptions.ImageName,
			OpenRouterKeyPath: normalizedOptions.OpenRouterKeyPath,
		})
		if errorValue != nil {
			return statuses, errorValue
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (runtime ContainerRuntime) Status(options ContainerStatusOptions) (ContainerRuntimeStatus, error) {
	workDirectoryPath, errorValue := ResolveContainerWorkDirectoryPath(options.WorkDirectoryPath)
	if errorValue != nil {
		return ContainerRuntimeStatus{}, errorValue
	}
	tenants, errorValue := containerTenantsFromConfig(workDirectoryPath)
	if errorValue != nil {
		return ContainerRuntimeStatus{}, errorValue
	}
	status := ContainerRuntimeStatus{WorkDirectoryPath: workDirectoryPath, Tenants: make([]ContainerTenantStatus, 0, len(tenants))}
	for _, tenant := range tenants {
		tenantStatus, errorValue := runtime.statusForTenant(workDirectoryPath, tenant)
		if errorValue != nil {
			return status, errorValue
		}
		status.Tenants = append(status.Tenants, tenantStatus)
	}
	return status, nil
}

func (runtime ContainerRuntime) BuildTenantImageIfMissing(workDirectoryPath string, imageName string) error {
	normalizedWorkDirectoryPath, errorValue := ResolveContainerWorkDirectoryPath(workDirectoryPath)
	if errorValue != nil {
		return errorValue
	}
	normalizedImageName := defaultContainerTenantImageName(imageName)
	if _, errorValue := runtime.executor().CombinedOutput(ContainerCommandInvocation{
		ExecutableName: "docker",
		Arguments:      []string{"image", "inspect", normalizedImageName},
	}); errorValue == nil {
		return nil
	}
	buildContextPath := filepath.Join(normalizedWorkDirectoryPath, "tenant")
	if !directoryExists(buildContextPath) {
		return fmt.Errorf("tenant image build context is required at %s; stage tenant Dockerfile, binaries, and migrations there first", buildContextPath)
	}
	return runtime.executor().Run(ContainerCommandInvocation{
		ExecutableName: "docker",
		Arguments:      []string{"build", "-t", normalizedImageName, buildContextPath},
		Stdout:         runtime.output(),
		Stderr:         runtime.errorOutput(),
	})
}

func ContainerTenantForIndex(index int) (ContainerTenant, error) {
	if index <= 0 {
		return ContainerTenant{}, errors.New("tenant index must be positive")
	}
	indexText := fmt.Sprintf("%02d", index)
	return containerTenantFromIndexText(indexText)
}

func ParseContainerTenant(value string) (ContainerTenant, error) {
	matches := containerTenantPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 2 {
		return ContainerTenant{}, errors.New("tenant must be tenantNN or tenant_NN with a zero-padded numeric index")
	}
	return containerTenantFromIndexText(matches[1])
}

func ParseMattermostToken(document []byte) (string, error) {
	var tokenList []struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(document, &tokenList) == nil {
		for _, entry := range tokenList {
			token := strings.TrimSpace(entry.Token)
			if token != "" {
				return token, nil
			}
		}
	}
	var tokenObject struct {
		Token string `json:"token"`
	}
	if errorValue := json.Unmarshal(document, &tokenObject); errorValue != nil {
		return "", errorValue
	}
	token := strings.TrimSpace(tokenObject.Token)
	if token == "" {
		return "", errors.New("mmctl token output did not contain token")
	}
	return token, nil
}

func RenderContainerTenantCompose(tenants []ContainerTenant, imageName string, openRouterKeyPath string, workDirectoryPath string) string {
	var buffer bytes.Buffer
	buffer.WriteString("name: internkim-poc-tenants\n\n")
	buffer.WriteString("networks:\n")
	buffer.WriteString("  internkim-poc:\n")
	buffer.WriteString("    external: true\n\n")
	buffer.WriteString("services:\n")
	normalizedImageName := defaultContainerTenantImageName(imageName)
	mountOpenRouterKeyPath := containerComposeMountPath(workDirectoryPath, openRouterKeyPath)
	for _, tenant := range tenants {
		buffer.WriteString("  " + tenant.RuntimeID + ":\n")
		buffer.WriteString("    image: " + normalizedImageName + "\n")
		buffer.WriteString("    networks:\n")
		buffer.WriteString("      internkim-poc:\n")
		buffer.WriteString("        aliases: [" + tenant.RuntimeID + "]\n")
		buffer.WriteString("    environment:\n")
		buffer.WriteString("      POSTGRES_HOST: postgres\n")
		buffer.WriteString("      MATTERMOST_HOST: mattermost\n")
		buffer.WriteString("      MATTERMOST_TEAM: " + tenant.TeamName + "\n")
		buffer.WriteString("      BOT_USERNAME: " + tenant.AgentUsername + "\n")
		buffer.WriteString("      ENABLE_ADMIND: \"1\"\n")
		buffer.WriteString("    volumes:\n")
		buffer.WriteString("      - ./config/" + tenant.RuntimeID + ":/etc/blueclaw:rw\n")
		buffer.WriteString("      - ./workspace/" + tenant.RuntimeID + ":/workspace:rw\n")
		buffer.WriteString("      - " + mountOpenRouterKeyPath + ":/secrets/openrouter-key:ro\n")
		buffer.WriteString("      - ./secrets/" + tenant.RuntimeID + "/mattermost-bot-token:/secrets/mattermost-bot-token:ro\n")
		buffer.WriteString("      - ./secrets/" + tenant.RuntimeID + "/mattermost-bot-token:/root/.internkim/secrets/mattermost-bot-token:ro\n")
		buffer.WriteString("      - ./secrets/" + tenant.RuntimeID + "/mm-admin-pass:/root/.internkim/secrets/mm-admin-pass:ro\n")
		buffer.WriteString("      - ./secrets/" + tenant.RuntimeID + "/admin-email:/root/.internkim/secrets/admin-email:ro\n")
		buffer.WriteString("      - ./secrets/" + tenant.RuntimeID + "/device-url:/root/.internkim/env/device-url:ro\n")
		buffer.WriteString("      - ./secrets/" + tenant.RuntimeID + "/flow-public-url:/root/.internkim/env/flow-public-url:ro\n")
		buffer.WriteString("    restart: on-failure\n")
	}
	return buffer.String()
}

func ResolveContainerWorkDirectoryPath(path string) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		trimmedPath = DefaultContainerWorkDirectoryPath
	}
	if trimmedPath == "~" || strings.HasPrefix(trimmedPath, "~/") {
		homeDirectoryPath, errorValue := os.UserHomeDir()
		if errorValue != nil {
			return "", errorValue
		}
		if trimmedPath == "~" {
			trimmedPath = homeDirectoryPath
		} else {
			trimmedPath = filepath.Join(homeDirectoryPath, strings.TrimPrefix(trimmedPath, "~/"))
		}
	}
	absolutePath, errorValue := filepath.Abs(trimmedPath)
	if errorValue != nil {
		return "", errorValue
	}
	return filepath.Clean(absolutePath), nil
}

func containerTenantFromIndexText(indexText string) (ContainerTenant, error) {
	index, errorValue := strconv.Atoi(indexText)
	if errorValue != nil || index <= 0 {
		return ContainerTenant{}, errors.New("tenant index must be positive")
	}
	paddedIndexText := fmt.Sprintf("%02d", index)
	runtimeID := "tenant_" + paddedIndexText
	teamName := "tenant" + paddedIndexText
	return ContainerTenant{
		Index:         index,
		TenantID:      teamName,
		RuntimeID:     runtimeID,
		TeamName:      teamName,
		DatabaseName:  runtimeID,
		AgentUsername: "internkim" + paddedIndexText,
		ContainerName: runtimeID,
		DisplayName:   "Tenant " + paddedIndexText,
	}, nil
}

func normalizeContainerInfraUpOptions(options ContainerInfraUpOptions) (ContainerInfraUpOptions, error) {
	workDirectoryPath, errorValue := ResolveContainerWorkDirectoryPath(options.WorkDirectoryPath)
	if errorValue != nil {
		return ContainerInfraUpOptions{}, errorValue
	}
	tenantCount := options.TenantCount
	if tenantCount <= 0 {
		tenantCount = 10
	}
	return ContainerInfraUpOptions{
		WorkDirectoryPath:   workDirectoryPath,
		TenantCount:         tenantCount,
		OpenRouterKeyPath:   resolveContainerOpenRouterKeyPath(workDirectoryPath, options.OpenRouterKeyPath),
		MattermostPublicURL: defaultContainerMattermostURL(options.MattermostPublicURL),
	}, nil
}

func normalizeContainerTenantAddOptions(options ContainerTenantAddOptions) (ContainerTenantAddOptions, ContainerTenant, error) {
	workDirectoryPath, errorValue := ResolveContainerWorkDirectoryPath(options.WorkDirectoryPath)
	if errorValue != nil {
		return ContainerTenantAddOptions{}, ContainerTenant{}, errorValue
	}
	tenant, errorValue := ParseContainerTenant(options.TenantID)
	if errorValue != nil {
		return ContainerTenantAddOptions{}, ContainerTenant{}, errorValue
	}
	return ContainerTenantAddOptions{
		WorkDirectoryPath:     workDirectoryPath,
		TenantID:              tenant.TenantID,
		ModelName:             defaultContainerTenantModelName(options.ModelName),
		ImageName:             defaultContainerTenantImageName(options.ImageName),
		OpenRouterKeyPath:     resolveContainerOpenRouterKeyPath(workDirectoryPath, options.OpenRouterKeyPath),
		AdminEmail:            defaultContainerCompanyAdminEmail(tenant, options.AdminEmail),
		MattermostPublicURL:   defaultContainerMattermostURL(options.MattermostPublicURL),
		MattermostLocalURL:    defaultContainerMattermostLocalURL(options.MattermostLocalURL),
		FlowHostBaseLabel:     defaultContainerFlowHostBaseLabel(options.FlowHostBaseLabel),
		CloudflareEnvironment: resolveContainerCloudflareEnvironmentPath(workDirectoryPath, options.CloudflareEnvironment),
		CloudflareAPIBaseURL:  firstNonEmptyContainerString(options.CloudflareAPIBaseURL, DefaultCloudflareAPIBaseURL),
		CloudflareAPIToken:    strings.TrimSpace(options.CloudflareAPIToken),
		CloudflareAccountID:   strings.TrimSpace(options.CloudflareAccountID),
		CloudflareZoneID:      strings.TrimSpace(options.CloudflareZoneID),
		CloudflareZoneName:    defaultContainerCloudflareZoneName(options.CloudflareZoneName),
		CloudflareTunnelName:  defaultContainerCloudflareTunnelName(options.CloudflareTunnelName),
		CloudflareTunnelID:    strings.TrimSpace(options.CloudflareTunnelID),
	}, tenant, nil
}

func normalizeContainerTenantRemoveOptions(options ContainerTenantRemoveOptions) (ContainerTenantRemoveOptions, ContainerTenant, error) {
	workDirectoryPath, errorValue := ResolveContainerWorkDirectoryPath(options.WorkDirectoryPath)
	if errorValue != nil {
		return ContainerTenantRemoveOptions{}, ContainerTenant{}, errorValue
	}
	tenant, errorValue := ParseContainerTenant(options.TenantID)
	if errorValue != nil {
		return ContainerTenantRemoveOptions{}, ContainerTenant{}, errorValue
	}
	return ContainerTenantRemoveOptions{
		WorkDirectoryPath: workDirectoryPath,
		TenantID:          tenant.TenantID,
		PurgeData:         options.PurgeData,
		ImageName:         defaultContainerTenantImageName(options.ImageName),
		OpenRouterKeyPath: resolveContainerOpenRouterKeyPath(workDirectoryPath, options.OpenRouterKeyPath),
	}, tenant, nil
}

func normalizeContainerResetOptions(options ContainerResetOptions) (ContainerResetOptions, error) {
	workDirectoryPath, errorValue := ResolveContainerWorkDirectoryPath(options.WorkDirectoryPath)
	if errorValue != nil {
		return ContainerResetOptions{}, errorValue
	}
	return ContainerResetOptions{
		WorkDirectoryPath:      workDirectoryPath,
		Purge:                  options.Purge,
		ImageName:              defaultContainerTenantImageName(options.ImageName),
		OpenRouterKeyPath:      resolveContainerOpenRouterKeyPath(workDirectoryPath, options.OpenRouterKeyPath),
		ConfirmSuperAdmin:      options.ConfirmSuperAdmin,
		SuperAdminPasswordPath: strings.TrimSpace(options.SuperAdminPasswordPath),
		MattermostLocalURL:     defaultContainerMattermostLocalURL(options.MattermostLocalURL),
		FlowHostBaseLabel:      defaultContainerFlowHostBaseLabel(options.FlowHostBaseLabel),
		CloudflareEnvironment:  resolveContainerCloudflareEnvironmentPath(workDirectoryPath, options.CloudflareEnvironment),
		CloudflareAPIBaseURL:   firstNonEmptyContainerString(options.CloudflareAPIBaseURL, DefaultCloudflareAPIBaseURL),
		CloudflareAPIToken:     strings.TrimSpace(options.CloudflareAPIToken),
		CloudflareAccountID:    strings.TrimSpace(options.CloudflareAccountID),
		CloudflareZoneID:       strings.TrimSpace(options.CloudflareZoneID),
		CloudflareZoneName:     defaultContainerCloudflareZoneName(options.CloudflareZoneName),
		CloudflareTunnelName:   defaultContainerCloudflareTunnelName(options.CloudflareTunnelName),
		CloudflareTunnelID:     strings.TrimSpace(options.CloudflareTunnelID),
	}, nil
}

func normalizeContainerUpOptions(options ContainerUpOptions) (ContainerUpOptions, error) {
	workDirectoryPath, errorValue := ResolveContainerWorkDirectoryPath(options.WorkDirectoryPath)
	if errorValue != nil {
		return ContainerUpOptions{}, errorValue
	}
	if options.Count <= 0 {
		return ContainerUpOptions{}, errors.New("--count must be positive")
	}
	return ContainerUpOptions{
		WorkDirectoryPath: workDirectoryPath,
		Count:             options.Count,
		ModelName:         defaultContainerTenantModelName(options.ModelName),
		ImageName:         defaultContainerTenantImageName(options.ImageName),
		OpenRouterKeyPath: resolveContainerOpenRouterKeyPath(workDirectoryPath, options.OpenRouterKeyPath),
	}, nil
}

func resolveContainerOpenRouterKeyPath(workDirectoryPath string, openRouterKeyPath string) string {
	if strings.TrimSpace(openRouterKeyPath) == "" {
		return filepath.Join(workDirectoryPath, "secrets", "openrouter-key")
	}
	if filepath.IsAbs(openRouterKeyPath) {
		return filepath.Clean(openRouterKeyPath)
	}
	return filepath.Clean(openRouterKeyPath)
}

func defaultContainerTenantImageName(imageName string) string {
	if strings.TrimSpace(imageName) == "" {
		return DefaultContainerTenantImageName
	}
	return strings.TrimSpace(imageName)
}

func defaultContainerTenantModelName(modelName string) string {
	if strings.TrimSpace(modelName) == "" {
		return DefaultContainerTenantModelName
	}
	return strings.TrimSpace(modelName)
}

func defaultContainerMattermostURL(mattermostURL string) string {
	if strings.TrimSpace(mattermostURL) == "" {
		return DefaultContainerMattermostURL
	}
	return strings.TrimRight(strings.TrimSpace(mattermostURL), "/")
}

func defaultContainerMattermostLocalURL(mattermostURL string) string {
	if strings.TrimSpace(mattermostURL) == "" {
		return "http://localhost:8065"
	}
	return strings.TrimRight(strings.TrimSpace(mattermostURL), "/")
}

func defaultContainerFlowHostBaseLabel(label string) string {
	if strings.TrimSpace(label) == "" {
		return DefaultContainerFlowHostBaseLabel
	}
	return strings.Trim(strings.ToLower(strings.TrimSpace(label)), ".")
}

func defaultContainerCloudflareZoneName(zoneName string) string {
	if strings.TrimSpace(zoneName) == "" {
		return DefaultContainerCloudflareZone
	}
	return strings.Trim(strings.ToLower(strings.TrimSpace(zoneName)), ".")
}

func defaultContainerCloudflareTunnelName(tunnelName string) string {
	if strings.TrimSpace(tunnelName) == "" {
		return DefaultContainerTunnelName
	}
	return strings.TrimSpace(tunnelName)
}

func defaultContainerCompanyAdminEmail(tenant ContainerTenant, adminEmail string) string {
	if strings.TrimSpace(adminEmail) != "" {
		return strings.ToLower(strings.TrimSpace(adminEmail))
	}
	return "admin" + fmt.Sprintf("%02d", tenant.Index) + "@intern.kim"
}

func resolveContainerCloudflareEnvironmentPath(workDirectoryPath string, path string) string {
	if strings.TrimSpace(path) == "" {
		return filepath.Join(workDirectoryPath, "cf.env")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(workDirectoryPath, path)
}

func ensureContainerOpenRouterKey(path string) error {
	if !fileExists(path) {
		return fmt.Errorf("OpenRouter key file is required at %s; place it there or pass --openrouter-key", path)
	}
	return nil
}

func writeContainerInfraFiles(workDirectoryPath string, mattermostPublicURL string) error {
	if errorValue := os.MkdirAll(filepath.Join(workDirectoryPath, "infra", "postgres-init"), 0o755); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Join(workDirectoryPath, "config"), 0o755); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Join(workDirectoryPath, "secrets"), 0o700); errorValue != nil {
		return errorValue
	}
	superuserPassword, errorValue := ensureContainerSuperuserPassword(workDirectoryPath)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(containerInfraComposePath(workDirectoryPath), []byte(containerInfraComposeDocument(superuserPassword, mattermostPublicURL)), 0o644); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(filepath.Join(workDirectoryPath, "infra", "postgres-init", "01-create-databases.sh"), []byte(containerPostgresInitDocument()), 0o755)
}

func writeContainerTenantFiles(options ContainerTenantAddOptions, tenant ContainerTenant, mattermostToken string, databasePassword string, operatorAdminPassword string) error {
	if errorValue := os.MkdirAll(containerTenantConfigPath(options.WorkDirectoryPath, tenant), 0o755); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(containerTenantWorkspacePath(options.WorkDirectoryPath, tenant), 0o755); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(containerTenantSecretPath(options.WorkDirectoryPath, tenant), 0o700); errorValue != nil {
		return errorValue
	}
	runtimeDocument, errorValue := blueclaw.BlueclawRuntimeConfigDocumentWithOptions(blueclaw.RuntimeConfigOptions{
		DirectExecution:          true,
		ModelName:                options.ModelName,
		CapabilitySocketPath:     "/run/internkim/capability.sock",
		DatabaseConnectionString: tenantDatabaseConnectionString(tenant.RuntimeID, databasePassword, "postgres:5432"),
		MigrationDirectoryPath:   "/opt/blueclaw/migrations",
		WorkspaceRootPath:        "/workspace",
		POSIXHelperPath:          "",
		MattermostBaseURL:        "http://mattermost:8065",
	})
	if errorValue != nil {
		return errorValue
	}
	policyDocument, errorValue := blueclaw.BlueclawPolicyDocument(options.AdminEmail)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(filepath.Join(containerTenantConfigPath(options.WorkDirectoryPath, tenant), "runtime.json"), []byte(runtimeDocument), 0o644); errorValue != nil {
		return errorValue
	}
	policyPath := filepath.Join(containerTenantConfigPath(options.WorkDirectoryPath, tenant), "policy.json")
	if !fileExists(policyPath) {
		if errorValue := os.WriteFile(policyPath, []byte(policyDocument), 0o644); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := os.WriteFile(filepath.Join(containerTenantSecretPath(options.WorkDirectoryPath, tenant), "mattermost-bot-token"), []byte(strings.TrimSpace(mattermostToken)), 0o600); errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(filepath.Join(containerTenantSecretPath(options.WorkDirectoryPath, tenant), "mm-admin-pass"), []byte(strings.TrimSpace(operatorAdminPassword)+"\n"), 0o600); errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(filepath.Join(containerTenantSecretPath(options.WorkDirectoryPath, tenant), "admin-email"), []byte(strings.TrimSpace(options.AdminEmail)+"\n"), 0o600); errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(filepath.Join(containerTenantSecretPath(options.WorkDirectoryPath, tenant), "device-url"), []byte(strings.TrimRight(options.MattermostPublicURL, "/")+"\n"), 0o600); errorValue != nil {
		return errorValue
	}
	flowPublicURL := "https://" + containerTenantFlowHostname(options, tenant)
	return os.WriteFile(filepath.Join(containerTenantSecretPath(options.WorkDirectoryPath, tenant), "flow-public-url"), []byte(flowPublicURL+"\n"), 0o600)
}

func (runtime ContainerRuntime) ensureTenantDatabase(workDirectoryPath string, tenant ContainerTenant, password string) error {
	database := containerTenantPostgresDatabase{runtime: runtime, workDirectoryPath: workDirectoryPath}
	return createTenantRole(context.Background(), database, tenant.RuntimeID, password)
}

func (runtime ContainerRuntime) dropTenantDatabase(workDirectoryPath string, tenant ContainerTenant) error {
	database := containerTenantPostgresDatabase{runtime: runtime, workDirectoryPath: workDirectoryPath}
	return dropTenantRole(context.Background(), database, tenant.RuntimeID)
}

func (runtime ContainerRuntime) lockMattermostDatabase(workDirectoryPath string) error {
	database := containerTenantPostgresDatabase{runtime: runtime, workDirectoryPath: workDirectoryPath}
	return database.ExecutePostgres(context.Background(), `REVOKE ALL ON DATABASE "mattermost" FROM PUBLIC;`+"\n")
}

func createTenantRole(ctx context.Context, database tenantPostgresDatabase, runtimeID string, password string) error {
	if errorValue := validateContainerRuntimeID(runtimeID); errorValue != nil {
		return errorValue
	}
	if errorValue := validateTenantDatabasePassword(password); errorValue != nil {
		return errorValue
	}
	return database.ExecutePostgres(ctx, createTenantRoleSQL(runtimeID, password))
}

func dropTenantRole(ctx context.Context, database tenantPostgresDatabase, runtimeID string) error {
	if errorValue := validateContainerRuntimeID(runtimeID); errorValue != nil {
		return errorValue
	}
	return database.ExecutePostgres(ctx, dropTenantRoleSQL(runtimeID))
}

func createTenantRoleSQL(runtimeID string, password string) string {
	identifier := quoteTenantPostgresIdentifier(runtimeID)
	runtimeIDLiteral := quoteTenantPostgresLiteral(runtimeID)
	passwordLiteral := quoteTenantPostgresLiteral(password)
	return strings.Join([]string{
		"DO $$",
		"BEGIN",
		"  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = " + runtimeIDLiteral + ") THEN",
		"    CREATE ROLE " + identifier + " WITH LOGIN PASSWORD " + passwordLiteral + ";",
		"  ELSE",
		"    ALTER ROLE " + identifier + " WITH LOGIN PASSWORD " + passwordLiteral + ";",
		"  END IF;",
		"END",
		"$$;",
		"SELECT 'CREATE DATABASE " + identifier + " OWNER " + identifier + "' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = " + runtimeIDLiteral + ")\\gexec",
		"ALTER DATABASE " + identifier + " OWNER TO " + identifier + ";",
		"REVOKE CONNECT ON DATABASE " + identifier + " FROM PUBLIC;",
		"GRANT CONNECT ON DATABASE " + identifier + " TO " + identifier + ";",
		"",
	}, "\n")
}

func dropTenantRoleSQL(runtimeID string) string {
	identifier := quoteTenantPostgresIdentifier(runtimeID)
	return strings.Join([]string{
		"DROP DATABASE IF EXISTS " + identifier + ";",
		"DROP ROLE IF EXISTS " + identifier + ";",
		"",
	}, "\n")
}

func (database containerTenantPostgresDatabase) ExecutePostgres(ctx context.Context, statement string) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	return database.runtime.executor().Run(ContainerCommandInvocation{
		ExecutableName: "docker",
		Arguments: []string{
			"compose", "-f", containerInfraComposePath(database.workDirectoryPath),
			"exec", "-T", "postgres",
			"psql", "-v", "ON_ERROR_STOP=1", "-U", "internkim", "-d", "postgres",
		},
		Stdin:  strings.NewReader(statement),
		Stdout: database.runtime.output(),
		Stderr: database.runtime.errorOutput(),
	})
}

func (runtime ContainerRuntime) ensureTenantMattermost(options ContainerTenantAddOptions, tenant ContainerTenant, operatorAdminPassword containerStoredPassword, companyAdminPassword containerStoredPassword) (string, error) {
	adminUsername := containerCompanyAdminUsername(tenant)
	agentPassword, errorValue := generateTenantLoginPassword()
	if errorValue != nil {
		return "", errorValue
	}
	_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"user", "create", "--email", "admin@intern.kim", "--username", "admin", "--password", operatorAdminPassword.Value, "--system-admin"})
	if operatorAdminPassword.IsNew {
		_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"user", "change-password", "admin", "--password", operatorAdminPassword.Value})
	}
	_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"config", "set", "TeamSettings.TeammateNameDisplay", containerMattermostNameDisplay})
	_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"config", "set", "LocalizationSettings.DefaultClientLocale", containerMattermostDefaultLocale})
	_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"config", "set", "LocalizationSettings.DefaultServerLocale", containerMattermostDefaultLocale})
	_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"team", "create", "--name", tenant.TeamName, "--display-name", tenant.DisplayName})
	_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"team", "users", "add", tenant.TeamName, "admin"})
	_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"user", "create", "--email", tenant.AgentUsername + "@intern.kim", "--username", tenant.AgentUsername, "--password", agentPassword})
	_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"team", "users", "add", tenant.TeamName, tenant.AgentUsername})
	_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"user", "create", "--email", options.AdminEmail, "--username", adminUsername, "--password", companyAdminPassword.Value})
	if companyAdminPassword.IsNew {
		_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"user", "change-password", adminUsername, "--password", companyAdminPassword.Value})
	}
	_ = runtime.runMattermostCommand(options.WorkDirectoryPath, []string{"team", "users", "add", tenant.TeamName, adminUsername})
	client := containerMattermostClient{baseURL: options.MattermostLocalURL, httpClient: http.DefaultClient}
	adminToken, errorValue := client.login(context.Background(), operatorAdminPassword.Value)
	if errorValue != nil {
		return "", errorValue
	}
	agentUserID, errorValue := client.patchUserProfile(context.Background(), adminToken, tenant.AgentUsername, "Intern", "Kim", "김인턴")
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := runtime.uploadContainerBotProfileImage(context.Background(), options, client, adminToken, agentUserID); errorValue != nil {
		fmt.Fprintf(runtime.errorOutput(), "[add] bot profile image upload skipped: %v\n", errorValue)
	}
	if errorValue := client.promoteTeamAdmin(context.Background(), adminToken, tenant.TeamName, adminUsername); errorValue != nil {
		return "", errorValue
	}
	companyAdminUserID, errorValue := client.patchUserProfile(context.Background(), adminToken, adminUsername, "Admin", "", "admin")
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := client.ensureDirectChannel(context.Background(), adminToken, companyAdminUserID, agentUserID); errorValue != nil {
		fmt.Fprintf(runtime.errorOutput(), "[add] 김인턴 direct message channel skipped: %v\n", errorValue)
	}
	output, errorValue := runtime.mattermostOutput(options.WorkDirectoryPath, []string{"token", "generate", tenant.AgentUsername, "poc-token", "--json"})
	if errorValue != nil {
		return "", fmt.Errorf("Mattermost token generation failed for %s: %w", tenant.AgentUsername, errorValue)
	}
	return ParseMattermostToken(output)
}

func containerCompanyAdminUsername(tenant ContainerTenant) string {
	return "admin" + fmt.Sprintf("%02d", tenant.Index)
}

func ensureTenantDatabasePassword(workDirectoryPath string, tenant ContainerTenant) (string, error) {
	path := tenantDatabasePasswordPath(workDirectoryPath, tenant)
	document, errorValue := os.ReadFile(path)
	if errorValue == nil {
		return normalizeTenantDatabasePassword(string(document))
	}
	if !errors.Is(errorValue, os.ErrNotExist) {
		return "", errorValue
	}
	password, errorValue := generateTenantDatabasePassword()
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return "", errorValue
	}
	return password, os.WriteFile(path, []byte(password+"\n"), 0o600)
}

func ensureTenantCompanyAdminPassword(workDirectoryPath string, tenant ContainerTenant) (containerStoredPassword, error) {
	path := tenantCompanyAdminPasswordPath(workDirectoryPath, tenant)
	password, found, errorValue := readContainerStoredPassword(path, "stored company admin password is empty")
	if found || errorValue != nil {
		return containerStoredPassword{Value: password}, errorValue
	}
	legacyPath := legacyTenantCompanyAdminPasswordPath(workDirectoryPath, tenant)
	password, found, errorValue = readContainerStoredPassword(legacyPath, "stored company admin password is empty")
	if found || errorValue != nil {
		if errorValue != nil {
			return containerStoredPassword{}, errorValue
		}
		return containerStoredPassword{Value: password}, writeContainerStoredPassword(path, password)
	}
	credentials, errorValue := GenerateTenantCredentials()
	if errorValue != nil {
		return containerStoredPassword{}, errorValue
	}
	return containerStoredPassword{Value: credentials.AdminPassword, IsNew: true}, writeContainerStoredPassword(path, credentials.AdminPassword)
}

func generateTenantDatabasePassword() (string, error) {
	document := make([]byte, 32)
	if _, errorValue := rand.Read(document); errorValue != nil {
		return "", errorValue
	}
	return base64.RawURLEncoding.EncodeToString(document), nil
}

func readContainerStoredPassword(path string, emptyMessage string) (string, bool, error) {
	document, errorValue := os.ReadFile(path)
	if errors.Is(errorValue, os.ErrNotExist) {
		return "", false, nil
	}
	if errorValue != nil {
		return "", false, errorValue
	}
	password := strings.TrimSpace(string(document))
	if password == "" {
		return "", false, errors.New(emptyMessage)
	}
	return password, true, nil
}

func writeContainerStoredPassword(path string, password string) error {
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(password)+"\n"), 0o600)
}

func containerSuperuserPasswordPath(workDirectoryPath string) string {
	return filepath.Join(workDirectoryPath, "secrets", "postgres-superuser-password")
}

func containerMattermostAdminPasswordPath(workDirectoryPath string) string {
	return filepath.Join(workDirectoryPath, "secrets", "mm-admin-pass")
}

func ensureContainerSuperuserPassword(workDirectoryPath string) (string, error) {
	path := containerSuperuserPasswordPath(workDirectoryPath)
	document, errorValue := os.ReadFile(path)
	if errorValue == nil {
		return normalizeTenantDatabasePassword(string(document))
	}
	if !errors.Is(errorValue, os.ErrNotExist) {
		return "", errorValue
	}
	password, errorValue := generateTenantDatabasePassword()
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return "", errorValue
	}
	return password, os.WriteFile(path, []byte(password+"\n"), 0o600)
}

func ensureContainerMattermostAdminPassword(workDirectoryPath string) (containerStoredPassword, error) {
	path := containerMattermostAdminPasswordPath(workDirectoryPath)
	password, found, errorValue := readContainerStoredPassword(path, "stored Mattermost super-admin password is empty")
	if found || errorValue != nil {
		return containerStoredPassword{Value: password}, errorValue
	}
	credentials, errorValue := GenerateTenantCredentials()
	if errorValue != nil {
		return containerStoredPassword{}, errorValue
	}
	return containerStoredPassword{Value: credentials.AdminPassword, IsNew: true}, writeContainerStoredPassword(path, credentials.AdminPassword)
}

func verifyContainerResetSuperAdmin(options ContainerResetOptions) error {
	if !options.ConfirmSuperAdmin {
		return errors.New("reset requires --confirm-super-admin")
	}
	if strings.TrimSpace(options.SuperAdminPasswordPath) == "" {
		return errors.New("reset requires --super-admin-password pointing to the Mattermost super-admin password file")
	}
	expectedPassword, errorValue := ensureContainerMattermostAdminPassword(options.WorkDirectoryPath)
	if errorValue != nil {
		return errorValue
	}
	document, errorValue := os.ReadFile(options.SuperAdminPasswordPath)
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(string(document)) != expectedPassword.Value {
		return errors.New("super-admin password verification failed")
	}
	return nil
}

func normalizeTenantDatabasePassword(document string) (string, error) {
	password := strings.TrimSpace(document)
	if errorValue := validateTenantDatabasePassword(password); errorValue != nil {
		return "", errorValue
	}
	return password, nil
}

func tenantDatabaseConnectionString(runtimeID string, password string, host string) string {
	return "postgres://" + runtimeID + ":" + password + "@" + host + "/" + runtimeID + "?sslmode=disable"
}

func validateContainerRuntimeID(runtimeID string) error {
	if !containerRuntimeIDPattern.MatchString(runtimeID) {
		return errors.New("tenant runtime id must be tenant_NN with a zero-padded numeric index")
	}
	return nil
}

func validateTenantDatabasePassword(password string) error {
	if !containerDatabasePasswordPattern.MatchString(password) {
		return errors.New("tenant database password must be a generated URL-safe secret")
	}
	return nil
}

func quoteTenantPostgresIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func quoteTenantPostgresLiteral(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `''`) + `'`
}

func (runtime ContainerRuntime) renderTenantsCompose(workDirectoryPath string, imageName string, openRouterKeyPath string) error {
	tenants, errorValue := containerTenantsFromConfig(workDirectoryPath)
	if errorValue != nil {
		return errorValue
	}
	if len(tenants) == 0 {
		if fileExists(containerTenantsComposePath(workDirectoryPath)) {
			if errorValue := os.Remove(containerTenantsComposePath(workDirectoryPath)); errorValue != nil {
				return errorValue
			}
		}
		return nil
	}
	document := RenderContainerTenantCompose(tenants, imageName, openRouterKeyPath, workDirectoryPath)
	return os.WriteFile(containerTenantsComposePath(workDirectoryPath), []byte(document), 0o644)
}

type containerMattermostClient struct {
	baseURL    string
	httpClient *http.Client
}

type containerMattermostUserResponse struct {
	ID string `json:"id"`
}

type containerMattermostTeamResponse struct {
	ID string `json:"id"`
}

func (client containerMattermostClient) login(ctx context.Context, password string) (string, error) {
	var responseValue containerMattermostUserResponse
	response, errorValue := client.request(ctx, http.MethodPost, "/api/v4/users/login", "", map[string]string{
		"login_id": "admin",
		"password": password,
	}, &responseValue)
	if errorValue != nil {
		return "", errorValue
	}
	token := strings.TrimSpace(response.Header.Get("Token"))
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || token == "" {
		return "", errors.New("Mattermost super-admin login failed")
	}
	return token, nil
}

func (runtime ContainerRuntime) uploadContainerBotProfileImage(ctx context.Context, options ContainerTenantAddOptions, client containerMattermostClient, token string, userID string) error {
	return client.uploadUserImage(ctx, token, userID, botassets.AvatarFileName(), botassets.AvatarPNG())
}

func (client containerMattermostClient) patchUserProfile(ctx context.Context, token string, username string, firstName string, lastName string, nickname string) (string, error) {
	userID, errorValue := client.userIDByUsername(ctx, token, username)
	if errorValue != nil {
		return "", errorValue
	}
	_, errorValue = client.request(ctx, http.MethodPut, "/api/v4/users/"+url.PathEscape(userID)+"/patch", token, map[string]string{
		"first_name": firstName,
		"last_name":  lastName,
		"nickname":   nickname,
		"locale":     containerMattermostDefaultLocale,
	}, nil)
	return userID, errorValue
}

func (client containerMattermostClient) uploadUserImage(ctx context.Context, token string, userID string, filename string, imageDocument []byte) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	imagePart, errorValue := writer.CreateFormFile("image", filename)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := imagePart.Write(imageDocument); errorValue != nil {
		return errorValue
	}
	if errorValue := writer.Close(); errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(client.baseURL, "/")+"/api/v4/users/"+url.PathEscape(userID)+"/image", body)
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	httpClient := client.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, errorValue := httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Mattermost image upload failed: %s", response.Status)
	}
	return nil
}

func (client containerMattermostClient) promoteTeamAdmin(ctx context.Context, token string, teamName string, username string) error {
	teamID, errorValue := client.teamIDByName(ctx, token, teamName)
	if errorValue != nil {
		return errorValue
	}
	userID, errorValue := client.userIDByUsername(ctx, token, username)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = client.request(ctx, http.MethodPut, "/api/v4/teams/"+url.PathEscape(teamID)+"/members/"+url.PathEscape(userID)+"/schemeRoles", token, map[string]bool{
		"scheme_admin": true,
		"scheme_user":  true,
	}, nil)
	return errorValue
}

func (client containerMattermostClient) ensureDirectChannel(ctx context.Context, token string, firstUserID string, secondUserID string) error {
	_, errorValue := client.request(ctx, http.MethodPost, "/api/v4/channels/direct", token, []string{firstUserID, secondUserID}, nil)
	return errorValue
}

func (client containerMattermostClient) teamIDByName(ctx context.Context, token string, teamName string) (string, error) {
	var responseValue containerMattermostTeamResponse
	response, errorValue := client.request(ctx, http.MethodGet, "/api/v4/teams/name/"+url.PathEscape(teamName), token, nil, &responseValue)
	if errorValue != nil {
		return "", errorValue
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || strings.TrimSpace(responseValue.ID) == "" {
		return "", errors.New("Mattermost team lookup failed")
	}
	return responseValue.ID, nil
}

func (client containerMattermostClient) userIDByUsername(ctx context.Context, token string, username string) (string, error) {
	var responseValue containerMattermostUserResponse
	response, errorValue := client.request(ctx, http.MethodGet, "/api/v4/users/username/"+url.PathEscape(username), token, nil, &responseValue)
	if errorValue != nil {
		return "", errorValue
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || strings.TrimSpace(responseValue.ID) == "" {
		return "", errors.New("Mattermost user lookup failed")
	}
	return responseValue.ID, nil
}

func (client containerMattermostClient) request(ctx context.Context, method string, path string, token string, body any, responseValue any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		document, errorValue := json.Marshal(body)
		if errorValue != nil {
			return nil, errorValue
		}
		reader = bytes.NewReader(document)
	}
	request, errorValue := http.NewRequestWithContext(ctx, method, strings.TrimRight(client.baseURL, "/")+path, reader)
	if errorValue != nil {
		return nil, errorValue
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	httpClient := client.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, errorValue := httpClient.Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if responseValue != nil {
		if errorValue := json.NewDecoder(response.Body).Decode(responseValue); errorValue != nil {
			return response, errorValue
		}
	} else {
		_, _ = io.Copy(io.Discard, response.Body)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return response, errors.New("Mattermost request failed: " + method + " " + path)
	}
	return response, nil
}

type containerCloudflareOptions struct {
	APIBaseURL string
	APIToken   string
	AccountID  string
	ZoneID     string
	TunnelName string
	TunnelID   string
	ZoneName   string
}

type containerCloudflareTunnelListResponse struct {
	Success bool `json:"success"`
	Result  []struct {
		ID string `json:"id"`
	} `json:"result"`
	Errors []cloudflareAPIMessage `json:"errors"`
}

type containerCloudflareDNSResponse struct {
	Success bool `json:"success"`
	Result  []struct {
		ID string `json:"id"`
	} `json:"result"`
	Errors []cloudflareAPIMessage `json:"errors"`
}

func syncContainerTenantFlowRoute(ctx context.Context, options ContainerTenantAddOptions, tenant ContainerTenant) error {
	cloudflareOptions, errorValue := containerCloudflareOptionsFromAddOptions(options)
	if errorValue != nil {
		return errorValue
	}
	client := containerCloudflareClient(cloudflareOptions)
	tunnelID, errorValue := client.resolveTunnelID(ctx, cloudflareOptions)
	if errorValue != nil {
		return errorValue
	}
	hostname := containerTenantFlowHostname(options, tenant)
	if errorValue := client.upsertFlowIngress(ctx, tunnelID, options.MattermostPublicURL, hostname, "http://"+tenant.RuntimeID+":18080"); errorValue != nil {
		return errorValue
	}
	return client.upsertCNAME(ctx, cloudflareOptions.ZoneID, hostname, tunnelID+".cfargotunnel.com")
}

func removeContainerTenantFlowRoute(ctx context.Context, options ContainerResetOptions, tenant ContainerTenant) (bool, error) {
	cloudflareOptions, errorValue := containerCloudflareOptionsFromResetOptions(options)
	if errorValue != nil {
		return false, errorValue
	}
	client := containerCloudflareClient(cloudflareOptions)
	tunnelID, errorValue := client.resolveTunnelID(ctx, cloudflareOptions)
	if errorValue != nil {
		return false, errorValue
	}
	hostname := containerTenantFlowHostnameForParts(options.FlowHostBaseLabel, options.CloudflareZoneName, tenant)
	removed, errorValue := client.removeFlowIngress(ctx, tunnelID, hostname)
	if errorValue != nil {
		return false, errorValue
	}
	if errorValue := client.deleteDNSRecord(ctx, cloudflareOptions.ZoneID, hostname); errorValue != nil {
		return false, errorValue
	}
	return removed, nil
}

func containerCloudflareOptionsFromAddOptions(options ContainerTenantAddOptions) (containerCloudflareOptions, error) {
	values := readContainerCloudflareEnvironment(options.CloudflareEnvironment)
	cloudflareOptions := containerCloudflareOptions{
		APIBaseURL: firstNonEmptyContainerString(options.CloudflareAPIBaseURL, DefaultCloudflareAPIBaseURL),
		APIToken:   firstNonEmptyContainerString(options.CloudflareAPIToken, values["CF_API_TOKEN"], values["CLOUDFLARE_API_TOKEN"]),
		AccountID:  firstNonEmptyContainerString(options.CloudflareAccountID, values["CF_ACCOUNT_ID"]),
		ZoneID:     firstNonEmptyContainerString(options.CloudflareZoneID, values["CF_ZONE_ID"]),
		TunnelName: firstNonEmptyContainerString(options.CloudflareTunnelName, values["TUNNEL_NAME"], DefaultContainerTunnelName),
		TunnelID:   firstNonEmptyContainerString(options.CloudflareTunnelID, values["CF_TUNNEL_ID"], values["TUNNEL_ID"]),
		ZoneName:   options.CloudflareZoneName,
	}
	return cloudflareOptions, validateContainerCloudflareOptions(cloudflareOptions)
}

func containerCloudflareOptionsFromResetOptions(options ContainerResetOptions) (containerCloudflareOptions, error) {
	values := readContainerCloudflareEnvironment(options.CloudflareEnvironment)
	cloudflareOptions := containerCloudflareOptions{
		APIBaseURL: firstNonEmptyContainerString(options.CloudflareAPIBaseURL, DefaultCloudflareAPIBaseURL),
		APIToken:   firstNonEmptyContainerString(options.CloudflareAPIToken, values["CF_API_TOKEN"], values["CLOUDFLARE_API_TOKEN"]),
		AccountID:  firstNonEmptyContainerString(options.CloudflareAccountID, values["CF_ACCOUNT_ID"]),
		ZoneID:     firstNonEmptyContainerString(options.CloudflareZoneID, values["CF_ZONE_ID"]),
		TunnelName: firstNonEmptyContainerString(options.CloudflareTunnelName, values["TUNNEL_NAME"], DefaultContainerTunnelName),
		TunnelID:   firstNonEmptyContainerString(options.CloudflareTunnelID, values["CF_TUNNEL_ID"], values["TUNNEL_ID"]),
		ZoneName:   options.CloudflareZoneName,
	}
	return cloudflareOptions, validateContainerCloudflareOptions(cloudflareOptions)
}

func validateContainerCloudflareOptions(options containerCloudflareOptions) error {
	if strings.TrimSpace(options.APIToken) == "" {
		return errors.New("Cloudflare API token is required in cf.env or --cloudflare-api-token")
	}
	if strings.TrimSpace(options.AccountID) == "" {
		return errors.New("Cloudflare account id is required in cf.env or --cloudflare-account-id")
	}
	if strings.TrimSpace(options.ZoneID) == "" {
		return errors.New("Cloudflare zone id is required in cf.env or --cloudflare-zone-id")
	}
	if strings.TrimSpace(options.TunnelID) == "" && strings.TrimSpace(options.TunnelName) == "" {
		return errors.New("Cloudflare tunnel id or tunnel name is required")
	}
	return nil
}

func readContainerCloudflareEnvironment(path string) map[string]string {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return map[string]string{}
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(document), "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" {
			values[key] = value
		}
	}
	return values
}

func containerCloudflareClient(options containerCloudflareOptions) cloudflareTunnelAPIClient {
	return cloudflareTunnelAPIClient{httpClient: http.DefaultClient, baseURL: firstNonEmptyContainerString(options.APIBaseURL, DefaultCloudflareAPIBaseURL), accountID: options.AccountID, tunnelID: options.TunnelID, apiToken: options.APIToken}
}

func (client cloudflareTunnelAPIClient) resolveTunnelID(ctx context.Context, options containerCloudflareOptions) (string, error) {
	if strings.TrimSpace(options.TunnelID) != "" {
		return strings.TrimSpace(options.TunnelID), nil
	}
	requestURL := strings.TrimRight(client.baseURL, "/") + "/accounts/" + url.PathEscape(options.AccountID) + "/cfd_tunnel?name=" + url.QueryEscape(options.TunnelName) + "&is_deleted=false"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if errorValue != nil {
		return "", errorValue
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(options.APIToken))
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	var document containerCloudflareTunnelListResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&document); errorValue != nil {
		return "", errorValue
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !document.Success || len(document.Result) == 0 || strings.TrimSpace(document.Result[0].ID) == "" {
		return "", errors.New("Cloudflare tunnel lookup failed: " + cloudflareErrorMessage(document.Errors))
	}
	return strings.TrimSpace(document.Result[0].ID), nil
}

func (client cloudflareTunnelAPIClient) upsertFlowIngress(ctx context.Context, tunnelID string, mattermostPublicURL string, hostname string, serviceURL string) error {
	originalTunnelID := client.tunnelID
	client.tunnelID = tunnelID
	configuration, errorValue := client.fetchConfiguration(ctx)
	if errorValue != nil {
		return errorValue
	}
	configuration.Ingress = upsertContainerFlowIngress(configuration.Ingress, mattermostPublicURL, hostname, serviceURL)
	errorValue = client.updateConfiguration(ctx, configuration)
	client.tunnelID = originalTunnelID
	return errorValue
}

func (client cloudflareTunnelAPIClient) removeFlowIngress(ctx context.Context, tunnelID string, hostname string) (bool, error) {
	originalTunnelID := client.tunnelID
	client.tunnelID = tunnelID
	configuration, errorValue := client.fetchConfiguration(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	updatedIngress, removed := removeContainerFlowIngress(configuration.Ingress, hostname)
	configuration.Ingress = updatedIngress
	errorValue = client.updateConfiguration(ctx, configuration)
	client.tunnelID = originalTunnelID
	return removed, errorValue
}

func (client cloudflareTunnelAPIClient) upsertCNAME(ctx context.Context, zoneID string, hostname string, target string) error {
	recordID, errorValue := client.dnsRecordID(ctx, zoneID, hostname)
	if errorValue != nil {
		return errorValue
	}
	method := http.MethodPost
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records"
	if recordID != "" {
		method = http.MethodPut
		path += "/" + url.PathEscape(recordID)
	}
	body := map[string]any{"type": "CNAME", "name": hostname, "content": target, "proxied": true}
	return client.cloudflareJSONRequest(ctx, method, path, body, nil)
}

func (client cloudflareTunnelAPIClient) deleteDNSRecord(ctx context.Context, zoneID string, hostname string) error {
	recordID, errorValue := client.dnsRecordID(ctx, zoneID, hostname)
	if errorValue != nil || recordID == "" {
		return errorValue
	}
	return client.cloudflareJSONRequest(ctx, http.MethodDelete, "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(recordID), nil, nil)
}

func (client cloudflareTunnelAPIClient) dnsRecordID(ctx context.Context, zoneID string, hostname string) (string, error) {
	var document containerCloudflareDNSResponse
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records?type=CNAME&name=" + url.QueryEscape(hostname)
	if errorValue := client.cloudflareJSONRequest(ctx, http.MethodGet, path, nil, &document); errorValue != nil {
		return "", errorValue
	}
	if len(document.Result) == 0 {
		return "", nil
	}
	return strings.TrimSpace(document.Result[0].ID), nil
}

func (client cloudflareTunnelAPIClient) cloudflareJSONRequest(ctx context.Context, method string, path string, body any, responseValue any) error {
	var reader io.Reader
	if body != nil {
		document, errorValue := json.Marshal(body)
		if errorValue != nil {
			return errorValue
		}
		reader = bytes.NewReader(document)
	}
	request, errorValue := http.NewRequestWithContext(ctx, method, strings.TrimRight(client.baseURL, "/")+path, reader)
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(client.apiToken))
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if responseValue != nil {
		if errorValue := json.NewDecoder(response.Body).Decode(responseValue); errorValue != nil {
			return errorValue
		}
		return nil
	}
	var document struct {
		Success bool                   `json:"success"`
		Errors  []cloudflareAPIMessage `json:"errors"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&document); errorValue != nil {
		return errorValue
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !document.Success {
		return errors.New("Cloudflare request failed: " + cloudflareErrorMessage(document.Errors))
	}
	return nil
}

func upsertContainerFlowIngress(ingress []cloudflareTunnelIngress, mattermostPublicURL string, hostname string, serviceURL string) []cloudflareTunnelIngress {
	primaryHostname := containerHostnameFromURL(mattermostPublicURL)
	result := []cloudflareTunnelIngress{}
	fallback := []cloudflareTunnelIngress{}
	hasPrimary := false
	for _, entry := range ingress {
		if entry.Hostname == hostname {
			continue
		}
		if entry.Hostname == primaryHostname {
			hasPrimary = true
		}
		if strings.TrimSpace(entry.Hostname) == "" {
			fallback = append(fallback, entry)
			continue
		}
		result = append(result, entry)
	}
	if primaryHostname != "" && !hasPrimary {
		result = append([]cloudflareTunnelIngress{{Hostname: primaryHostname, Service: "http://mattermost:8065"}}, result...)
	}
	result = append(result, cloudflareTunnelIngress{Hostname: hostname, Service: serviceURL})
	if len(fallback) == 0 {
		fallback = append(fallback, cloudflareTunnelIngress{Service: "http_status:404"})
	}
	return append(result, fallback...)
}

func removeContainerFlowIngress(ingress []cloudflareTunnelIngress, hostname string) ([]cloudflareTunnelIngress, bool) {
	result := []cloudflareTunnelIngress{}
	removed := false
	for _, entry := range ingress {
		if entry.Hostname == hostname {
			removed = true
			continue
		}
		result = append(result, entry)
	}
	return result, removed
}

func containerHostnameFromURL(value string) string {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(value))
	if errorValue == nil && parsedURL.Hostname() != "" {
		return parsedURL.Hostname()
	}
	return strings.Trim(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(value), "https://"), "http://"), "/")
}

func containerTenantFlowHostname(options ContainerTenantAddOptions, tenant ContainerTenant) string {
	return containerTenantFlowHostnameForParts(options.FlowHostBaseLabel, options.CloudflareZoneName, tenant)
}

func containerTenantFlowHostnameForParts(baseLabel string, zoneName string, tenant ContainerTenant) string {
	return strings.Trim(baseLabel, ".") + "-t" + fmt.Sprintf("%02d", tenant.Index) + "." + strings.Trim(zoneName, ".")
}

func containerTenantsFromConfig(workDirectoryPath string) ([]ContainerTenant, error) {
	configDirectoryPath := filepath.Join(workDirectoryPath, "config")
	entries, errorValue := os.ReadDir(configDirectoryPath)
	if errors.Is(errorValue, os.ErrNotExist) {
		return []ContainerTenant{}, nil
	}
	if errorValue != nil {
		return nil, errorValue
	}
	tenants := []ContainerTenant{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		tenant, errorValue := ParseContainerTenant(entry.Name())
		if errorValue != nil {
			continue
		}
		tenants = append(tenants, tenant)
	}
	sort.Slice(tenants, func(firstIndex int, secondIndex int) bool {
		return tenants[firstIndex].Index < tenants[secondIndex].Index
	})
	return tenants, nil
}

func (runtime ContainerRuntime) statusForTenant(workDirectoryPath string, tenant ContainerTenant) (ContainerTenantStatus, error) {
	containerState := runtime.tenantContainerState(workDirectoryPath, tenant)
	mattermostTeamPresent := runtime.mattermostTeamExists(workDirectoryPath, tenant)
	databasePresent, _ := runtime.tenantDatabaseExists(workDirectoryPath, tenant)
	companyAdminEmail := strings.TrimSpace(readContainerFileIfPresent(filepath.Join(containerTenantSecretPath(workDirectoryPath, tenant), "admin-email")))
	companyAdminPassword := strings.TrimSpace(readContainerFileIfPresent(tenantCompanyAdminPasswordPath(workDirectoryPath, tenant)))
	return ContainerTenantStatus{
		TenantID:              tenant.TenantID,
		RuntimeID:             tenant.RuntimeID,
		ContainerName:         tenant.ContainerName,
		ContainerState:        containerState,
		MattermostTeamName:    tenant.TeamName,
		MattermostTeamURL:     "http://localhost:8065/" + tenant.TeamName,
		CompanyAdminEmail:     companyAdminEmail,
		CompanyAdminPassword:  companyAdminPassword,
		MattermostTeamPresent: mattermostTeamPresent,
		DatabaseName:          tenant.DatabaseName,
		DatabasePresent:       databasePresent,
		ConfigPath:            containerTenantConfigPath(workDirectoryPath, tenant),
		SecretPath:            containerTenantSecretPath(workDirectoryPath, tenant),
	}, nil
}

func readContainerFileIfPresent(path string) string {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return ""
	}
	return string(document)
}

func (runtime ContainerRuntime) tenantContainerState(workDirectoryPath string, tenant ContainerTenant) string {
	if !fileExists(containerTenantsComposePath(workDirectoryPath)) {
		return "missing-compose"
	}
	output, errorValue := runtime.executor().CombinedOutput(runtime.dockerComposeInvocation(containerTenantsComposePath(workDirectoryPath), []string{"ps", tenant.RuntimeID, "--format", "json"}))
	if errorValue != nil {
		return "unknown"
	}
	return parseDockerComposeState(output)
}

func (runtime ContainerRuntime) mattermostTeamExists(workDirectoryPath string, tenant ContainerTenant) bool {
	output, errorValue := runtime.mattermostOutput(workDirectoryPath, []string{"team", "search", tenant.TeamName, "--json"})
	if errorValue != nil {
		return false
	}
	return bytes.Contains(output, []byte(`"name":"`+tenant.TeamName+`"`)) || bytes.Contains(output, []byte(`"Name":"`+tenant.TeamName+`"`))
}

func (runtime ContainerRuntime) tenantDatabaseExists(workDirectoryPath string, tenant ContainerTenant) (bool, error) {
	output, errorValue := runtime.executor().CombinedOutput(ContainerCommandInvocation{
		ExecutableName: "docker",
		Arguments: []string{
			"compose", "-f", containerInfraComposePath(workDirectoryPath),
			"exec", "-T", "postgres",
			"psql", "-U", "internkim", "-d", "postgres", "-tAc",
			"SELECT 1 FROM pg_database WHERE datname = '" + tenant.DatabaseName + "'",
		},
	})
	if errorValue != nil {
		return false, errorValue
	}
	return strings.TrimSpace(string(output)) == "1", nil
}

func parseDockerComposeState(output []byte) string {
	trimmedOutput := strings.TrimSpace(string(output))
	if trimmedOutput == "" || trimmedOutput == "[]" {
		return "absent"
	}
	var arrayStatus []struct {
		State  string `json:"State"`
		Status string `json:"Status"`
	}
	if json.Unmarshal(output, &arrayStatus) == nil && len(arrayStatus) > 0 {
		return firstNonEmptyContainerString(arrayStatus[0].State, arrayStatus[0].Status, "unknown")
	}
	var objectStatus struct {
		State  string `json:"State"`
		Status string `json:"Status"`
	}
	if json.Unmarshal(output, &objectStatus) == nil {
		return firstNonEmptyContainerString(objectStatus.State, objectStatus.Status, "unknown")
	}
	return trimmedOutput
}

func (runtime ContainerRuntime) runMattermostCommand(workDirectoryPath string, arguments []string) error {
	return runtime.executor().Run(runtime.mattermostInvocation(workDirectoryPath, arguments))
}

func firstNonEmptyContainerString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (runtime ContainerRuntime) mattermostOutput(workDirectoryPath string, arguments []string) ([]byte, error) {
	return runtime.executor().CombinedOutput(runtime.mattermostInvocation(workDirectoryPath, arguments))
}

func (runtime ContainerRuntime) mattermostInvocation(workDirectoryPath string, arguments []string) ContainerCommandInvocation {
	commandArguments := []string{
		"compose", "-f", containerInfraComposePath(workDirectoryPath),
		"exec", "-T", "mattermost",
		"mmctl", "--local",
	}
	commandArguments = append(commandArguments, arguments...)
	return ContainerCommandInvocation{ExecutableName: "docker", Arguments: commandArguments, Stdout: runtime.output(), Stderr: runtime.errorOutput()}
}

func (runtime ContainerRuntime) dockerComposeInvocation(composePath string, arguments []string) ContainerCommandInvocation {
	commandArguments := []string{"compose", "-f", composePath}
	commandArguments = append(commandArguments, arguments...)
	return ContainerCommandInvocation{ExecutableName: "docker", Arguments: commandArguments, Stdout: runtime.output(), Stderr: runtime.errorOutput()}
}

func (runtime ContainerRuntime) executor() ContainerCommandExecutor {
	if runtime.CommandExecutor != nil {
		return runtime.CommandExecutor
	}
	return OperatingSystemContainerCommandExecutor{}
}

func (runtime ContainerRuntime) output() io.Writer {
	if runtime.Output != nil {
		return runtime.Output
	}
	return io.Discard
}

func (runtime ContainerRuntime) errorOutput() io.Writer {
	if runtime.ErrorOutput != nil {
		return runtime.ErrorOutput
	}
	return io.Discard
}

func containerCommandEnvironment(environment []string) []string {
	if len(environment) == 0 {
		return os.Environ()
	}
	return append(os.Environ(), environment...)
}

func containerInfraComposePath(workDirectoryPath string) string {
	return filepath.Join(workDirectoryPath, "infra", "docker-compose.yml")
}

func containerTenantsComposePath(workDirectoryPath string) string {
	return filepath.Join(workDirectoryPath, "tenants.generated.yml")
}

func containerTenantConfigPath(workDirectoryPath string, tenant ContainerTenant) string {
	return filepath.Join(workDirectoryPath, "config", tenant.RuntimeID)
}

func containerTenantWorkspacePath(workDirectoryPath string, tenant ContainerTenant) string {
	return filepath.Join(workDirectoryPath, "workspace", tenant.RuntimeID)
}

func containerTenantSecretPath(workDirectoryPath string, tenant ContainerTenant) string {
	return filepath.Join(workDirectoryPath, "secrets", tenant.RuntimeID)
}

func tenantDatabasePasswordPath(workDirectoryPath string, tenant ContainerTenant) string {
	return filepath.Join(containerTenantSecretPath(workDirectoryPath, tenant), "db-password")
}

func tenantCompanyAdminPasswordPath(workDirectoryPath string, tenant ContainerTenant) string {
	return filepath.Join(containerTenantSecretPath(workDirectoryPath, tenant), "company-admin-password")
}

func legacyTenantCompanyAdminPasswordPath(workDirectoryPath string, tenant ContainerTenant) string {
	return filepath.Join(workDirectoryPath, "secrets", "company-admins", tenant.RuntimeID+"-password")
}

func containerComposeMountPath(workDirectoryPath string, mountedPath string) string {
	if strings.TrimSpace(mountedPath) == "" {
		return "./secrets/openrouter-key"
	}
	absoluteWorkDirectoryPath, errorValue := filepath.Abs(workDirectoryPath)
	if errorValue != nil {
		return filepath.Clean(mountedPath)
	}
	absoluteMountedPath, errorValue := filepath.Abs(mountedPath)
	if errorValue != nil {
		return filepath.Clean(mountedPath)
	}
	relativePath, errorValue := filepath.Rel(absoluteWorkDirectoryPath, absoluteMountedPath)
	if errorValue == nil && relativePath != "." && !strings.HasPrefix(relativePath, "..") {
		return "./" + filepath.ToSlash(relativePath)
	}
	return filepath.Clean(mountedPath)
}

func removeContainerPath(path string) bool {
	if !directoryExists(path) && !fileExists(path) {
		return false
	}
	return os.RemoveAll(path) == nil
}

func removeContainerGeneratedState(workDirectoryPath string) []string {
	removedPaths := []string{}
	for _, path := range []string{
		filepath.Join(workDirectoryPath, "config"),
		filepath.Join(workDirectoryPath, "infra"),
		containerTenantsComposePath(workDirectoryPath),
	} {
		if removeContainerPath(path) {
			removedPaths = append(removedPaths, path)
		}
	}
	tenantSecretDirectoryPath := filepath.Join(workDirectoryPath, "secrets")
	entries, errorValue := os.ReadDir(tenantSecretDirectoryPath)
	if errorValue != nil {
		return removedPaths
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "tenant_") {
			continue
		}
		path := filepath.Join(tenantSecretDirectoryPath, entry.Name())
		if removeContainerPath(path) {
			removedPaths = append(removedPaths, path)
		}
	}
	return removedPaths
}

func containerInfraComposeDocument(superuserPassword string, mattermostPublicURL string) string {
	return strings.Join([]string{
		"name: internkim-poc-infra",
		"",
		"networks:",
		"  internkim-poc:",
		"    name: internkim-poc",
		"",
		"volumes:",
		"  postgres-data:",
		"  mattermost-data:",
		"",
		"services:",
		"  postgres:",
		"    image: postgres:16-alpine",
		"    networks: [internkim-poc]",
		"    environment:",
		"      POSTGRES_USER: internkim",
		"      POSTGRES_PASSWORD: " + superuserPassword,
		"      POSTGRES_DB: internkim",
		"      TENANT_COUNT: ${TENANT_COUNT:-10}",
		"    volumes:",
		"      - postgres-data:/var/lib/postgresql/data",
		"      - ./postgres-init:/docker-entrypoint-initdb.d:ro",
		"    healthcheck:",
		`      test: ["CMD-SHELL", "pg_isready -U internkim"]`,
		"      interval: 5s",
		"      timeout: 5s",
		"      retries: 20",
		"    ports:",
		`      - "55432:5432"`,
		"",
		"  mattermost:",
		"    image: mattermost/mattermost-team-edition:10.5",
		"    networks: [internkim-poc]",
		"    depends_on:",
		"      postgres:",
		"        condition: service_healthy",
		"    environment:",
		"      MM_SQLSETTINGS_DRIVERNAME: postgres",
		`      MM_SQLSETTINGS_DATASOURCE: "postgres://internkim:` + superuserPassword + `@postgres:5432/mattermost?sslmode=disable&connect_timeout=10"`,
		`      MM_SERVICESETTINGS_SITEURL: "` + strings.TrimRight(strings.TrimSpace(mattermostPublicURL), "/") + `"`,
		`      MM_SERVICESETTINGS_ENABLEBOTACCOUNTCREATION: "true"`,
		`      MM_SERVICESETTINGS_ENABLEUSERACCESSTOKENS: "true"`,
		`      MM_SERVICESETTINGS_ENABLELOCALMODE: "true"`,
		`      MM_RATELIMITSETTINGS_ENABLE: "false"`,
		`      MM_TEAMSETTINGS_MAXUSERSPERTEAM: "200"`,
		`      MM_TEAMSETTINGS_ENABLEOPENSERVER: "false"`,
		`      MM_TEAMSETTINGS_RESTRICTDIRECTMESSAGE: "team"`,
		`      MM_PLUGINSETTINGS_ENABLE: "true"`,
		`      MM_PLUGINSETTINGS_ENABLEUPLOADS: "true"`,
		`      MM_LOGSETTINGS_CONSOLELEVEL: "ERROR"`,
		"    volumes:",
		"      - mattermost-data:/mattermost/data",
		"    healthcheck:",
		`      test: ["CMD", "curl", "-fsS", "http://localhost:8065/api/v4/system/ping"]`,
		"      interval: 10s",
		"      timeout: 5s",
		"      retries: 30",
		"    ports:",
		`      - "8065:8065"`,
		"",
	}, "\n")
}

func containerPostgresInitDocument() string {
	return strings.Join([]string{
		"#!/bin/bash",
		"set -euo pipefail",
		"",
		"createDatabase() {",
		`  local databaseName="$1"`,
		`  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-SQL`,
		`	SELECT 'CREATE DATABASE ${databaseName}'`,
		`	WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${databaseName}')\gexec`,
		"	SQL",
		"}",
		"",
		"createDatabase mattermost",
		"",
		`tenantCount="${TENANT_COUNT:-10}"`,
		`for number in $(seq 1 "${tenantCount}"); do`,
		`  index="$(printf '%02d' "${number}")"`,
		`  createDatabase "tenant_${index}"`,
		"done",
		"",
	}, "\n")
}
