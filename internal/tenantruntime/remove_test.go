package tenantruntime

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRemoveTenantStopsUnitsRemovesFilesAndDeletesTenantRoot(t *testing.T) {
	commandRunner := &fakeCommandRunner{}
	service := Service{
		BasePath:                   t.TempDir(),
		SystemdSystemDirectoryPath: t.TempDir(),
		SystemdNspawnDirectoryPath: t.TempDir(),
		CommandRunner:              commandRunner,
	}
	manifest := newTestCloudSharedManifest(t)
	manifest.TenantID = "pilot-01"
	manifest.ContainerName = "internkim-pilot-01"
	if _, errorValue := service.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	paths, errorValue := BuildRuntimePaths(service.BasePath, manifest.TenantID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, unit := range tenantRemovalUnits(service.SystemdSystemDirectoryPath, manifest) {
		if unit.path != "" {
			writeTestFileWithMode(t, unit.path, unit.name, 0o644)
		}
	}
	writeTestFileWithMode(t, service.containerUnitPath(manifest), "nspawn", 0o644)

	report, errorValue := service.RemoveTenant(context.Background(), manifest.TenantID, TenantRemoveOptions{})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if !report.ManifestExisted || report.TenantID != "pilot-01" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if directoryExists(paths.TenantRootPath) {
		t.Fatalf("expected tenant root to be deleted: %s", paths.TenantRootPath)
	}
	for _, unit := range tenantRemovalUnits(service.SystemdSystemDirectoryPath, manifest) {
		if unit.path != "" && fileExists(unit.path) {
			t.Fatalf("expected unit file to be removed: %s", unit.path)
		}
	}
	if fileExists(service.containerUnitPath(manifest)) {
		t.Fatalf("expected nspawn config to be removed: %s", service.containerUnitPath(manifest))
	}
	assertRemoveTenantCommands(t, commandRunner.commands)
}

func TestRemoveTenantRequiresManifestUnlessForced(t *testing.T) {
	service := Service{BasePath: t.TempDir(), CommandRunner: &fakeCommandRunner{}}

	_, errorValue := service.RemoveTenant(context.Background(), "pilot-01", TenantRemoveOptions{})
	if errorValue == nil {
		t.Fatal("expected missing manifest to fail without force")
	}

	report, errorValue := service.RemoveTenant(context.Background(), "pilot-01", TenantRemoveOptions{Force: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.ManifestExisted {
		t.Fatalf("expected forced removal to report missing manifest: %+v", report)
	}
}

func assertRemoveTenantCommands(t *testing.T, commands []ExecutableCommand) {
	t.Helper()
	expected := [][]string{
		{"stop", "internkim-tenant-admind-pilot-01.service"},
		{"disable", "internkim-tenant-admind-pilot-01.service"},
		{"stop", "internkim-tenant-blueclaw-pilot-01.service"},
		{"disable", "internkim-tenant-blueclaw-pilot-01.service"},
		{"stop", "internkim-tenant-capabilityd-pilot-01.service"},
		{"disable", "internkim-tenant-capabilityd-pilot-01.service"},
		{"stop", "internkim-tenant-graphiti-pilot-01.service"},
		{"disable", "internkim-tenant-graphiti-pilot-01.service"},
		{"stop", "internkim-mattermost-pilot-01.service"},
		{"disable", "internkim-mattermost-pilot-01.service"},
		{"stop", "internkim-mm-tunnel-pilot-01.service"},
		{"disable", "internkim-mm-tunnel-pilot-01.service"},
		{"stop", "systemd-nspawn@internkim-pilot-01.service"},
		{"disable", "systemd-nspawn@internkim-pilot-01.service"},
		{"daemon-reload"},
	}
	actual := make([][]string, 0, len(commands))
	for _, command := range commands {
		if command.ExecutableName != "systemctl" {
			t.Fatalf("expected systemctl command, got %+v", command)
		}
		actual = append(actual, command.Arguments)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected commands:\nwant: %+v\n got: %+v", expected, actual)
	}
}

func TestRemoveTenantFileReportsAbsentPath(t *testing.T) {
	report := removeTenantFile(filepath.Join(t.TempDir(), "missing.service"))
	if report.Removed || report.SkippedReason != "absent" {
		t.Fatalf("unexpected absent file report: %+v", report)
	}
}

func TestRemoveTenantDirectoryReportsRemovedPath(t *testing.T) {
	path := t.TempDir()
	report := removeTenantDirectory(path)
	if !report.Removed || report.Path != path {
		t.Fatalf("unexpected directory report: %+v", report)
	}
	if _, errorValue := os.Stat(path); !os.IsNotExist(errorValue) {
		t.Fatalf("expected directory to be removed, got %v", errorValue)
	}
}
