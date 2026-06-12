package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/tenantruntime"
)

func runTenant() {
	if len(os.Args) < 3 || containsArg("--help") || containsArg("-h") {
		printTenantUsage()
		return
	}
	switch os.Args[2] {
	case "create":
		runTenantCreate(os.Args[3:])
	case "provision":
		runTenantProvision(os.Args[3:])
	case "create-fleet":
		runTenantCreateFleet(os.Args[3:])
	case "status":
		runTenantStatus(os.Args[3:])
	case "backup":
		runTenantBackup(os.Args[3:])
	case "restore":
		runTenantRestore(os.Args[3:])
	case "install-container":
		runTenantInstallContainer(os.Args[3:])
	case "install-host-runtime":
		runTenantInstallHostRuntime(os.Args[3:])
	case "bootstrap":
		runTenantBootstrap(os.Args[3:])
	case "bootstrap-mattermost":
		runTenantBootstrapMattermost(os.Args[3:])
	case "expose-mattermost":
		runTenantExposeMattermost(os.Args[3:])
	case "sync-cloudflare-tunnel":
		runTenantSyncCloudflareTunnel(os.Args[3:])
	case "start":
		runTenantStart(os.Args[3:])
	case "stop":
		runTenantStop(os.Args[3:])
	default:
		printTenantUsage()
	}
}

func printTenantUsage() {
	fmt.Println("Usage: internkim tenant <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  create   Create a tenant manifest and runtime directories")
	fmt.Println("  provision  Create, install, bootstrap, start, configure Mattermost members, and sync tunnel for one tenant")
	fmt.Println("  create-fleet  Create many cloud-shared tenants and print Mattermost URLs")
	fmt.Println("  status   Read tenant manifest and runtime directory status")
	fmt.Println("  backup   Create an encrypted tenant backup")
	fmt.Println("  restore  Restore an encrypted tenant backup")
	fmt.Println("  install-container  Write the systemd-nspawn container configuration")
	fmt.Println("  install-host-runtime  Write tenant-scoped host runtime services")
	fmt.Println("  bootstrap  Install tenant rootfs services and gateway token")
	fmt.Println("  bootstrap-mattermost  Ensure tenant Mattermost team, bot, default channels, and optional members")
	fmt.Println("  expose-mattermost  Start Cloudflare quick tunnels for tenant Mattermost instances")
	fmt.Println("  sync-cloudflare-tunnel  Route tenant app paths and Mattermost through a Cloudflare named tunnel")
	fmt.Println("  start    Start the tenant systemd-nspawn container")
	fmt.Println("  stop     Stop the tenant systemd-nspawn container")
}

func runTenantCreate(arguments []string) {
	flags := flag.NewFlagSet("tenant create", flag.ExitOnError)
	tenantID := flags.String("tenant", "", "tenant id")
	displayName := flags.String("display-name", "", "tenant display name")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	assignedHost := flags.String("assigned-host", "", "assigned host name")
	publicURL := flags.String("public-url", "", "tenant public URL")
	mirrorHost := flags.String("mirror-host", "", "backup mirror host")
	profile := flags.String("profile", tenantruntime.ProfileCloudShared, "tenant profile")
	deviceID := flags.String("device-id", "", "edge appliance device id")
	templateRootFilesystemPath := flags.String("template-rootfs", "", "root filesystem template path")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	manifest, errorValue := tenantManifestFromFlags(*profile, *tenantID, *displayName, *assignedHost, *publicURL, *mirrorHost, *deviceID)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	status, errorValue := tenantruntime.Service{BasePath: *basePath}.CreateTenantFromTemplate(manifest, *templateRootFilesystemPath)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	printTenantStatus(status)
}

func runTenantProvision(arguments []string) {
	options, errorValue := parseTenantProvisionOptions(arguments)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	summary, errorValue := executeTenantProvision(context.Background(), options)
	if errorValue != nil {
		var stepError tenantProvisionStepError
		if errors.As(errorValue, &stepError) {
			fmt.Fprintf(os.Stderr, "tenant provision failed at step %s: %v\n", stepError.Step, stepError.Cause)
			fmt.Fprintf(os.Stderr, "resume with: %s\n", tenantProvisionResumeCommand(stepError.Step, options.TenantID))
			os.Exit(1)
		}
		fatal(errorValue.Error())
	}
	document, errorValue := json.MarshalIndent(summary, "", "  ")
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	fmt.Println(string(document))
}

type tenantProvisionOptions struct {
	TenantID                         string
	DisplayName                      string
	BasePath                         string
	AssignedHost                     string
	PublicURL                        string
	MirrorHost                       string
	Profile                          string
	DeviceID                         string
	TemplateRootFilesystemPath       string
	NspawnDirectoryPath              string
	BinaryDirectoryPath              string
	GatewayURL                       string
	DeviceToken                      string
	GatewaySharedSecret              string
	ReleaseDownloadToken             string
	AdminPassword                    string
	AdminEmail                       string
	ModelName                        string
	MattermostBaseURLTemplate        string
	BlueclawURLTemplate              string
	MattermostPublicURLTemplate      string
	MattermostPortStart              int
	Language                         string
	TokenOutputRoot                  string
	CloudflareAccountID              string
	CloudflareTunnelID               string
	CloudflareAPITokenPath           string
	CloudflareAPIBaseURL             string
	CloudflarePublicHostnameTemplate string
	Members                          []tenantProvisionMember
}

