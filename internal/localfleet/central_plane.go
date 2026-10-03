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
		"--admin-port", strconv.Itoa(service.options.AdminHostPort),
		"--agent-name", service.options.VirtualMachineName,
	)
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
