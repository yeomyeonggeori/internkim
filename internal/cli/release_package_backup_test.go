package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func systemctlAnsweringInState(t *testing.T, enablement, systemState string) (string, string) {
	t.Helper()
	directory := t.TempDir()
	logPath := filepath.Join(directory, "systemctl.log")
	script := "#!/bin/sh\necho \"$*\" >> " + logPath + "\n" +
		"if [ \"$1\" = is-enabled ]; then echo " + enablement + "; fi\n" +
		"if [ \"$1\" = is-system-running ]; then echo " + systemState + "; fi\n"
	if errorValue := os.WriteFile(filepath.Join(directory, "systemctl"), []byte(script), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	return directory, logPath
}

func runTheBackupSchedule(t *testing.T, enablement string) string {
	return runTheBackupScheduleInState(t, enablement, "running")
}

func runTheBackupScheduleInState(t *testing.T, enablement, systemState string) string {
	t.Helper()
	directory, logPath := systemctlAnsweringInState(t, enablement, systemState)
	script := "refuse() { echo \"$1\" >&2; exit 1; }\n" + scheduleTheBackupUnlessMasked()
	command := exec.Command("sh", "-c", script)
	command.Env = append(os.Environ(), "PATH="+directory+":"+os.Getenv("PATH"))
	if output, errorValue := command.CombinedOutput(); errorValue != nil {
		t.Fatalf("the post-install's backup step failed: %v: %s", errorValue, output)
	}
	logged, _ := os.ReadFile(logPath)
	return string(logged)
}

func TestThePostInstallSchedulesTheDailyBackup(t *testing.T) {
	timer := blueclaw.CompanyPackageBackupUnits().Timer.FileName()
	for _, enablement := range []string{"disabled", "enabled"} {
		logged := runTheBackupSchedule(t, enablement)
		if !strings.Contains(logged, "enable "+timer) || !strings.Contains(logged, "restart "+timer) {
			t.Errorf("with the timer %s the post-install ran:\n%s", enablement, logged)
		}
	}
}

func TestThePostInstallSchedulesTheBackupInAnImageWithoutStartingIt(t *testing.T) {
	timer := blueclaw.CompanyPackageBackupUnits().Timer.FileName()
	logged := runTheBackupScheduleInState(t, "disabled", "offline")
	if !strings.Contains(logged, "enable "+timer) {
		t.Errorf("an install into an image left the backup unscheduled:\n%s", logged)
	}
	if strings.Contains(logged, "restart "+timer) {
		t.Errorf("an install with systemd not running tried to restart the timer:\n%s", logged)
	}
}

func TestThePostInstallLeavesAMaskedBackupTimerOff(t *testing.T) {
	if logged := runTheBackupSchedule(t, "masked"); strings.Contains(logged, "enable ") || strings.Contains(logged, "restart ") || strings.Contains(logged, "unmask") {
		t.Errorf("an upgrade turned scheduled backups back on:\n%s", logged)
	}
}

func TestThePostInstallNeverUnmasksAUnit(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		if strings.Contains(maintainerScript(format, postInstallScript), "unmask") {
			t.Errorf("%s's postinst undoes an administrator's mask", format.Name)
		}
	}
}

func TestTheBackupServiceIsNeitherEnabledNorRestartedByThePackage(t *testing.T) {
	service := blueclaw.CompanyPackageBackupUnits().Service.FileName()
	for name, names := range map[string]string{"enabled": unitFileNames(), "restarted": restartedUnitFileNames()} {
		if strings.Contains(names, service) {
			t.Errorf("%s is %s with the bundle, so installing the package would run a backup", service, name)
		}
	}
	if strings.Contains(blueclaw.CompanyPackageBackupUnits().Service.Contents, "[Install]") {
		t.Errorf("%s has an [Install] section, so it could be started by something other than its timer", service)
	}
}

func TestARemovalStopsTheBackupTimer(t *testing.T) {
	timer := blueclaw.CompanyPackageBackupUnits().Timer.FileName()
	for _, format := range linuxPackageFormats() {
		removal := maintainerScript(format, preRemoveScript)
		if !strings.Contains(removal, "stop "+timer) || !strings.Contains(removal, "disable "+timer) {
			t.Errorf("%s's prerm leaves %s scheduled", format.Name, timer)
		}
	}
}

func TestTheOperatorsSettingsFileDecidesHowManyBackupsAreKept(t *testing.T) {
	service := blueclaw.CompanyPackageBackupUnits().Service.Contents
	defaults := systemdEnvironment(t, service, map[string]string{})
	if defaults[blueclaw.CompanyHostBackupsKeptSetting] != blueclaw.CompanyHostBackupsKeptByDefault {
		t.Errorf("the unit keeps %q backups by default", defaults[blueclaw.CompanyHostBackupsKeptSetting])
	}
	edited := systemdEnvironment(t, service, map[string]string{
		blueclaw.CompanyHostSettingsPath: blueclaw.CompanyHostBackupsKeptSetting + "=30\n",
	})
	if edited[blueclaw.CompanyHostBackupsKeptSetting] != "30" {
		t.Errorf("an operator's %s=30 became %q", blueclaw.CompanyHostBackupsKeptSetting, edited[blueclaw.CompanyHostBackupsKeptSetting])
	}
	if !strings.Contains(service, " backup --keep ${"+blueclaw.CompanyHostBackupsKeptSetting+"}") {
		t.Errorf("the service does not run the backup with the kept count:\n%s", service)
	}
}

func TestThePackageCarriesTheBackupTimerAndItsService(t *testing.T) {
	staging := t.TempDir()
	packaged, errorValue := writeRenderedFiles(staging)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	backup := blueclaw.CompanyPackageBackupUnits()
	wanted := map[string]bool{backup.Service.InstalledPath(): false, backup.Timer.InstalledPath(): false}
	for _, file := range packaged {
		if _, isWanted := wanted[file.Destination]; isWanted {
			wanted[file.Destination] = true
		}
	}
	for destination, isCarried := range wanted {
		if !isCarried {
			t.Errorf("the package does not carry %s", destination)
		}
	}
}

// systemd.exec(5): "Settings from these files override settings made with
// Environment=", whatever order the two are written in.
func systemdEnvironment(t *testing.T, unit string, files map[string]string) map[string]string {
	t.Helper()
	environment := map[string]string{}
	filePaths := []string{}
	for _, line := range strings.Split(unit, "\n") {
		if assignment, isSetting := strings.CutPrefix(line, "Environment="); isSetting {
			name, value, _ := strings.Cut(assignment, "=")
			environment[name] = value
		}
		if filePath, isFile := strings.CutPrefix(line, "EnvironmentFile="); isFile {
			filePaths = append(filePaths, strings.TrimPrefix(filePath, "-"))
		}
	}
	for _, filePath := range filePaths {
		for _, line := range strings.Split(files[filePath], "\n") {
			if name, value, isAssignment := strings.Cut(line, "="); isAssignment {
				environment[name] = value
			}
		}
	}
	return environment
}
