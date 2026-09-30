package cli

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/goreleaser/nfpm/v2"
	"github.com/goreleaser/nfpm/v2/files"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestEveryFormatsScriptsAreValidShell(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		for _, script := range []packageScript{postInstallScript, preRemoveScript, postRemoveScript} {
			command := exec.Command("sh", "-n")
			command.Stdin = strings.NewReader(maintainerScript(format, script))
			if output, errorValue := command.CombinedOutput(); errorValue != nil {
				t.Errorf("the %s %s is not valid shell: %s", format.Name, script, output)
			}
		}
	}
}

func TestNoFormatsScriptNamesADistributionsOwnAccountOrUnitTools(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		for _, script := range []packageScript{postInstallScript, preRemoveScript, postRemoveScript} {
			body := maintainerScript(format, script)
			for _, tool := range []string{"deb-systemd", "adduser", "addgroup", "useradd", "groupadd"} {
				if strings.Contains(body, tool) {
					t.Errorf("the %s %s names %s, which belongs to one family", format.Name, script, tool)
				}
			}
		}
	}
}

func TestEveryFormatsScriptsAreTheSameProgramBesideTheirPrologue(t *testing.T) {
	withoutPrologue := func(format linuxPackageFormat) string {
		return strings.ReplaceAll(maintainerScript(format, postInstallScript), format.RunsOnlyWhen(string(postInstallScript)), "")
	}
	for _, format := range linuxPackageFormats() {
		if withoutPrologue(format) != withoutPrologue(debianPackageFormat) {
			t.Errorf("the %s postinst differs from the deb's beyond its prologue", format.Name)
		}
	}
}

func TestEveryFormatsPostInstallDeclaresAccountsAndDirectoriesToSystemd(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		script := maintainerScript(format, postInstallScript)
		for _, command := range []string{
			"systemd-sysusers " + blueclaw.CompanyPackageSysusersPath,
			"systemd-tmpfiles --create " + blueclaw.CompanyPackageTmpfilesPath,
			"systemctl daemon-reload",
			"systemctl enable",
		} {
			if !strings.Contains(script, command) {
				t.Errorf("the %s postinst does not run %q", format.Name, command)
			}
		}
	}
}

func TestTheDeclaredAccountsAreTheFourServiceAccountsWithNoLogin(t *testing.T) {
	declared := blueclaw.CompanyHostSysusersFile()
	for _, account := range []string{
		blueclaw.BlueclawUser, blueclaw.RelayUserName,
		blueclaw.CompanyHostDatabaseUser, blueclaw.CompanyHostCacheUser,
	} {
		if !strings.Contains(declared, "u "+account+" - ") {
			t.Errorf("sysusers does not declare %s:\n%s", account, declared)
		}
	}
	if strings.Count(declared, "/usr/sbin/nologin") != 4 {
		t.Errorf("an account can log in:\n%s", declared)
	}
}

func TestTheDeclaredDirectoriesCarryTheStateRootModeAndTheHelpersSetuidBit(t *testing.T) {
	declared := blueclaw.CompanyHostTmpfilesFile()
	for _, line := range []string{
		"d " + blueclaw.CompanyHostStateRoot + " 0700 root root -",
		"d " + blueclaw.CompanyHostCompaniesRoot + " 0700 root root -",
		"z " + blueclaw.CompanyHostPOSIXHelperPath + " 4755 root root -",
	} {
		if !strings.Contains(declared, line) {
			t.Errorf("tmpfiles does not carry %q:\n%s", line, declared)
		}
	}
}

func TestOnlyARemovalStopsTheUnitsInEveryFormat(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		script := maintainerScript(format, preRemoveScript)
		if !strings.Contains(script, "if "+format.RemovalTest("prerm")+"; then") {
			t.Errorf("the %s prerm stops the units whatever the reason it ran", format.Name)
		}
	}
}

func TestEveryFormatDependsOnTheSameDeclarationInItsOwnDialect(t *testing.T) {
	target := packageTargets[0]
	for _, format := range linuxPackageFormats() {
		information := linuxPackageInformation(format, target, "1.2.3", files.Contents{}, nfpm.Scripts{})
		declared := blueclaw.HostPackageDependsFor(format.Manager)
		if strings.Join(information.Depends, ",") != strings.Join(declared, ",") {
			t.Errorf("the %s package depends on %v and the declaration gives %v", format.Name, information.Depends, declared)
		}
	}
}