type tenantProvisionMember struct {
	Email               string
	Name                string
	Password            string
	IsPasswordGenerated bool
}

type tenantProvisionSummary struct {
	TenantID         string                         `json:"tenantID"`
	PublicURL        string                         `json:"publicURL"`
	MattermostURL    string                         `json:"mattermostURL"`
	AdminCredentials tenantProvisionAdminCredential `json:"adminCredentials"`
	Members          []tenantProvisionMemberResult  `json:"members"`
}

type tenantProvisionAdminCredential struct {
	Username     string `json:"username"`
	Email        string `json:"email"`
	PasswordPath string `json:"passwordPath"`
}

type tenantProvisionMemberResult struct {
	Email               string `json:"email"`
	Name                string `json:"name"`
	Username            string `json:"username,omitempty"`
	UserID              string `json:"userID,omitempty"`
	Outcome             string `json:"outcome"`
	Reason              string `json:"reason,omitempty"`
	BlueclawInvited     bool   `json:"blueclawInvited,omitempty"`
	Password            string `json:"password,omitempty"`
	IsPasswordGenerated bool   `json:"passwordGenerated,omitempty"`
}

type tenantProvisionStepError struct {
	Step  string
	Cause error
}

func (errorValue tenantProvisionStepError) Error() string {
	return errorValue.Step + ": " + errorValue.Cause.Error()
}

func (errorValue tenantProvisionStepError) Unwrap() error {
	return errorValue.Cause
}

func parseTenantProvisionOptions(arguments []string) (tenantProvisionOptions, error) {
	flags := flag.NewFlagSet("tenant provision", flag.ContinueOnError)
	tenantID := flags.String("tenant", "", "tenant id")
	displayName := flags.String("display-name", "", "tenant display name")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	assignedHost := flags.String("assigned-host", "", "assigned host name")
	publicURL := flags.String("public-url", "", "tenant public URL")
	mirrorHost := flags.String("mirror-host", "", "backup mirror host")
	profile := flags.String("profile", tenantruntime.ProfileCloudShared, "tenant profile")
	deviceID := flags.String("device-id", "", "edge appliance device id")
	templateRootFilesystemPath := flags.String("template-rootfs", "", "root filesystem template path")
	nspawnDirectoryPath := flags.String("nspawn-dir", "/etc/systemd/nspawn", "systemd-nspawn configuration directory")
	binaryDirectoryPath := flags.String("bin-dir", "/usr/local/bin", "directory containing tenant service binaries")
	gatewayURL := flags.String("gateway-url", "", "LLM gateway OpenRouter-compatible chat completion URL")
	deviceToken := flags.String("device-token", "", "tenant device-scoped LLM token")
	gatewaySharedSecret := flags.String("gateway-shared-secret", "", "tenant LLM gateway shared secret")
	releaseDownloadToken := flags.String("release-download-token", "", "release registry download token")
	adminPassword := flags.String("admin-password", "", "tenant initial admin password")
	adminEmail := flags.String("admin-email", "", "tenant admin email")
	modelName := flags.String("model", "", "optional remote model override")
	mattermostBaseURLTemplate := flags.String("base-url-template", "http://127.0.0.1:{port}", "Mattermost base URL template containing {tenant} or {port}")
	blueclawURLTemplate := flags.String("blueclaw-url-template", "http://127.0.0.1:{blueclawPort}", "Blueclaw URL template containing {tenant} or {blueclawPort}; empty disables policy invite")
	mattermostPublicURLTemplate := flags.String("public-url-template", "", "tenant public URL template containing {tenant}")
	mattermostPortStart := flags.Int("port-start", 18065, "first tenant Mattermost port")
	language := flags.String("language", "ko", "workspace language")
	tokenOutputRoot := flags.String("token-output-root", "", "optional tenant root for writing Mattermost bot tokens")
	cloudflareAccountID := flags.String("account-id", "", "Cloudflare account id")
	cloudflareTunnelID := flags.String("tunnel-id", "", "Cloudflare tunnel id")
	cloudflareAPITokenPath := flags.String("api-token-path", "", "Cloudflare API token file path")
	cloudflareAPIBaseURL := flags.String("api-base-url", tenantruntime.DefaultCloudflareAPIBaseURL, "Cloudflare API base URL")
	cloudflarePublicHostnameTemplate := flags.String("hostname-template", "", "optional public hostname template containing {tenant}")
	memberValues := repeatedStringFlag{}
	flags.Var(&memberValues, "member", "tenant member as email:name:password; password may be omitted")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return tenantProvisionOptions{}, errorValue
	}
	members, errorValue := parseTenantProvisionMembers(memberValues.Values())
	if errorValue != nil {
		return tenantProvisionOptions{}, errorValue
	}
	return tenantProvisionOptions{
		TenantID:                         strings.TrimSpace(*tenantID),
		DisplayName:                      strings.TrimSpace(*displayName),
		BasePath:                         strings.TrimSpace(*basePath),
		AssignedHost:                     strings.TrimSpace(*assignedHost),
		PublicURL:                        strings.TrimSpace(*publicURL),
		MirrorHost:                       strings.TrimSpace(*mirrorHost),
		Profile:                          strings.TrimSpace(*profile),
		DeviceID:                         strings.TrimSpace(*deviceID),
		TemplateRootFilesystemPath:       strings.TrimSpace(*templateRootFilesystemPath),
		NspawnDirectoryPath:              strings.TrimSpace(*nspawnDirectoryPath),
		BinaryDirectoryPath:              strings.TrimSpace(*binaryDirectoryPath),
		GatewayURL:                       strings.TrimSpace(*gatewayURL),
		DeviceToken:                      strings.TrimSpace(*deviceToken),
		GatewaySharedSecret:              strings.TrimSpace(*gatewaySharedSecret),
		ReleaseDownloadToken:             strings.TrimSpace(*releaseDownloadToken),
		AdminPassword:                    *adminPassword,
		AdminEmail:                       strings.TrimSpace(*adminEmail),
		ModelName:                        strings.TrimSpace(*modelName),
		MattermostBaseURLTemplate:        strings.TrimSpace(*mattermostBaseURLTemplate),
		BlueclawURLTemplate:              strings.TrimSpace(*blueclawURLTemplate),
		MattermostPublicURLTemplate:      strings.TrimSpace(*mattermostPublicURLTemplate),
		MattermostPortStart:              *mattermostPortStart,
		Language:                         strings.TrimSpace(*language),
		TokenOutputRoot:                  strings.TrimSpace(*tokenOutputRoot),
		CloudflareAccountID:              strings.TrimSpace(*cloudflareAccountID),
		CloudflareTunnelID:               strings.TrimSpace(*cloudflareTunnelID),
		CloudflareAPITokenPath:           strings.TrimSpace(*cloudflareAPITokenPath),
		CloudflareAPIBaseURL:             strings.TrimSpace(*cloudflareAPIBaseURL),
		CloudflarePublicHostnameTemplate: strings.TrimSpace(*cloudflarePublicHostnameTemplate),
		Members:                          members,
	}, nil
}

