package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"gitlab.com/eastriver/internkim/internal/tenantruntime"
)

func runTenantContainer(arguments []string) {
	if len(arguments) == 0 || arguments[0] == "--help" || arguments[0] == "-h" {
		printTenantContainerUsage()
		return
	}
	exitCode, errorValue := executeTenantContainerCommand(arguments[0], arguments[1:], tenantruntime.OperatingSystemContainerCommandExecutor{}, os.Stdout, os.Stderr)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func printTenantContainerUsage() {
	fmt.Println("Usage: internkim tenant container <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  infra-up    Write and start shared Postgres and Mattermost compose services")
	fmt.Println("  infra-down  Stop shared Postgres and Mattermost compose services")
	fmt.Println("  add         Add one tenant container")
	fmt.Println("  remove      Remove one tenant container")
	fmt.Println("  reset       Remove all tenant containers")
	fmt.Println("  up          Start infra, build the staged tenant image if missing, and add N tenants")
	fmt.Println("  status      Print container tenant status as JSON")
	fmt.Println()
	fmt.Println("Tenant image staging for up: place Dockerfile, entrypoint.sh, bin/, and migrations/ under --workdir tenant/")
}

func executeTenantContainerCommand(commandName string, arguments []string, executor tenantruntime.ContainerCommandExecutor, output io.Writer, errorOutput io.Writer) (int, error) {
	runtime := tenantruntime.ContainerRuntime{CommandExecutor: executor, Output: output, ErrorOutput: errorOutput}
	switch commandName {
	case "infra-up":
		return executeTenantContainerInfraUp(arguments, runtime, output)
	case "infra-down":
		return executeTenantContainerInfraDown(arguments, runtime, output)
	case "add":
		return executeTenantContainerAdd(arguments, runtime, output)
	case "remove":
		return executeTenantContainerRemove(arguments, runtime, output)
	case "reset":
		return executeTenantContainerReset(arguments, runtime, output)
	case "up":
		return executeTenantContainerUp(arguments, runtime, output)
	case "status":
		return executeTenantContainerStatus(arguments, runtime, output)
	default:
		printTenantContainerUsage()
		return 0, nil
	}
}

func executeTenantContainerInfraUp(arguments []string, runtime tenantruntime.ContainerRuntime, output io.Writer) (int, error) {
	flags := flag.NewFlagSet("tenant container infra-up", flag.ContinueOnError)
	workDirectoryPath := flags.String("workdir", tenantruntime.DefaultContainerWorkDirectoryPath, "container runtime work directory")
	tenantCount := flags.Int("tenant-count", 10, "tenant database count created by the initial Postgres volume")
	openRouterKeyPath := flags.String("openrouter-key", "", "OpenRouter key file path")
	mattermostPublicURL := flags.String("mattermost-public-url", tenantruntime.DefaultContainerMattermostURL, "public Mattermost URL used as the shared Mattermost SiteURL")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return 1, errorValue
	}
	status, errorValue := runtime.InfraUp(tenantruntime.ContainerInfraUpOptions{
		WorkDirectoryPath:   *workDirectoryPath,
		TenantCount:         *tenantCount,
		OpenRouterKeyPath:   *openRouterKeyPath,
		MattermostPublicURL: *mattermostPublicURL,
	})
	if errorValue != nil {
		return 1, errorValue
	}
	return printTenantContainerDocument(output, status)
}

func executeTenantContainerInfraDown(arguments []string, runtime tenantruntime.ContainerRuntime, output io.Writer) (int, error) {
	flags := flag.NewFlagSet("tenant container infra-down", flag.ContinueOnError)
	workDirectoryPath := flags.String("workdir", tenantruntime.DefaultContainerWorkDirectoryPath, "container runtime work directory")
	purge := flags.Bool("purge", false, "remove compose volumes")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return 1, errorValue
	}
	status, errorValue := runtime.InfraDown(tenantruntime.ContainerInfraDownOptions{WorkDirectoryPath: *workDirectoryPath, Purge: *purge})
	if errorValue != nil {
		return 1, errorValue
	}
	return printTenantContainerDocument(output, status)
}

