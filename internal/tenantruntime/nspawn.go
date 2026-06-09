package tenantruntime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (service Service) InstallTenantContainer(tenantID string) (TenantStatus, error) {
	manifest, paths, errorValue := service.cloudSharedTenant(tenantID)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	if errorValue := os.MkdirAll(service.nspawnDirectoryPath(), 0o755); errorValue != nil {
		return TenantStatus{}, errorValue
	}
	document := buildNspawnConfiguration(paths)
	if errorValue := os.WriteFile(service.containerUnitPath(manifest), []byte(document), 0o644); errorValue != nil {
		return TenantStatus{}, errorValue
	}
	return service.Status(manifest.TenantID)
}

func (service Service) StartTenant(ctx context.Context, tenantID string) error {
	manifest, paths, errorValue := service.cloudSharedTenant(tenantID)
	if errorValue != nil {
		return errorValue
	}
	if !bootableRootFilesystemExists(paths.ContainerRootPath) {
		return errors.New("tenant rootfs is not bootable: expected systemd or init inside rootfs")
	}
	if _, errorValue := service.InstallTenantContainer(manifest.TenantID); errorValue != nil {
		return errorValue
	}
	_, errorValue = service.commandRunner().Run(ctx, ExecutableCommand{
		ExecutableName: "systemctl",
		Arguments:      []string{"start", systemdNspawnServiceName(manifest)},
	})
	return errorValue
}

func (service Service) StopTenant(ctx context.Context, tenantID string) error {
	manifest, _, errorValue := service.cloudSharedTenant(tenantID)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = service.commandRunner().Run(ctx, ExecutableCommand{
		ExecutableName: "systemctl",
		Arguments:      []string{"stop", systemdNspawnServiceName(manifest)},
	})
	return errorValue
}

func (service Service) cloudSharedTenant(tenantID string) (Manifest, RuntimePaths, error) {
	manifest, errorValue := service.ReadManifest(tenantID)
	if errorValue != nil {
		return Manifest{}, RuntimePaths{}, errorValue
	}
	if manifest.Profile != ProfileCloudShared {
		return Manifest{}, RuntimePaths{}, errors.New("tenant container lifecycle is only supported for cloud-shared tenants")
	}
	paths, errorValue := BuildRuntimePaths(service.BasePath, manifest.TenantID)
	if errorValue != nil {
		return Manifest{}, RuntimePaths{}, errorValue
	}
	return manifest, paths, nil
}

func (service Service) commandRunner() CommandRunner {
	if service.CommandRunner != nil {
		return service.CommandRunner
	}
	return OperatingSystemCommandRunner{}
}

func buildNspawnConfiguration(paths RuntimePaths) string {
	return strings.Join([]string{
		"[Exec]",
		"Boot=yes",
		"PrivateUsers=no",
		"",
		"[Files]",
		"Directory=" + filepath.Clean(paths.ContainerRootPath),
		"Bind=" + filepath.Clean(paths.InternKimPath) + ":/root/.internkim",
		"Bind=" + filepath.Clean(paths.BlueclawRootPath) + ":/root/.blueclaw",
		"TemporaryFileSystem=/tmp",
		"",
		"[Network]",
		"Private=yes",
		"VirtualEthernet=yes",
		"",
	}, "\n")
}

func systemdNspawnServiceName(manifest Manifest) string {
	return fmt.Sprintf("systemd-nspawn@%s.service", manifest.ContainerName)
}