func parseTenantProvisionMembers(values []string) ([]tenantProvisionMember, error) {
	members := make([]tenantProvisionMember, 0, len(values))
	for _, value := range values {
		member, errorValue := parseTenantProvisionMember(value)
		if errorValue != nil {
			return nil, errorValue
		}
		members = append(members, member)
	}
	return members, nil
}

func parseTenantProvisionMember(value string) (tenantProvisionMember, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return tenantProvisionMember{}, errors.New("--member must use email:name:password")
	}
	email := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	password := ""
	if len(parts) == 3 {
		password = strings.TrimSpace(parts[2])
	}
	if email == "" || name == "" {
		return tenantProvisionMember{}, errors.New("--member requires email and name")
	}
	address, errorValue := mail.ParseAddress(email)
	if errorValue != nil || address.Address != email {
		return tenantProvisionMember{}, errors.New("--member email is invalid")
	}
	isPasswordGenerated := password == ""
	if isPasswordGenerated {
		credentials, errorValue := tenantruntime.GenerateTenantCredentials()
		if errorValue != nil {
			return tenantProvisionMember{}, errorValue
		}
		password = credentials.AdminPassword
	}
	return tenantProvisionMember{
		Email:               email,
		Name:                name,
		Password:            password,
		IsPasswordGenerated: isPasswordGenerated,
	}, nil
}

