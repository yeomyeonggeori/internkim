package localfleet

import (
	"path/filepath"
	"strconv"
)

func (service Service) companyPlaneScenarioPlans() []CommandPlan {
	return []CommandPlan{
		service.startCentralPlanePlan(),
		service.command(filepath.Join(service.options.RepositoryRootPath, "tools", "prepare-company-plane")),
		service.prepareContainerKernelPlan(),
		service.labCommand("vm-up"),
		service.shellPlan("check shared workspace", service.checkSharedWorkspaceCommand()),
		service.shellPlan("start localhost tunnel", service.startTunnelCommand()),
		service.labCommand("vm-ssh", "bash /mnt/shared/workspace/lab/scripts/provision-blueclaw-dev-session.sh admin /mnt/shared 1"),
		service.companyPlaneTestPlan(),
	}
}

func (service Service) companyPlaneTestPlan() CommandPlan {
	arguments := []string{
		"sudo", "bash", "/mnt/shared/workspace/lab/scripts/scenario-company-plane.sh",
		service.virtualSessionArtifactDirectoryPath("company-plane"),
		strconv.Itoa(service.options.CompanyAppPort),
	}
	arguments = append(arguments, service.options.ScenarioArguments...)
	return service.labCommand("vm-ssh", quoteShellArguments(arguments))
}
