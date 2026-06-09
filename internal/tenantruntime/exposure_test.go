package tenantruntime

import (
	"context"
	"path/filepath"
	"testing"
)

type outputCommandRunner struct {
	commands []ExecutableCommand
	outputs  []string
}

func (commandRunner *outputCommandRunner) Run(ctx context.Context, executableCommand ExecutableCommand) (string, error) {
	if errorValue := ctx.Err(); errorValue != nil {
		return "", errorValue
	}
	commandRunner.commands = append(commandRunner.commands, executableCommand)
	if len(commandRunner.outputs) == 0 {
		return "", nil
	}
	output := commandRunner.outputs[0]
	commandRunner.outputs = commandRunner.outputs[1:]
	return output, nil
}

func TestExposeMattermostWritesTunnelUnitAndReturnsPublicURL(t *testing.T) {
	commandRunner := &outputCommandRunner{
		outputs: []string{"", "", "", "INF +--------------------------------------------------------------------------------------------+\nINF |  https://tenant-one.trycloudflare.com                                                     |\n"},
	}
	service := Service{
		BasePath:                   t.TempDir(),
		SystemdSystemDirectoryPath: t.TempDir(),
		CloudflaredPath:            "/usr/bin/cloudflared",
		CommandRunner:              commandRunner,
	}
	manifest := newTestCloudSharedManifest(t)
	manifest.MattermostInstance = MattermostInstance{Port: 18065}
	if _, errorValue := service.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}

	exposures, errorValue := service.ExposeMattermost(context.Background(), []string{manifest.TenantID})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(exposures) != 1 || exposures[0].PublicURL != "https://tenant-one.trycloudflare.com" {
		t.Fatalf("unexpected exposures: %+v", exposures)
	}
	unitPath := filepath.Join(service.SystemdSystemDirectoryPath, "internkim-mm-tunnel-acme.service")
	assertFileContains(t, unitPath, "ExecStart=/usr/bin/cloudflared tunnel --no-autoupdate --url http://127.0.0.1:18065")
	if len(commandRunner.commands) != 4 {
		t.Fatalf("expected daemon-reload, enable, restart, journalctl commands, got %+v", commandRunner.commands)
	}
}