func executeTenantProvision(ctx context.Context, options tenantProvisionOptions) (tenantProvisionSummary, error) {
	manifest, errorValue := tenantProvisionManifest(options)
	if errorValue != nil {
		return tenantProvisionSummary{}, tenantProvisionStepError{Step: "create", Cause: errorValue}
	}
	createService := tenantruntime.Service{BasePath: options.BasePath}
	if _, errorValue := createService.CreateTenantFromTemplate(manifest, options.TemplateRootFilesystemPath); errorValue != nil {
		return tenantProvisionSummary{}, tenantProvisionStepError{Step: "create", Cause: errorValue}
	}
	containerService := tenantruntime.Service{BasePath: options.BasePath, SystemdNspawnDirectoryPath: options.NspawnDirectoryPath}
	if _, errorValue := containerService.InstallTenantContainer(manifest.TenantID); errorValue != nil {
		return tenantProvisionSummary{}, tenantProvisionStepError{Step: "install-container", Cause: errorValue}
	}
	bootstrapOptions := tenantruntime.BootstrapOptions{
		BinaryDirectoryPath:  options.BinaryDirectoryPath,
		GatewayURL:           options.GatewayURL,
		DeviceToken:          options.DeviceToken,
		GatewaySharedSecret:  options.GatewaySharedSecret,
		ReleaseDownloadToken: resolveReleaseDownloadToken(options.ReleaseDownloadToken),
		AdminPassword:        options.AdminPassword,
		AdminEmail:           options.AdminEmail,
		ModelName:            options.ModelName,
	}
	if _, errorValue := createService.BootstrapTenant(manifest.TenantID, bootstrapOptions); errorValue != nil {
		return tenantProvisionSummary{}, tenantProvisionStepError{Step: "bootstrap", Cause: errorValue}
	}
	if errorValue := containerService.StartTenant(ctx, manifest.TenantID); errorValue != nil {
		return tenantProvisionSummary{}, tenantProvisionStepError{Step: "start", Cause: errorValue}
	}
	adminPassword, adminPasswordPath, errorValue := tenantProvisionAdminPassword(options)
	if errorValue != nil {
		return tenantProvisionSummary{}, tenantProvisionStepError{Step: "bootstrap-mattermost", Cause: errorValue}
	}
	mattermostStatus, errorValue := tenantruntime.BootstrapMattermostTenantResources(manifest.TenantID, tenantruntime.TenantInitialAdminUsername, adminPassword, tenantruntime.MattermostFleetBootstrapOptions{
		BaseURLTemplate:     options.MattermostBaseURLTemplate,
		BlueclawURLTemplate: options.BlueclawURLTemplate,
		PublicURLTemplate:   tenantProvisionMattermostPublicURL(manifest, options),
		PortStart:           options.MattermostPortStart,
		Language:            options.Language,
		TokenOutputRoot:     options.TokenOutputRoot,
		AdminEmail:          options.AdminEmail,
		Members:             tenantProvisionMattermostMembers(options.Members),
	})
	if errorValue != nil {
		return tenantProvisionSummary{}, tenantProvisionStepError{Step: "bootstrap-mattermost", Cause: errorValue}
	}
	if _, errorValue := createService.SyncCloudflareTenantTunnel(ctx, tenantruntime.CloudflareTunnelSyncOptions{
		AccountID:              options.CloudflareAccountID,
		TunnelID:               options.CloudflareTunnelID,
		APITokenPath:           options.CloudflareAPITokenPath,
		APIBaseURL:             options.CloudflareAPIBaseURL,
		PublicHostnameTemplate: options.CloudflarePublicHostnameTemplate,
		TenantIDs:              []string{manifest.TenantID},
	}); errorValue != nil {
		return tenantProvisionSummary{}, tenantProvisionStepError{Step: "sync-cloudflare-tunnel", Cause: errorValue}
	}
	return tenantProvisionSummary{
		TenantID:      manifest.TenantID,
		PublicURL:     firstNonEmptyString(manifest.PublicURL, mattermostStatus.PublicURL),
		MattermostURL: firstNonEmptyString(manifest.MattermostInstance.PublicURL, mattermostStatus.PublicURL),
		AdminCredentials: tenantProvisionAdminCredential{
			Username:     tenantruntime.TenantInitialAdminUsername,
			Email:        firstNonEmptyString(options.AdminEmail, tenantruntime.DefaultTenantAdminEmail(manifest.TenantID)),
			PasswordPath: adminPasswordPath,
		},
		Members: tenantProvisionMemberResults(options.Members, mattermostStatus.Members),
	}, nil
}

func tenantProvisionManifest(options tenantProvisionOptions) (tenantruntime.Manifest, error) {
	manifest, errorValue := tenantManifestFromFlags(options.Profile, options.TenantID, options.DisplayName, options.AssignedHost, options.PublicURL, options.MirrorHost, options.DeviceID)
	if errorValue != nil {
		return tenantruntime.Manifest{}, errorValue
	}
	if manifest.Profile != tenantruntime.ProfileCloudShared {
		return manifest, nil
	}
	instance, errorValue := tenantProvisionMattermostInstance(manifest.TenantID, options)
	if errorValue != nil {
		return tenantruntime.Manifest{}, errorValue
	}
	manifest.MattermostInstance = instance
	if strings.TrimSpace(manifest.PublicURL) == "" {
		manifest.PublicURL = instance.PublicURL
	}
	return manifest, nil
}

func tenantProvisionMattermostInstance(tenantID string, options tenantProvisionOptions) (tenantruntime.MattermostInstance, error) {
	port, errorValue := tenantProvisionMattermostPort(tenantID, options.MattermostPortStart)
	if errorValue != nil {
		return tenantruntime.MattermostInstance{}, errorValue
	}
	publicURL := strings.TrimSpace(options.PublicURL)
	if strings.TrimSpace(options.MattermostPublicURLTemplate) != "" {
		publicURL = tenantProvisionExpandTemplate(options.MattermostPublicURLTemplate, tenantID, port)
	}
	return tenantruntime.MattermostInstance{
		PublicURL:    publicURL,
		InternalURL:  "http://127.0.0.1:" + strconv.Itoa(port),
		Port:         port,
		DatabaseName: tenantProvisionMattermostDatabaseName(tenantID),
	}, nil
}

func tenantProvisionMattermostPort(tenantID string, portStart int) (int, error) {
	if portStart <= 0 {
		return 0, errors.New("mattermost port start is required")
	}
	parts := strings.Split(strings.TrimSpace(tenantID), "-")
	index, errorValue := strconv.Atoi(parts[len(parts)-1])
	if errorValue != nil || index <= 0 {
		return 0, errors.New("tenant id must end with a numeric index")
	}
	return portStart + index - 1, nil
}

