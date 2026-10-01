package admind

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
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
	if !strings.Contains(command, "systemd-run --unit=internkim-migration-export ") {
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

func TestTheLiveExportLooksForTheGuestWhereTheSupervisorPutsIt(t *testing.T) {
	if !strings.Contains(migrationExportScript(""), "find "+blueclaw.BlueclawRuntimeInstanceDirectoryPath+" -name cloud-hypervisor-api.socket") {
		t.Fatalf("the Cloud Hypervisor API socket lives under the supervisor's runtime directory %s", blueclaw.BlueclawRuntimeInstanceDirectoryPath)
	}
}

func TestAResumedExportNeverPausesTheGuest(t *testing.T) {
	script := migrationExportScript("20261001T143757Z")
	if !strings.Contains(script, "mode=resume") || !strings.Contains(script, "stamp=20261001T143757Z") {
		t.Fatal("a stamp target resumes that export")
	}
	resumeBranch := script[strings.Index(script, `if [ "$mode" = resume ]; then`):]
	resumeBranch = resumeBranch[:strings.Index(resumeBranch, "\nelse\n")]
	if strings.Contains(resumeBranch, "vm.pause") || strings.Contains(resumeBranch, "systemctl stop") {
		t.Fatal("resuming must not pause or stop the agent")
	}
}

func TestTheExportDumpsOnlyTheHostDatabasesTheMoveCarries(t *testing.T) {
	script := migrationExportScript("")
	if !strings.Contains(script, "for database in buzz mattermost; do") {
		t.Fatal("the host databases are named, not listed from the cluster")
	}
	if strings.Contains(script, "from pg_database") {
		t.Fatal("listing every database brings back the damaged leftover blueclaw")
	}
	if !strings.Contains(script, `pg_restore --file=/dev/null`) {
		t.Fatal("each host dump is read back before the export trusts it")
	}
}

func TestADetachedActionRefusesARunningUnitAndClearsAFailedOne(t *testing.T) {
	commands := map[string]string{
		"internkim-migration-export":          migrationExportCommand(""),
		"internkim-blueclaw-workspace-repair": blueclawWorkspaceRepairCommand(),
		"internkim-blueclaw-postgres-salvage": blueclawPostgresSalvageCommand(),
	}
	for unitName, command := range commands {
		refuse := strings.Index(command, "systemctl is-active --quiet "+unitName+"; then")
		clear := strings.Index(command, "systemctl reset-failed "+unitName)
		start := strings.Index(command, "systemd-run --unit="+unitName+" ")
		if refuse < 0 || clear < 0 || start < 0 || !(refuse < clear && clear < start) {
			t.Errorf("%s must refuse a running unit, then clear a failed one, then start: refuse=%d clear=%d start=%d", unitName, refuse, clear, start)
		}
	}
}

func TestADetachedActionRunsTheSameWayAgainAfterItFailed(t *testing.T) {
	directory := t.TempDir()
	log := directory + "/calls"
	fakeSystemctl := "#!/bin/sh\necho \"systemctl $*\" >>" + log + "\n[ \"$1\" = is-active ] && exit 3\nexit 0\n"
	fakeSystemdRun := "#!/bin/sh\necho \"systemd-run $1\" >>" + log + "\n"
	for name, body := range map[string]string{"systemctl": fakeSystemctl, "systemd-run": fakeSystemdRun} {
		if errorValue := os.WriteFile(directory+"/"+name, []byte(body), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	command := detachedRecoveryCommand("internkim-example", "true", "started")
	for attempt := 0; attempt < 2; attempt++ {
		output, errorValue := exec.Command("sh", "-c", "PATH="+directory+":$PATH; "+command).CombinedOutput()
		if errorValue != nil || !strings.Contains(string(output), "started") {
			t.Fatalf("attempt %d: %v %s", attempt, errorValue, output)
		}
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "systemd-run --unit=internkim-example") != 2 {
		t.Fatalf("calls:\n%s", calls)
	}
}

func TestTheMediaMirrorHasADirectoryToWriteInto(t *testing.T) {
	script := migrationExportScript("")
	created := strings.Index(script, `install -d -m 0700 "$export_directory/media"`)
	mirrored := strings.Index(script, `mc mirror`)
	if created < 0 || mirrored < 0 || created > mirrored {
		t.Fatal("mc mirror refuses a destination that does not exist, so the media directory is made first")
	}
}
