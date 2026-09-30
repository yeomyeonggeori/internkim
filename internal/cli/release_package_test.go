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

func TestOnlyDebianScriptsNameDebianTools(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		for _, script := range []packageScript{postInstallScript, preRemoveScript, postRemoveScript} {
			names := strings.Contains(maintainerScript(format, script), "deb-systemd")
			if names != (format.Name == "deb") {
				t.Errorf("the %s %s names deb-systemd-* = %v, and only the deb's scripts may", format.Name, script, names)
			}
		}
	}
}

func TestNoFormatUsesAccountToolsOnlyDebianHas(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		script := maintainerScript(format, postInstallScript)
		for _, debianOnly := range []string{"adduser", "addgroup"} {
			if strings.Contains(script, debianOnly) {
				t.Errorf("the %s postinst calls %s, which is Debian's alone", format.Name, debianOnly)
			}
		}
	}
}

func TestEveryFormatsPostInstallCreatesEveryServiceAccount(t *testing.T) {
	accounts := []string{
		blueclaw.BlueclawUser, blueclaw.RelayUserName,
		blueclaw.CompanyHostDatabaseUser, blueclaw.CompanyHostCacheUser,
	}
	for _, format := range linuxPackageFormats() {
		script := maintainerScript(format, postInstallScript)
		for _, account := range accounts {
			if !strings.Contains(script, "create_system_user "+account+" ") {
				t.Errorf("the %s postinst does not create %s", format.Name, account)
			}
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
	target := debianTargets[0]
	for _, format := range linuxPackageFormats() {
		information := linuxPackageInformation(format, target, "1.2.3", files.Contents{}, nfpm.Scripts{})
		declared := blueclaw.HostPackageDependsForManagers(format.Managers...)
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
	target := debianTargets[0]
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
		information := linuxPackageInformation(format, debianTargets[0], version, files.Contents{}, nfpm.Scripts{})
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
		for _, line := range strings.Split(maintainerScript(format, postInstallScript), "\n") {
			if !strings.HasPrefix(line, "restart_units ") {
				continue
			}
			for _, name := range []string{blueclaw.CompanyHostDatabaseServiceName, blueclaw.CompanyHostCacheServiceName} {
				if strings.Contains(line, name) {
					t.Errorf("the %s postinst restarts %s together with the units bound to it, and systemd fails the job for each of them", format.Name, name)
				}
			}
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

func TestTheRPMOffersEachNameDnfAndZypperGiveARow(t *testing.T) {
	information := linuxPackageInformation(rpmPackageFormat, debianTargets[0], "1.2.3", files.Contents{}, nfpm.Scripts{})
	depends := strings.Join(information.Depends, "\n")
	for _, name := range []string{"nmap-ncat", "netcat-openbsd"} {
		if !strings.Contains(depends, name) {
			t.Errorf("the rpm's dependencies do not offer %s, so a machine that names netcat that way cannot install it:\n%s", name, depends)
		}
	}
}
