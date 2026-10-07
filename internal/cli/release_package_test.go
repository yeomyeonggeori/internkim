package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goreleaser/nfpm/v2"
	"github.com/goreleaser/nfpm/v2/files"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
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

func TestTheDeclaredAccountsAreTheFiveServiceAccountsWithNoLogin(t *testing.T) {
	declared := blueclaw.CompanyHostSysusersFile()
	for _, account := range []string{
		blueclaw.BlueclawUser, blueclaw.RelayUserName, blueclaw.EmbeddingUserName,
		blueclaw.CompanyHostDatabaseUser, blueclaw.CompanyHostCacheUser,
	} {
		if !strings.Contains(declared, "u "+account+" - ") {
			t.Errorf("sysusers does not declare %s:\n%s", account, declared)
		}
	}
	if strings.Count(declared, "/usr/sbin/nologin") != 5 {
		t.Errorf("an account can log in:\n%s", declared)
	}
}

func TestTheAdministratorGroupIsDeclared(t *testing.T) {
	if declared := blueclaw.CompanyHostSysusersFile(); !strings.Contains(declared, "g "+blueclaw.CompanyHostAdministratorGroup+" -\n") {
		t.Fatalf("sysusers does not declare the %s group:\n%s", blueclaw.CompanyHostAdministratorGroup, declared)
	}
}

func TestTheAdministratorGroupManagesEveryPackageUnitAndNothingElse(t *testing.T) {
	rules := blueclaw.CompanyHostPolkitRules()
	for _, unit := range blueclaw.CompanyPackageUnits() {
		if !strings.Contains(rules, `"`+unit.FileName()+`"`) {
			t.Errorf("the polkit rules leave out %s:\n%s", unit.FileName(), rules)
		}
	}
	for _, wanted := range []string{`action.id === "org.freedesktop.systemd1.manage-units"`, `subject.isInGroup("` + blueclaw.CompanyHostAdministratorGroup + `")`} {
		if !strings.Contains(rules, wanted) {
			t.Errorf("the polkit rules do not check %s:\n%s", wanted, rules)
		}
	}
	if strings.Count(rules, "polkit.Result.YES") != 1 {
		t.Errorf("the polkit rules grant more than one thing:\n%s", rules)
	}
}

func TestTheDeclaredDirectoriesCarryTheStateRootModeAndTheHelpersSetuidBit(t *testing.T) {
	declared := blueclaw.CompanyHostTmpfilesFile()
	for _, line := range []string{
		"d " + blueclaw.CompanyHostStateRoot + " 0700 root root -",
		"d " + blueclaw.CompanyHostCompaniesRoot + " 0700 root root -",
		"z " + packageLayout.POSIXHelperPath() + " 4755 root root -",
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

func TestEachFormatAndArchitectureShipsUnderANameNoVersionChanges(t *testing.T) {
	expected := []string{
		"internkim-arm64.deb", "internkim-arm64.rpm", "internkim-arm64.pkg.tar.zst",
		"internkim-amd64.deb", "internkim-amd64.rpm", "internkim-amd64.pkg.tar.zst",
	}
	if names := linuxPackageAssetNames(); strings.Join(names, " ") != strings.Join(expected, " ") {
		t.Errorf("a release ships %v, and install.sh asks for %v", names, expected)
	}
}

func TestTheChecksumListNamesEveryPackageTheDirectoryHolds(t *testing.T) {
	directory := t.TempDir()
	for name, contents := range map[string]string{
		"internkim-arm64.deb": "deb bytes", "internkim-amd64.rpm": "rpm bytes", "notes.txt": "not an asset",
	} {
		if errorValue := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o644); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if errorValue := writeReleaseChecksums(directory, "1"); errorValue != nil {
		t.Fatal(errorValue)
	}
	written, errorValue := os.ReadFile(filepath.Join(directory, releaseChecksumsName))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expected := sha256Hex("deb bytes") + "  internkim-arm64.deb\n" + sha256Hex("rpm bytes") + "  internkim-amd64.rpm\n"
	if string(written) != expected {
		t.Errorf("SHA256SUMS reads\n%s\nwanted\n%s", written, expected)
	}
}

func sha256Hex(contents string) string {
	sum := sha256.Sum256([]byte(contents))
	return hex.EncodeToString(sum[:])
}

func TestAnUnknownFormatIsRefusedByName(t *testing.T) {
	if _, errorValue := releaseFormatsNamed("apk"); errorValue == nil {
		t.Fatal("a format nothing builds was accepted")
	}
	chosen, errorValue := releaseFormatsNamed("rpm,archlinux")
	if errorValue != nil || len(chosen.Linux) != 2 || chosen.Homebrew {
		t.Fatalf("rpm,archlinux chose %+v (%v)", chosen, errorValue)
	}
	chosen, errorValue = releaseFormatsNamed(homebrewFormatName)
	if errorValue != nil || len(chosen.Linux) != 0 || !chosen.Homebrew {
		t.Fatalf("homebrew chose %+v (%v)", chosen, errorValue)
	}
}

func TestNoFormatNamedBuildsEverythingARelease(t *testing.T) {
	chosen, errorValue := releaseFormatsNamed("")
	if errorValue != nil || len(chosen.Linux) != len(linuxPackageFormats()) || !chosen.Homebrew {
		t.Fatalf("the release directory a plain `release packages` writes is not the release: %+v (%v)", chosen, errorValue)
	}
}

func TestNoFormatRestartsTheDatabaseOrTheCacheFromAMaintainerScript(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		restarts := 0
		for _, line := range strings.Split(maintainerScript(format, postInstallScript), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "systemctl restart ") {
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
