package machost

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"

	blueclawworkspace "github.com/yeomyeonggeori/internkim/internal/blueclawworkspace"
	blueclaw "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type InstallRequest struct {
	Layout                   Layout
	RepositoryRootPath       string
	ArtifactDirectoryPath    string
	PayloadDirectoryPath     string
	HostHTTPListenAddress    string
	ModelName                string
	WorkspaceMinimumBytes    int64
	ReadCodesignEntitlements CodesignRunner
}

func Install(request InstallRequest) error {
	if errorValue := request.Layout.AssertVSockSocketPathFits(); errorValue != nil {
		return errorValue
	}
	monitorPath := FindVirtualMachineMonitor(request.Layout)
	if errorValue := RequireVirtualMachineMonitor(monitorPath, request.ReadCodesignEntitlements); errorValue != nil {
		return errorValue
	}
	if _, errorValue := blueclaw.ValidatePayloadArtifactDirectory(request.PayloadDirectoryPath); errorValue != nil {
		return fmt.Errorf("blueclaw payload artifact invalid: %w", errorValue)
	}
	if errorValue := InstallRuntimeArtifacts(request.Layout, request.ArtifactDirectoryPath); errorValue != nil {
		return errorValue
	}
	if errorValue := blueclaw.EnsureBlueclawSupervisorBinaryForHost(
		request.Layout.SupervisorBinaryPath(), request.RepositoryRootPath, goruntime.GOOS, goruntime.GOARCH,
	); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(request.Layout.RuntimeRootPath, deliveryDirectoryMode); errorValue != nil {
		return errorValue
	}
	sources, errorValue := deliverySourcesFor(request, monitorPath)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := WriteDeliveryDirectory(request.Layout, sources); errorValue != nil {
		return errorValue
	}
	return SealDeliveryDirectory(request.Layout)
}

func deliverySourcesFor(request InstallRequest, monitorPath string) (DeliverySources, error) {
	runtimeConfigurationJSON, errorValue := blueclaw.BlueclawRuntimeConfigDocumentWithOptions(runtimeConfigOptionsFor(request, monitorPath))
	if errorValue != nil {
		return DeliverySources{}, errorValue
	}
	policyJSON, errorValue := blueclaw.BlueclawPolicyDocument("")
	if errorValue != nil {
		return DeliverySources{}, errorValue
	}
	skillRootPaths, errorValue := blueclawworkspace.SkillRootPaths(request.RepositoryRootPath)
	if errorValue != nil {
		return DeliverySources{}, errorValue
	}
	return DeliverySources{
		PayloadRuntimePath:       filepath.Join(blueclaw.PayloadWorkspacePath(request.PayloadDirectoryPath), ".blueclaw", "runtime", "current"),
		SkillPaths:               skillRootPaths,
		RuntimeConfigurationJSON: runtimeConfigurationJSON,
		PolicyJSON:               policyJSON,
	}, nil
}

func runtimeConfigOptionsFor(request InstallRequest, monitorPath string) blueclaw.RuntimeConfigOptions {
	return blueclaw.RuntimeConfigOptions{
		ModelName:                  request.ModelName,
		VirtualMachineMonitor:      blueclaw.VfkitMonitorName,
		VfkitPath:                  monitorPath,
		KernelImagePath:            request.Layout.KernelImagePath(),
		RootFilesystemImagePath:    request.Layout.RootFilesystemImagePath(),
		WorkspaceImagePath:         request.Layout.WorkspaceImagePath(),
		DeliveryDirectoryPath:      request.Layout.DeliveryPath(),
		LogDirectoryPath:           request.Layout.LogDirectoryPath(),
		RuntimeDirectoryPath:       request.Layout.RuntimeRootPath,
		HostHTTPListenAddress:      request.HostHTTPListenAddress,
		WorkspaceMinimumBytes:      request.WorkspaceMinimumBytes,
		HostRunsNoCapabilityDaemon: true,
		BaseURL:                    "http://" + request.HostHTTPListenAddress,
	}
}