func tenantProvisionExpandTemplate(template string, tenantID string, port int) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(template), "{tenant}", tenantID), "{port}", strconv.Itoa(port))
}

func tenantProvisionMattermostDatabaseName(tenantID string) string {
	return "mattermost_" + strings.NewReplacer("-", "_", ".", "_").Replace(strings.TrimSpace(tenantID))
}

func tenantProvisionMattermostPublicURL(manifest tenantruntime.Manifest, options tenantProvisionOptions) string {
	return firstNonEmptyString(options.MattermostPublicURLTemplate, manifest.MattermostInstance.PublicURL, manifest.PublicURL)
}

func tenantProvisionAdminPassword(options tenantProvisionOptions) (string, string, error) {
	paths, errorValue := tenantruntime.BuildRuntimePaths(options.BasePath, options.TenantID)
	if errorValue != nil {
		return "", "", errorValue
	}
	passwordPath := filepath.Join(paths.InternKimSecretsPath, "mm-admin-pass")
	if strings.TrimSpace(options.AdminPassword) != "" {
		return options.AdminPassword, passwordPath, nil
	}
	document, errorValue := os.ReadFile(passwordPath)
	if errorValue != nil {
		return "", "", errorValue
	}
	password := strings.TrimSpace(string(document))
	if password == "" {
		return "", "", errors.New("tenant admin password file is empty")
	}
	return password, passwordPath, nil
}

func tenantProvisionMattermostMembers(members []tenantProvisionMember) []tenantruntime.MattermostBootstrapMember {
	mattermostMembers := make([]tenantruntime.MattermostBootstrapMember, 0, len(members))
	for _, member := range members {
		mattermostMembers = append(mattermostMembers, tenantruntime.MattermostBootstrapMember{
			Email:    member.Email,
			Name:     member.Name,
			Password: member.Password,
		})
	}
	return mattermostMembers
}

func tenantProvisionMemberResults(members []tenantProvisionMember, results []tenantruntime.MattermostBootstrapMemberResult) []tenantProvisionMemberResult {
	passwords := tenantProvisionMemberPasswords(members)
	provisionResults := make([]tenantProvisionMemberResult, 0, len(results))
	for _, result := range results {
		password := passwords[result.Email]
		provisionResults = append(provisionResults, tenantProvisionMemberResult{
			Email:               result.Email,
			Name:                result.Name,
			Username:            result.Username,
			UserID:              result.UserID,
			Outcome:             result.Outcome,
			Reason:              result.Reason,
			BlueclawInvited:     result.BlueclawInvited,
			Password:            password.password,
			IsPasswordGenerated: password.isGenerated,
		})
	}
	return provisionResults
}

type tenantProvisionMemberPassword struct {
	password    string
	isGenerated bool
}

func tenantProvisionMemberPasswords(members []tenantProvisionMember) map[string]tenantProvisionMemberPassword {
	passwords := map[string]tenantProvisionMemberPassword{}
	for _, member := range members {
		if member.IsPasswordGenerated {
			passwords[member.Email] = tenantProvisionMemberPassword{password: member.Password, isGenerated: true}
		}
	}
	return passwords
}

func tenantProvisionResumeCommand(step string, tenantID string) string {
	commands := []string{
		"internkim tenant create --tenant " + tenantID,
		"internkim tenant install-container --tenant " + tenantID,
		"internkim tenant bootstrap --tenant " + tenantID,
		"internkim tenant start --tenant " + tenantID,
		"internkim tenant bootstrap-mattermost --credentials <credentials.json> <member flags>",
		"internkim tenant sync-cloudflare-tunnel --tenants " + tenantID,
	}
	for index, commandStep := range []string{"create", "install-container", "bootstrap", "start", "bootstrap-mattermost", "sync-cloudflare-tunnel"} {
		if commandStep == step {
			return strings.Join(commands[index:], " && ")
		}
	}
	return strings.Join(commands, " && ")
}