func executeTenantContainerAdd(arguments []string, runtime tenantruntime.ContainerRuntime, output io.Writer) (int, error) {
	flags := flag.NewFlagSet("tenant container add", flag.ContinueOnError)
	workDirectoryPath := flags.String("workdir", tenantruntime.DefaultContainerWorkDirectoryPath, "container runtime work directory")
	tenantID := flags.String("tenant", "", "tenant name such as tenant01")
	modelName := flags.String("model", "", "language model name")
	imageName := flags.String("image", "", "tenant image name")
	openRouterKeyPath := flags.String("openrouter-key", "", "OpenRouter key file path")
	adminEmail := flags.String("admin-email", "", "admin email for policy.json")
	mattermostPublicURL := flags.String("mattermost-public-url", tenantruntime.DefaultContainerMattermostURL, "public Mattermost URL written to admind device-url")
	mattermostLocalURL := flags.String("mattermost-local-url", "http://localhost:8065", "host-reachable Mattermost URL for provisioning API calls")
	flowHostBaseLabel := flags.String("flow-host-base-label", tenantruntime.DefaultContainerFlowHostBaseLabel, "base DNS label for Flow hosts, producing <label>-tNN.<zone>")
	cloudflareEnvironment := flags.String("cloudflare-env", "cf.env", "Cloudflare environment file relative to workdir")
	cloudflareAPIBaseURL := flags.String("cloudflare-api-base-url", tenantruntime.DefaultCloudflareAPIBaseURL, "Cloudflare API base URL")
	cloudflareAPIToken := flags.String("cloudflare-api-token", "", "Cloudflare API token")
	cloudflareAccountID := flags.String("cloudflare-account-id", "", "Cloudflare account id")
	cloudflareZoneID := flags.String("cloudflare-zone-id", "", "Cloudflare zone id")
	cloudflareZoneName := flags.String("cloudflare-zone-name", tenantruntime.DefaultContainerCloudflareZone, "Cloudflare zone name")
	cloudflareTunnelName := flags.String("cloudflare-tunnel-name", tenantruntime.DefaultContainerTunnelName, "Cloudflare named tunnel")
	cloudflareTunnelID := flags.String("cloudflare-tunnel-id", "", "Cloudflare tunnel id")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return 1, errorValue
	}
	if *tenantID == "" {
		return 1, errors.New("--tenant is required")
	}
	status, errorValue := runtime.AddTenant(tenantruntime.ContainerTenantAddOptions{
		WorkDirectoryPath:     *workDirectoryPath,
		TenantID:              *tenantID,
		ModelName:             *modelName,
		ImageName:             *imageName,
		OpenRouterKeyPath:     *openRouterKeyPath,
		AdminEmail:            *adminEmail,
		MattermostPublicURL:   *mattermostPublicURL,
		MattermostLocalURL:    *mattermostLocalURL,
		FlowHostBaseLabel:     *flowHostBaseLabel,
		CloudflareEnvironment: *cloudflareEnvironment,
		CloudflareAPIBaseURL:  *cloudflareAPIBaseURL,
		CloudflareAPIToken:    *cloudflareAPIToken,
		CloudflareAccountID:   *cloudflareAccountID,
		CloudflareZoneID:      *cloudflareZoneID,
		CloudflareZoneName:    *cloudflareZoneName,
		CloudflareTunnelName:  *cloudflareTunnelName,
		CloudflareTunnelID:    *cloudflareTunnelID,
	})
	if errorValue != nil {
		return 1, errorValue
	}
	return printTenantContainerDocument(output, status)
}

func executeTenantContainerRemove(arguments []string, runtime tenantruntime.ContainerRuntime, output io.Writer) (int, error) {
	flags := flag.NewFlagSet("tenant container remove", flag.ContinueOnError)
	workDirectoryPath := flags.String("workdir", tenantruntime.DefaultContainerWorkDirectoryPath, "container runtime work directory")
	tenantID := flags.String("tenant", "", "tenant name such as tenant01")
	purgeData := flags.Bool("purge-data", false, "drop the tenant database")
	imageName := flags.String("image", "", "tenant image name used when regenerating compose")
	openRouterKeyPath := flags.String("openrouter-key", "", "OpenRouter key file path used when regenerating compose")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return 1, errorValue
	}
	if *tenantID == "" {
		return 1, errors.New("--tenant is required")
	}
	status, errorValue := runtime.RemoveTenant(tenantruntime.ContainerTenantRemoveOptions{
		WorkDirectoryPath: *workDirectoryPath,
		TenantID:          *tenantID,
		PurgeData:         *purgeData,
		ImageName:         *imageName,
		OpenRouterKeyPath: *openRouterKeyPath,
	})
	if errorValue != nil {
		return 1, errorValue
	}
	return printTenantContainerDocument(output, status)
}

