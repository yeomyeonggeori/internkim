package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
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
	case "bootstrap":
		runTenantBootstrap(os.Args[3:])
	case "bootstrap-mattermost":
		runTenantBootstrapMattermost(os.Args[3:])
	case "expose-mattermost":
		runTenantExposeMattermost(os.Args[3:])
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
	fmt.Println("  create-fleet  Create many cloud-shared tenants and print Mattermost URLs")
	fmt.Println("  status   Read tenant manifest and runtime directory status")
	fmt.Println("  backup   Create an encrypted tenant backup")
	fmt.Println("  restore  Restore an encrypted tenant backup")
	fmt.Println("  install-container  Write the systemd-nspawn container configuration")
	fmt.Println("  bootstrap  Install tenant rootfs services and gateway token")
	fmt.Println("  bootstrap-mattermost  Ensure tenant Mattermost team, bot, and default channels")
	fmt.Println("  expose-mattermost  Start Cloudflare quick tunnels for tenant Mattermost instances")
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
	adminPassword := flags.String("admin-password", "", "tenant initial admin password")
	adminEmail := flags.String("admin-email", "", "tenant admin email")
	modelName := flags.String("model", "", "optional remote model override")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	status, errorValue := (tenantruntime.Service{BasePath: *basePath}).BootstrapTenant(*tenantID, tenantruntime.BootstrapOptions{
		BinaryDirectoryPath: *binaryDirectoryPath,
		GatewayURL:          *gatewayURL,
		DeviceToken:         *deviceToken,
		GatewaySharedSecret: *gatewaySharedSecret,
		AdminPassword:       *adminPassword,
		AdminEmail:          *adminEmail,
		ModelName:           *modelName,
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

func runTenantBootstrapMattermost(arguments []string) {
	flags := flag.NewFlagSet("tenant bootstrap-mattermost", flag.ExitOnError)
	credentialsPath := flags.String("credentials", "", "tenant Mattermost credentials JSON path")
	baseURLTemplate := flags.String("base-url-template", "http://127.0.0.1:{port}", "Mattermost base URL template containing {tenant} or {port}")
	publicURLTemplate := flags.String("public-url-template", "", "tenant public URL template containing {tenant}")
	portStart := flags.Int("port-start", 18065, "first tenant Mattermost port")
	language := flags.String("language", "ko", "workspace language")
	tokenOutputRoot := flags.String("token-output-root", "", "optional tenant root for writing Mattermost bot tokens")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	statuses, errorValue := tenantruntime.BootstrapMattermostFleetResources(tenantruntime.MattermostFleetBootstrapOptions{
		CredentialsPath:   *credentialsPath,
		BaseURLTemplate:   *baseURLTemplate,
		PublicURLTemplate: *publicURLTemplate,
		PortStart:         *portStart,
		Language:          *language,
		TokenOutputRoot:   *tokenOutputRoot,
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