func runTenantCreateFleet(arguments []string) {
	flags := flag.NewFlagSet("tenant create-fleet", flag.ExitOnError)
	count := flags.Int("count", 10, "tenant count")
	startIndex := flags.Int("start-index", 1, "first tenant numeric index")
	tenantPrefix := flags.String("tenant-prefix", "poc", "tenant id prefix")
	displayNamePrefix := flags.String("display-name-prefix", "PoC", "tenant display name prefix")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	assignedHost := flags.String("assigned-host", "", "assigned host name")
	publicURLTemplate := flags.String("public-url-template", "", "tenant public URL template containing {tenant}")
	mirrorHost := flags.String("mirror-host", "", "backup mirror host")
	gatewayTokensPath := flags.String("gateway-tokens", "", "optional gateway device token JSON path")
	hardLimitMicrounits := flags.Int64("hard-limit-microunits", 0, "tenant LLM hard limit in microunits")
	requestsPerMinute := flags.Int("requests-per-minute", 30, "tenant LLM request limit per minute")
	mattermostPortStart := flags.Int("mattermost-port-start", 18065, "first tenant Mattermost port")
	openRouterManagementKeyPath := flags.String("openrouter-management-key", "", "OpenRouter management key file path for tenant provider key provisioning")
	openRouterAPIKeysURL := flags.String("openrouter-keys-url", tenantruntime.DefaultOpenRouterAPIKeysURL, "OpenRouter API keys management endpoint")
	openRouterKeyLimitUSD := flags.Float64("openrouter-key-limit-usd", 0, "upstream OpenRouter key spend limit in USD")
	openRouterKeyLimitReset := flags.String("openrouter-key-limit-reset", "", "upstream OpenRouter key limit reset: daily, weekly, monthly, or empty")
	openRouterKeyExpiresAt := flags.String("openrouter-key-expires-at", "", "upstream OpenRouter key expiration timestamp")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	statuses, errorValue := (tenantruntime.Service{BasePath: *basePath}).CreateCloudSharedFleet(tenantruntime.FleetCreateOptions{
		Count:                       *count,
		StartIndex:                  *startIndex,
		TenantPrefix:                *tenantPrefix,
		DisplayNamePrefix:           *displayNamePrefix,
		AssignedHost:                *assignedHost,
		PublicURLTemplate:           *publicURLTemplate,
		MirrorHost:                  *mirrorHost,
		GatewayTokensPath:           *gatewayTokensPath,
		HardLimitMicrounits:         *hardLimitMicrounits,
		RequestsPerMinute:           *requestsPerMinute,
		MattermostPortStart:         *mattermostPortStart,
		OpenRouterManagementKeyPath: *openRouterManagementKeyPath,
		OpenRouterAPIKeysURL:        *openRouterAPIKeysURL,
		OpenRouterKeyLimitUSD:       *openRouterKeyLimitUSD,
		OpenRouterKeyLimitReset:     *openRouterKeyLimitReset,
		OpenRouterKeyExpiresAt:      *openRouterKeyExpiresAt,
	})
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	printTenantFleetStatus(statuses)
}

func runTenantStatus(arguments []string) {
	flags := flag.NewFlagSet("tenant status", flag.ExitOnError)
	tenantID := flags.String("tenant", "", "tenant id")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	status, errorValue := tenantruntime.Service{BasePath: *basePath}.Status(*tenantID)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	printTenantStatus(status)
}

func runTenantInstallContainer(arguments []string) {
	flags := flag.NewFlagSet("tenant install-container", flag.ExitOnError)
	tenantID := flags.String("tenant", "", "tenant id")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	nspawnDirectoryPath := flags.String("nspawn-dir", "/etc/systemd/nspawn", "systemd-nspawn configuration directory")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	status, errorValue := (tenantruntime.Service{
		BasePath:                   *basePath,
		SystemdNspawnDirectoryPath: *nspawnDirectoryPath,
	}).InstallTenantContainer(*tenantID)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	printTenantStatus(status)
}

func runTenantBootstrap(arguments []string) {
	flags := flag.NewFlagSet("tenant bootstrap", flag.ExitOnError)
	tenantID := flags.String("tenant", "", "tenant id")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	binaryDirectoryPath := flags.String("bin-dir", "/usr/local/bin", "directory containing tenant service binaries")
	gatewayURL := flags.String("gateway-url", "", "LLM gateway OpenRouter-compatible chat completion URL")
	deviceToken := flags.String("device-token", "", "tenant device-scoped LLM token")
	gatewaySharedSecret := flags.String("gateway-shared-secret", "", "tenant LLM gateway shared secret")
	releaseDownloadToken := flags.String("release-download-token", "", "release registry download token")
	adminPassword := flags.String("admin-password", "", "tenant initial admin password")
	adminEmail := flags.String("admin-email", "", "tenant admin email")
	modelName := flags.String("model", "", "optional remote model override")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	status, errorValue := (tenantruntime.Service{BasePath: *basePath}).BootstrapTenant(*tenantID, tenantruntime.BootstrapOptions{
		BinaryDirectoryPath:  *binaryDirectoryPath,
		GatewayURL:           *gatewayURL,
		DeviceToken:          *deviceToken,
		GatewaySharedSecret:  *gatewaySharedSecret,
		ReleaseDownloadToken: resolveReleaseDownloadToken(*releaseDownloadToken),
		AdminPassword:        *adminPassword,
		AdminEmail:           *adminEmail,
		ModelName:            *modelName,
	})
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	printTenantStatus(status)
}

func runTenantStart(arguments []string) {
	flags := flag.NewFlagSet("tenant start", flag.ExitOnError)
	tenantID := flags.String("tenant", "", "tenant id")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	nspawnDirectoryPath := flags.String("nspawn-dir", "/etc/systemd/nspawn", "systemd-nspawn configuration directory")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	if errorValue := (tenantruntime.Service{
		BasePath:                   *basePath,
		SystemdNspawnDirectoryPath: *nspawnDirectoryPath,
	}).StartTenant(context.Background(), *tenantID); errorValue != nil {
		fatal(errorValue.Error())
	}
	fmt.Printf("tenant started: %s\n", *tenantID)
}

