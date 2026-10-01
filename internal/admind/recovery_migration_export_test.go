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
		"share":          migrationExportShareCommand("20261001T142240Z"),
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

func TestSharingAnExportKeepsTheImageRootOnly(t *testing.T) {
	root := t.TempDir()
	exportDirectory := root + "/20261001T142240Z"
	script := strings.NewReplacer(
		`chown -R root:"$readerGroup" "$export_directory"`, `:`,
		`chgrp root "$export_directory/workspace.ext4.tar.zst"`, `:`,
	).Replace(migrationExportShareSnippet)
	setup := "mkdir -p " + exportDirectory + "/media && umask 077 && touch " + exportDirectory + "/blueclaw.dump " + exportDirectory + "/media/object " + exportDirectory + "/workspace.ext4.tar.zst"
	output, errorValue := exec.Command("sh", "-c", setup+" && export_directory="+exportDirectory+" readerGroup=staff && "+script+" && stat -f '%Lp %N' "+exportDirectory+" "+exportDirectory+"/* "+exportDirectory+"/media/object 2>/dev/null || stat -c '%a %n' "+exportDirectory+" "+exportDirectory+"/* "+exportDirectory+"/media/object").CombinedOutput()
	if errorValue != nil {
		t.Fatalf("%v\n%s", errorValue, output)
	}
	modes := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		modes[strings.TrimPrefix(fields[1], root+"/")] = fields[0]
	}
	expected := map[string]string{
		"20261001T142240Z":                        "750",
		"20261001T142240Z/blueclaw.dump":          "640",
		"20261001T142240Z/media":                  "750",
		"20261001T142240Z/media/object":           "640",
		"20261001T142240Z/workspace.ext4.tar.zst": "600",
	}
	for path, mode := range expected {
		if modes[path] != mode {
			t.Errorf("%s has mode %s, want %s (all: %v)", path, modes[path], mode, modes)
		}
	}
}

func TestSharingRefusesAnythingButAStamp(t *testing.T) {
	for _, target := range []string{"", "../etc", "20261001T142240Z; rm -rf /", "latest"} {
		if strings.Contains(migrationExportShareCommand(target), "chmod") {
			t.Errorf("target %q reached the shell", target)
		}
	}
	if !strings.Contains(migrationExportShareCommand("20261001T142240Z"), "/var/lib/internkim-migration/20261001T142240Z") {
		t.Error("a stamp names its export directory")
	}
}