func TestEveryFormatNamesAnNfpmPackager(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		if _, errorValue := nfpm.Get(format.Name); errorValue != nil {
			t.Errorf("nfpm has no %s packager: %v", format.Name, errorValue)
		}
	}
}

func TestEachFormatWritesItsOwnConventionalFileName(t *testing.T) {
	target := packageTargets[0]
	expected := map[string]string{
		"deb":       "internkim_1.2.3_arm64.deb",
		"rpm":       "internkim-1.2.3-1.aarch64.rpm",
		"archlinux": "internkim-1.2.3-1-aarch64.pkg.tar.zst",
	}
	for _, format := range linuxPackageFormats() {
		information := linuxPackageInformation(format, target, "1.2.3", files.Contents{}, nfpm.Scripts{})
		fileName, errorValue := format.packageFileName(information)
		if errorValue != nil || fileName != expected[format.Name] {
			t.Errorf("the %s package is named %q (%v), expected %q", format.Name, fileName, errorValue, expected[format.Name])
		}
	}
}

func TestEveryFormatKeepsTheWholeVersionSoTwoBuildsOfOneDayAreTwoVersions(t *testing.T) {
	version := "0.0.0+20260930.abc1234"
	for _, format := range linuxPackageFormats() {
		information := linuxPackageInformation(format, packageTargets[0], version, files.Contents{}, nfpm.Scripts{})
		fileName, errorValue := format.packageFileName(information)
		if errorValue != nil || !strings.Contains(fileName, version) {
			t.Errorf("the %s package is named %q (%v), which drops part of %s, so a later build of the same day would not upgrade it", format.Name, fileName, errorValue, version)
		}
	}
}

func TestAnUnknownFormatIsRefusedByName(t *testing.T) {
	if _, errorValue := linuxPackageFormatsNamed("apk"); errorValue == nil {
		t.Fatal("a format nothing builds was accepted")
	}
	chosen, errorValue := linuxPackageFormatsNamed("rpm,archlinux")
	if errorValue != nil || len(chosen) != 2 {
		t.Fatalf("rpm,archlinux chose %v (%v)", chosen, errorValue)
	}
}

func TestNoFormatRestartsTheDatabaseOrTheCacheFromAMaintainerScript(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		restarts := 0
		for _, line := range strings.Split(maintainerScript(format, postInstallScript), "\n") {
			if !strings.HasPrefix(line, "systemctl restart ") {
				continue
			}
			restarts++
			for _, name := range []string{blueclaw.CompanyHostDatabaseServiceName, blueclaw.CompanyHostCacheServiceName} {
				if strings.Contains(line, name) {
					t.Errorf("the %s postinst restarts %s together with the units bound to it, and systemd fails the job for each of them", format.Name, name)
				}
			}
		}
		if restarts == 0 {
			t.Errorf("the %s postinst restarts nothing, so this test reads no line", format.Name)
		}
	}
}

func TestArchContentsGetWholeSecondTimesAndTheSetuidBitAsAPOSIXBit(t *testing.T) {
	fractional := time.Unix(1790210019, 600_000_000)
	contents := files.Contents{
		&files.Content{Destination: "/usr/lib/helper", FileInfo: &files.ContentFileInfo{Mode: os.ModeSetuid | 0o755, MTime: fractional}},
		&files.Content{Destination: "/usr/bin/plain", FileInfo: &files.ContentFileInfo{Mode: 0o755, MTime: fractional}},
	}
	translated := archContents(contents)
	if translated[0].FileInfo.Mode != 0o4755 || translated[1].FileInfo.Mode != 0o755 {
		t.Errorf("helper mode %o, plain mode %o, wanted 4755 and 755", translated[0].FileInfo.Mode, translated[1].FileInfo.Mode)
	}
	for _, content := range translated {
		if content.FileInfo.MTime.Nanosecond() != 0 || content.FileInfo.MTime.Unix() != 1790210019 {
			t.Errorf("%s keeps modification time %v, which the tar header and the .MTREE would round two ways", content.Destination, content.FileInfo.MTime)
		}
	}
	if contents[0].FileInfo.Mode != os.ModeSetuid|0o755 || contents[0].FileInfo.MTime != fractional {
		t.Fatal("the shared contents were changed in place")
	}
}