func executeTenantContainerReset(arguments []string, runtime tenantruntime.ContainerRuntime, output io.Writer) (int, error) {
	flags := flag.NewFlagSet("tenant container reset", flag.ContinueOnError)
	workDirectoryPath := flags.String("workdir", tenantruntime.DefaultContainerWorkDirectoryPath, "container runtime work directory")
	purge := flags.Bool("purge", false, "remove infra volumes and generated state")
	imageName := flags.String("image", "", "tenant image name used when regenerating compose")
	openRouterKeyPath := flags.String("openrouter-key", "", "OpenRouter key file path used when regenerating compose")
	confirmSuperAdmin := flags.Bool("confirm-super-admin", false, "confirm reset as the Mattermost super-admin")
	superAdminPasswordPath := flags.String("super-admin-password", "", "path containing the Mattermost super-admin password")
	mattermostLocalURL := flags.String("mattermost-local-url", "http://localhost:8065", "host-reachable Mattermost URL for reset API calls")
	flowHostBaseLabel := flags.String("flow-host-base-label", tenantruntime.DefaultContainerFlowHostBaseLabel, "base DNS label for Flow hosts")
	cloudflareEnvironment := flags.String("cloudflare-env", "cf.env", "Cloudflare environment file relative to workdir")
	cloudflareAPIBaseURL := flags.String("cloudflare-api-base-url", tenantruntime.DefaultCloudflareAPIBaseURL, "Cloudflare API base URL")
	cloudflareAPIToken := flags.String("cloudflare-api-token", "", "Cloudflare API token")
	cloudflareAccountID := flags.String("cloudflare-account-id", "", "Cloudflare account id")
	cloudflareZoneID := flags.String("cloudflare-zone-id", "", "Cloudflare zone id")
	cloudflareZoneName := flags.String("cloudflare-zone-name", tenantruntime.DefaultContainerCloudflareZone, "Cloudflare zone name")
	cloudflareTunnelName := flags.String("cloudflare-tunnel-name", tenantruntime.DefaultContainerTunnelName, "Cloudflare named tunnel")
	cloudflareTunnelID := flags.String("cloudflare-tunnel-id", "", "Cloudflare tunnel id")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return 1, errorValue
	}
	status, errorValue := runtime.Reset(tenantruntime.ContainerResetOptions{
		WorkDirectoryPath:      *workDirectoryPath,
		Purge:                  *purge,
		ImageName:              *imageName,
		OpenRouterKeyPath:      *openRouterKeyPath,
		ConfirmSuperAdmin:      *confirmSuperAdmin,
		SuperAdminPasswordPath: *superAdminPasswordPath,
		MattermostLocalURL:     *mattermostLocalURL,
		FlowHostBaseLabel:      *flowHostBaseLabel,
		CloudflareEnvironment:  *cloudflareEnvironment,
		CloudflareAPIBaseURL:   *cloudflareAPIBaseURL,
		CloudflareAPIToken:     *cloudflareAPIToken,
		CloudflareAccountID:    *cloudflareAccountID,
		CloudflareZoneID:       *cloudflareZoneID,
		CloudflareZoneName:     *cloudflareZoneName,
		CloudflareTunnelName:   *cloudflareTunnelName,
		CloudflareTunnelID:     *cloudflareTunnelID,
	})
	if errorValue != nil {
		return 1, errorValue
	}
	return printTenantContainerDocument(output, status)
}

func executeTenantContainerUp(arguments []string, runtime tenantruntime.ContainerRuntime, output io.Writer) (int, error) {
	flags := flag.NewFlagSet("tenant container up", flag.ContinueOnError)
	workDirectoryPath := flags.String("workdir", tenantruntime.DefaultContainerWorkDirectoryPath, "container runtime work directory")
	count := flags.Int("count", 0, "tenant count")
	modelName := flags.String("model", "", "language model name")
	imageName := flags.String("image", "", "tenant image name")
	openRouterKeyPath := flags.String("openrouter-key", "", "OpenRouter key file path")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return 1, errorValue
	}
	statuses, errorValue := runtime.Up(tenantruntime.ContainerUpOptions{
		WorkDirectoryPath: *workDirectoryPath,
		Count:             *count,
		ModelName:         *modelName,
		ImageName:         *imageName,
		OpenRouterKeyPath: *openRouterKeyPath,
	})
	if errorValue != nil {
		return 1, errorValue
	}
	return printTenantContainerDocument(output, statuses)
}

func executeTenantContainerStatus(arguments []string, runtime tenantruntime.ContainerRuntime, output io.Writer) (int, error) {
	flags := flag.NewFlagSet("tenant container status", flag.ContinueOnError)
	workDirectoryPath := flags.String("workdir", tenantruntime.DefaultContainerWorkDirectoryPath, "container runtime work directory")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return 1, errorValue
	}
	status, errorValue := runtime.Status(tenantruntime.ContainerStatusOptions{WorkDirectoryPath: *workDirectoryPath})
	if errorValue != nil {
		return 1, errorValue
	}
	return printTenantContainerDocument(output, status)
}

func printTenantContainerDocument(output io.Writer, value any) (int, error) {
	document, errorValue := json.MarshalIndent(value, "", "  ")
	if errorValue != nil {
		return 1, errorValue
	}
	fmt.Fprintln(output, string(document))
	return 0, nil
}
