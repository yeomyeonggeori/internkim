package admind

import (
	"os/exec"
	"strings"
	"testing"
)

func TestMigrationScriptsAreShellTheDeviceCanParse(t *testing.T) {
	scripts := map[string]string{
		"inventory":      migrationInventoryCommand(),
		"live export":    migrationExportScript(""),
		"cutover export": migrationExportScript("cutover"),
		"remove":         migrationExportRemoveCommand(),
	}
	for name, script := range scripts {
		output, errorValue := exec.Command("sh", "-n", "-c", script).CombinedOutput()
		if errorValue != nil {
			t.Errorf("%s does not parse: %v\n%s", name, errorValue, output)
		}
	}
}

func TestOnlyACutoverExportStopsTheAgent(t *testing.T) {
	if strings.Contains(migrationExportScript(""), "mode=cutover") {
		t.Fatal("an export without a target must pause the guest, not stop it")
	}
	if !strings.Contains(migrationExportScript("cutover"), "mode=cutover") {
		t.Fatal("the cutover export must stop the device's writers")
	}
	if strings.Contains(migrationExportScript("anything else"), "mode=cutover") {
		t.Fatal("only the exact target cutover stops the agent")
	}
}

func TestTheExportIsStartedDetachedFromTheRequest(t *testing.T) {
	command := migrationExportCommand("")
	if !strings.HasPrefix(command, "systemd-run --unit=internkim-migration-export ") {
		t.Fatalf("the export outlives the recovery request only under its own unit, got %q", command[:60])
	}
}