func runTenantInstallHostRuntime(arguments []string) {
	flags := flag.NewFlagSet("tenant install-host-runtime", flag.ExitOnError)
	tenantID := flags.String("tenant", "", "tenant id")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	systemdDirectoryPath := flags.String("systemd-dir", "/etc/systemd/system", "systemd unit directory")
	gatewayURL := flags.String("gateway-url", "", "LLM gateway OpenRouter-compatible chat completion URL")
	gatewaySharedSecret := flags.String("gateway-shared-secret", "", "tenant LLM gateway shared secret")
	releaseDownloadToken := flags.String("release-download-token", "", "release registry download token")
	modelName := flags.String("model", "", "optional remote model override")
	rootFilesystemTemplatePath := flags.String("rootfs-image", "/opt/internkim/blueclaw-runtime/rootfs.ext4", "Blueclaw rootfs image template path")
	workspaceImageTemplatePath := flags.String("workspace-image", "/var/lib/blueclaw/workspace.ext4", "Blueclaw workspace image template path")
	portBase := flags.Int("port-base", 0, "optional first tenant runtime port")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	status, errorValue := (tenantruntime.Service{
		BasePath:                   *basePath,
		SystemdSystemDirectoryPath: *systemdDirectoryPath,
	}).InstallHostRuntime(context.Background(), *tenantID, tenantruntime.HostRuntimeOptions{
		GatewayURL:                 *gatewayURL,
		GatewaySharedSecret:        *gatewaySharedSecret,
		ReleaseDownloadToken:       resolveReleaseDownloadToken(*releaseDownloadToken),
		ModelName:                  *modelName,
		RootFilesystemTemplatePath: *rootFilesystemTemplatePath,
		WorkspaceImageTemplatePath: *workspaceImageTemplatePath,
		PortBase:                   *portBase,
	})
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	document, errorValue := json.MarshalIndent(status, "", "  ")
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	fmt.Println(string(document))
}

func runTenantExposeMattermost(arguments []string) {
	flags := flag.NewFlagSet("tenant expose-mattermost", flag.ExitOnError)
	tenantIDs := flags.String("tenants", "", "comma-separated tenant ids")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	systemdDirectoryPath := flags.String("systemd-dir", "/etc/systemd/system", "systemd unit directory")
	cloudflaredPath := flags.String("cloudflared", "/usr/local/bin/cloudflared", "cloudflared binary path")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	exposures, errorValue := (tenantruntime.Service{
		BasePath:                   *basePath,
		SystemdSystemDirectoryPath: *systemdDirectoryPath,
		CloudflaredPath:            *cloudflaredPath,
	}).ExposeMattermost(context.Background(), splitCommaValues(*tenantIDs))
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	printMattermostExposures(exposures)
}

func resolveReleaseDownloadToken(explicitToken string) string {
	if strings.TrimSpace(explicitToken) != "" {
		return strings.TrimSpace(explicitToken)
	}
	if token := strings.TrimSpace(os.Getenv("INTERNKIM_RELEASE_DOWNLOAD_TOKEN")); token != "" {
		return token
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return ""
	}
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, ".local", "secrets", "release-download-token"))
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func runTenantSyncCloudflareTunnel(arguments []string) {
	flags := flag.NewFlagSet("tenant sync-cloudflare-tunnel", flag.ExitOnError)
	tenantIDs := flags.String("tenants", "", "comma-separated tenant ids")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	accountID := flags.String("account-id", "", "Cloudflare account id")
	tunnelID := flags.String("tunnel-id", "", "Cloudflare tunnel id")
	apiTokenPath := flags.String("api-token-path", "", "Cloudflare API token file path")
	apiBaseURL := flags.String("api-base-url", tenantruntime.DefaultCloudflareAPIBaseURL, "Cloudflare API base URL")
	publicHostnameTemplate := flags.String("hostname-template", "", "optional public hostname template containing {tenant}")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	status, errorValue := (tenantruntime.Service{BasePath: *basePath}).SyncCloudflareTenantTunnel(context.Background(), tenantruntime.CloudflareTunnelSyncOptions{
		AccountID:              *accountID,
		TunnelID:               *tunnelID,
		APITokenPath:           *apiTokenPath,
		APIBaseURL:             *apiBaseURL,
		PublicHostnameTemplate: *publicHostnameTemplate,
		TenantIDs:              splitCommaValues(*tenantIDs),
	})
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	document, errorValue := json.MarshalIndent(status, "", "  ")
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	fmt.Println(string(document))
}

