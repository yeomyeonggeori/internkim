package localfleet

import (
	"path/filepath"
	"strconv"
	"strings"
)

// The local record answers on the port the Supabase CLI always uses, and the
// device reaches it by the same number over the reverse forward.
const localRecordPort = 54321

func (service Service) startCentralPlanePlan() CommandPlan {
	return service.command(
		filepath.Join(service.options.RepositoryRootPath, "tools", "start-local-fleet-central-plane"),
		"--state-root", service.options.StateRootPath,
		"--app-port", strconv.Itoa(service.options.CompanyAppPort),
	)
}

func (service Service) centralPlaneSettingsPath() string {
	return filepath.Join(service.options.StateRootPath, "central-plane.env")
}

var centralPlaneDeviceSettings = []struct {
	variable string
	path     string
	mode     string
}{
	{"CENTRAL_PLANE_APP_URL", "/root/.internkim/env/central-plane-app-url", "644"},
	{"CENTRAL_PLANE_PROJECT_URL", "/root/.internkim/env/central-plane-project-url", "644"},
	{"CENTRAL_PLANE_PUBLISHABLE_KEY", "/root/.internkim/env/central-plane-publishable-key", "644"},
	{"CENTRAL_PLANE_AGENT_KEY", "/root/.internkim/secrets/central-plane-agent-key", "600"},
}

// A device that names no company keeps its own records, so the fleet says which
// company this one belongs to before setup brings its services up.
func (service Service) joinCentralPlaneCommand() string {
	remoteSteps := []string{
		"set -e",
		"mkdir -p /root/.internkim/env /root/.internkim/secrets",
	}
	carried := []string{}
	for _, setting := range centralPlaneDeviceSettings {
		remoteSteps = append(remoteSteps,
			"printf '%s\\n' \"$"+setting.variable+"\" > "+setting.path,
			"chmod "+setting.mode+" "+setting.path,
		)
		carried = append(carried, setting.variable+"=\"$"+setting.variable+"\"")
	}
	return strings.Join([]string{
		". " + quoteShell(service.centralPlaneSettingsPath()),
		quoteShell(service.options.ExecutablePath) + " lab vm-ssh --config " + quoteShell(service.configurationPath()) +
			" -- sudo env " + strings.Join(carried, " ") +
			" sh -c " + quoteShell(strings.Join(remoteSteps, "; ")),
	}, " && ")
}

// The company is only reachable while the fleet's ssh session is up, so the app
// it fronts stops with the fleet rather than outliving it.
func (service Service) stopCentralPlaneCommand() string {
	pidPath := quoteShell(filepath.Join(service.options.StateRootPath, "central-plane-app.pid"))
	return strings.Join([]string{
		"if [ -s " + pidPath + " ]; then kill \"$(cat " + pidPath + ")\" 2>/dev/null || true; fi",
		"rm -f " + pidPath,
	}, " && ")
}
