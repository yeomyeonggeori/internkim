package tenantruntime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeCommandRunner struct {
	commands []ExecutableCommand
}

func (fakeCommandRunner *fakeCommandRunner) Run(
	ctx context.Context,
	executableCommand ExecutableCommand,
) (string, error) {
	if errorValue := ctx.Err(); errorValue != nil {
		return "", errorValue
	}
	fakeCommandRunner.commands = append(fakeCommandRunner.commands, executableCommand)
	return "", nil
}

func TestCreateTenantFromTemplateCopiesBootableRootFilesystem(t *testing.T) {
	service := Service{BasePath: t.TempDir()}
	templateRootFilesystemPath := createTemplateRootFilesystem(t)
	manifest := newTestCloudSharedManifest(t)

	status, errorValue := service.CreateTenantFromTemplate(manifest, templateRootFilesystemPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if !status.ContainerRootBootable {
		t.Fatalf("expected tenant rootfs to be bootable: %+v", status)
	}
	paths, errorValue := BuildRuntimePaths(service.BasePath, manifest.TenantID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !fileExists(filepath.Join(paths.ContainerRootPath, "usr/lib/systemd/systemd")) {
		t.Fatalf("expected systemd marker to be copied into %s", paths.ContainerRootPath)
	}
}

func TestInstallTenantContainerWritesNspawnConfiguration(t *testing.T) {
	service := Service{
		BasePath:                   t.TempDir(),
		SystemdNspawnDirectoryPath: t.TempDir(),
	}
	manifest := newTestCloudSharedManifest(t)
	if _, errorValue := service.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}

	status, errorValue := service.InstallTenantContainer(manifest.TenantID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	document, errorValue := os.ReadFile(status.ContainerUnitPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	text := string(document)
	for _, expectedText := range []string{
		"[Exec]",
		"Boot=yes",
		"PrivateUsers=no",
		"[Files]",
		"Directory=",
		"Bind=",
		":/root/.internkim",
		":/root/.blueclaw",
		"[Network]",
		"Private=yes",
		"VirtualEthernet=yes",
	} {
		if !strings.Contains(text, expectedText) {
			t.Fatalf("expected nspawn configuration to contain %q, got:\n%s", expectedText, text)
		}
	}
	if !status.ContainerUnitInstalled {
		t.Fatalf("expected container unit to be installed: %+v", status)
	}
}

func TestStartTenantInstallsUnitAndStartsSystemdNspawnService(t *testing.T) {
	commandRunner := &fakeCommandRunner{}
	service := Service{
		BasePath:                   t.TempDir(),
		SystemdNspawnDirectoryPath: t.TempDir(),
		CommandRunner:              commandRunner,
	}
	manifest := newTestCloudSharedManifest(t)
	if _, errorValue := service.CreateTenantFromTemplate(manifest, createTemplateRootFilesystem(t)); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.StartTenant(context.Background(), manifest.TenantID); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertSingleCommand(t, commandRunner.commands, "systemctl", []string{"start", "systemd-nspawn@internkim-acme.service"})
	status, errorValue := service.Status(manifest.TenantID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !status.ContainerUnitInstalled {
		t.Fatalf("expected start to install container unit: %+v", status)
	}
}

func TestStartTenantRejectsUnbootableRootFilesystem(t *testing.T) {
	commandRunner := &fakeCommandRunner{}
	service := Service{
		BasePath:      t.TempDir(),
		CommandRunner: commandRunner,
	}
	manifest := newTestCloudSharedManifest(t)
	if _, errorValue := service.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}

	errorValue := service.StartTenant(context.Background(), manifest.TenantID)

	if errorValue == nil {
		t.Fatal("expected start to reject unbootable rootfs")
	}
	if len(commandRunner.commands) != 0 {
		t.Fatalf("expected no systemctl command, got %+v", commandRunner.commands)
	}
}

func TestStartTenantRejectsEdgeApplianceProfile(t *testing.T) {
	service := Service{BasePath: t.TempDir()}
	manifest, errorValue := NewEdgeApplianceManifest("edge", "Edge", "jetson-1", "https://edge.example.test")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}

	errorValue = service.StartTenant(context.Background(), manifest.TenantID)

	if errorValue == nil {
		t.Fatal("expected edge appliance tenant to reject container start")
	}
}

func TestStopTenantStopsSystemdNspawnService(t *testing.T) {
	commandRunner := &fakeCommandRunner{}
	service := Service{
		BasePath:      t.TempDir(),
		CommandRunner: commandRunner,
	}
	manifest := newTestCloudSharedManifest(t)
	if _, errorValue := service.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.StopTenant(context.Background(), manifest.TenantID); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertSingleCommand(t, commandRunner.commands, "systemctl", []string{"stop", "systemd-nspawn@internkim-acme.service"})
}

func createTemplateRootFilesystem(t *testing.T) string {
	t.Helper()
	templateRootFilesystemPath := t.TempDir()
	systemdPath := filepath.Join(templateRootFilesystemPath, "usr/lib/systemd/systemd")
	if errorValue := os.MkdirAll(filepath.Dir(systemdPath), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(systemdPath, []byte("systemd"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	return templateRootFilesystemPath
}

func newTestCloudSharedManifest(t *testing.T) Manifest {
	t.Helper()
	manifest, errorValue := NewCloudSharedManifest("acme", "Acme", "mac-a", "https://acme.example.test", "mac-b")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return manifest
}

func assertSingleCommand(
	t *testing.T,
	commands []ExecutableCommand,
	expectedExecutableName string,
	expectedArguments []string,
) {
	t.Helper()
	if len(commands) != 1 {
		t.Fatalf("expected one command, got %+v", commands)
	}
	if commands[0].ExecutableName != expectedExecutableName {
		t.Fatalf("expected executable %q, got %+v", expectedExecutableName, commands[0])
	}
	if strings.Join(commands[0].Arguments, "\n") != strings.Join(expectedArguments, "\n") {
		t.Fatalf("expected arguments %+v, got %+v", expectedArguments, commands[0].Arguments)
	}
}