func runTenantBootstrapMattermost(arguments []string) {
	flags := flag.NewFlagSet("tenant bootstrap-mattermost", flag.ExitOnError)
	credentialsPath := flags.String("credentials", "", "tenant Mattermost credentials JSON path")
	baseURLTemplate := flags.String("base-url-template", "http://127.0.0.1:{port}", "Mattermost base URL template containing {tenant} or {port}")
	blueclawURLTemplate := flags.String("blueclaw-url-template", "http://127.0.0.1:{blueclawPort}", "Blueclaw URL template containing {tenant} or {blueclawPort}; empty disables policy invite")
	publicURLTemplate := flags.String("public-url-template", "", "tenant public URL template containing {tenant}")
	portStart := flags.Int("port-start", 18065, "first tenant Mattermost port")
	language := flags.String("language", "ko", "workspace language")
	tokenOutputRoot := flags.String("token-output-root", "", "optional tenant root for writing Mattermost bot tokens")
	memberValues := repeatedStringFlag{}
	flags.Var(&memberValues, "member", "tenant member as email:name:password")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	members, errorValue := parseMattermostBootstrapMembers(memberValues.Values())
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	statuses, errorValue := tenantruntime.BootstrapMattermostFleetResources(tenantruntime.MattermostFleetBootstrapOptions{
		CredentialsPath:     *credentialsPath,
		BaseURLTemplate:     *baseURLTemplate,
		BlueclawURLTemplate: *blueclawURLTemplate,
		PublicURLTemplate:   *publicURLTemplate,
		PortStart:           *portStart,
		Language:            *language,
		TokenOutputRoot:     *tokenOutputRoot,
		Members:             members,
	})
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	document, errorValue := json.MarshalIndent(statuses, "", "  ")
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	fmt.Println(string(document))
}

func parseMattermostBootstrapMembers(values []string) ([]tenantruntime.MattermostBootstrapMember, error) {
	members := make([]tenantruntime.MattermostBootstrapMember, 0, len(values))
	for _, value := range values {
		parts := strings.Split(value, ":")
		if len(parts) != 3 {
			return nil, errors.New("--member must use email:name:password")
		}
		email := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		password := strings.TrimSpace(parts[2])
		if email == "" || name == "" || password == "" {
			return nil, errors.New("--member requires email, name, and password")
		}
		address, errorValue := mail.ParseAddress(email)
		if errorValue != nil || address.Address != email {
			return nil, errors.New("--member email is invalid")
		}
		members = append(members, tenantruntime.MattermostBootstrapMember{
			Email:    email,
			Name:     name,
			Password: password,
		})
	}
	return members, nil
}

func splitCommaValues(value string) []string {
	values := []string{}
	for _, part := range strings.Split(value, ",") {
		trimmedPart := strings.TrimSpace(part)
		if trimmedPart != "" {
			values = append(values, trimmedPart)
		}
	}
	return values
}

func runTenantStop(arguments []string) {
	flags := flag.NewFlagSet("tenant stop", flag.ExitOnError)
	tenantID := flags.String("tenant", "", "tenant id")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	if errorValue := (tenantruntime.Service{BasePath: *basePath}).StopTenant(context.Background(), *tenantID); errorValue != nil {
		fatal(errorValue.Error())
	}
	fmt.Printf("tenant stopped: %s\n", *tenantID)
}

func runTenantBackup(arguments []string) {
	flags := flag.NewFlagSet("tenant backup", flag.ExitOnError)
	tenantID := flags.String("tenant", "", "tenant id")
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	bundlePath := flags.String("bundle", "", "backup bundle path")
	passphrase := flags.String("passphrase", "", "backup passphrase")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	if *bundlePath == "" {
		fatal("--bundle is required")
	}
	if errorValue := (tenantruntime.Service{BasePath: *basePath}).CreateEncryptedBackup(*tenantID, *bundlePath, *passphrase); errorValue != nil {
		fatal(errorValue.Error())
	}
	fmt.Printf("tenant backup written: %s\n", *bundlePath)
}

func runTenantRestore(arguments []string) {
	flags := flag.NewFlagSet("tenant restore", flag.ExitOnError)
	basePath := flags.String("base", "/srv/internkim/tenants", "tenant base path")
	bundlePath := flags.String("bundle", "", "backup bundle path")
	passphrase := flags.String("passphrase", "", "backup passphrase")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	if *bundlePath == "" {
		fatal("--bundle is required")
	}
	status, errorValue := (tenantruntime.Service{BasePath: *basePath}).RestoreEncryptedBackup(*bundlePath, *passphrase)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	printTenantStatus(status)
}

func tenantManifestFromFlags(profile string, tenantID string, displayName string, assignedHost string, publicURL string, mirrorHost string, deviceID string) (tenantruntime.Manifest, error) {
	switch profile {
	case tenantruntime.ProfileCloudShared:
		return tenantruntime.NewCloudSharedManifest(tenantID, displayName, assignedHost, publicURL, mirrorHost)
	case tenantruntime.ProfileEdgeAppliance:
		return tenantruntime.NewEdgeApplianceManifest(tenantID, displayName, firstNonEmptyString(deviceID, assignedHost), publicURL)
	default:
		return tenantruntime.Manifest{}, fmt.Errorf("unsupported tenant profile: %s", profile)
	}
}

func printTenantStatus(status tenantruntime.TenantStatus) {
	document, errorValue := json.MarshalIndent(status, "", "  ")
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	fmt.Println(string(document))
}

func printTenantFleetStatus(statuses []tenantruntime.FleetTenantStatus) {
	document, errorValue := json.MarshalIndent(statuses, "", "  ")
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	fmt.Println(string(document))
}

func printMattermostExposures(exposures []tenantruntime.MattermostExposure) {
	document, errorValue := json.MarshalIndent(exposures, "", "  ")
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	fmt.Println(string(document))
}
