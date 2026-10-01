package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goreleaser/nfpm/v2"
	"github.com/goreleaser/nfpm/v2/files"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The install step puts uv's CPython first on every service's PATH, and the skills
// build their environments from that, so no format asks its distribution for a Python.
func TestThePackageAsksTheDistributionForNoPython(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		depends := strings.Join(linuxPackageInformation(format, packageTargets[0], "1.2.3", files.Contents{}, nfpm.Scripts{}).Depends, ", ")
		if strings.Contains(depends, "python") {
			t.Fatalf("the %s package's Depends is %q and still asks the distribution for a Python", format.Name, depends)
		}
	}
}

func TestThePackageAsksTheDistributionForNothingItCarries(t *testing.T) {
	depends := strings.Join(linuxPackageInformation(debianPackageFormat, packageTargets[0], "1.2.3", files.Contents{}, nfpm.Scripts{}).Depends, ", ")
	for _, carried := range []string{"fonts-nanum", "chromium"} {
		if strings.Contains(depends, carried) {
			t.Fatalf("the package's Depends is %q and still asks the distribution for %s", depends, carried)
		}
	}
}

// Every unit the package installs starts a program the package installs.
func TestThePackageShipsEveryProgramItsUnitsStart(t *testing.T) {
	for _, target := range packageTargets {
		shipped, errorValue := shippedProgramNames(target.Architecture)
		if errorValue != nil {
			t.Fatalf("%s: %v", target.Architecture, errorValue)
		}
		shippedPaths := map[string]bool{
			blueclaw.CompanyPackagePreparePath:                  true,
			blueclaw.LinuxCompanyHostLayout().DataServicePath(): true,
		}
		for _, name := range shipped {
			shippedPaths[blueclaw.CompanyPackageBinaryPath(name)] = true
		}
		for _, unit := range blueclaw.CompanyPackageUnits() {
			for _, line := range strings.Split(unit.Contents, "\n") {
				if !strings.HasPrefix(line, "ExecStart=") {
					continue
				}
				program := strings.Fields(strings.TrimPrefix(line, "ExecStart="))[0]
				if !shippedPaths[program] {
					t.Fatalf("the %s package installs no %s, and %s starts it",
						target.Architecture, program, unit.FileName())
				}
			}
		}
	}
}

// dpkg keeps a conffile's local edits across upgrades, and overwrites everything else.
// Anything an operator is expected to change has to be one.
func TestTheOperatorSettingsFileIsTheOnlyConfigurationFile(t *testing.T) {
	contents := contentsFor([]packagedFile{
		{SourcePath: "/tmp/a", Destination: "/usr/bin/one", Mode: 0o755},
		{SourcePath: "/tmp/b", Destination: blueclaw.CompanyHostSettingsPath, Mode: 0o644, IsConfiguration: true},
	})
	configurationFiles := []string{}
	for _, content := range contents {
		if strings.HasPrefix(content.Type, files.TypeConfig) {
			configurationFiles = append(configurationFiles, content.Destination)
		}
	}
	if len(configurationFiles) != 1 || configurationFiles[0] != blueclaw.CompanyHostSettingsPath {
		t.Fatalf("the configuration files are %v and the only one should be %s",
			configurationFiles, blueclaw.CompanyHostSettingsPath)
	}
}

// The state root holds a company's identity on the plane. A directory dpkg creates
// world-readable would hand it to every account on the box.
func TestTheStateRootIsPrivate(t *testing.T) {
	for _, directory := range ownedDirectories() {
		if directory.Destination != blueclaw.CompanyHostStateRoot && directory.Destination != blueclaw.CompanyHostCompaniesRoot {
			continue
		}
		if directory.Mode != 0o700 {
			t.Fatalf("%s is created %o and holds what signs a message as a person", directory.Destination, directory.Mode)
		}
	}
}

// The helper is setuid root. If the package ever stops saying so, the agent silently
// loses the ability to act as the person who asked.
func TestTheHelperIsShippedSetuidWhereTheRuntimeLooksForIt(t *testing.T) {
	for _, program := range packagedGoPrograms() {
		if program.Name != blueclaw.POSIXHelperProgramName {
			continue
		}
		if program.Mode&os.ModeSetuid == 0 {
			t.Fatalf("%s is shipped %o rather than setuid", program.Name, program.Mode)
		}
		if program.InstalledPath() != blueclaw.CompanyHostPOSIXHelperPath {
			t.Fatalf("the package installs the helper at %s and the rendered runtime names %s", program.InstalledPath(), blueclaw.CompanyHostPOSIXHelperPath)
		}
		return
	}
	t.Fatal("the package ships no POSIX helper, so the agent can act as nobody")
}

// The package is named after a command, and `internkim install` is the only way
// a box gets a company. A package that ships every daemon and not that command
// installs a machine nobody can finish setting up.
func TestThePackageShipsTheControlCommand(t *testing.T) {
	installed := map[string]bool{}
	for _, program := range packagedGoPrograms() {
		installed[program.InstalledPath()] = true
	}
	controlPath := blueclaw.CompanyPackageBinaryPath(blueclaw.CompanyPackageName)
	if !installed[controlPath] {
		t.Fatalf("the package installs no %s", controlPath)
	}
}

// A postinst that cannot do its job must fail naming what it was doing. A box with the
// package installed and no blueclaw user runs nothing and reports itself installed.
func TestThePostInstallRefusesEveryStepItCannotComplete(t *testing.T) {
	script := maintainerScript(debianPackageFormat, postInstallScript)
	for _, mustRefuse := range []string{
		"systemd-sysusers could not create the service accounts",
		"systemd-tmpfiles could not create the directories",
		"systemd did not reload",
		"could not enable $unit",
	} {
		if !strings.Contains(script, mustRefuse) {
			t.Fatalf("the postinst does not refuse over %q; it would leave a box that answers dpkg and runs nothing", mustRefuse)
		}
	}
	if !strings.Contains(script, "exit 1") {
		t.Fatal("the postinst never exits non-zero, so dpkg would call a broken install successful")
	}
}

func TestThePostInstallBuildsTheConversionEnvironmentBeforeItRestartsTheServices(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		script := maintainerScript(format, postInstallScript)
		restart := strings.Index(script, "systemctl restart")
		for _, command := range blueclaw.LinuxCompanyHostLayout().PythonSetupCommands() {
			position := strings.Index(script, shellWords(command.Arguments)+" || refuse")
			if position < 0 || position > restart {
				t.Fatalf("the %s postinst does not %s before it restarts the services:\n%s", format.Name, command.Purpose, script)
			}
		}
	}
}

func TestAnEnvironmentTheInstallCannotFetchFailsTheInstallNamingIt(t *testing.T) {
	root := t.TempDir()
	layout := blueclaw.CompanyHostLayout{BinaryRoot: filepath.Join(root, "bin"), LibraryRoot: filepath.Join(root, "lib")}
	if errorValue := os.MkdirAll(layout.BinaryRoot, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	unreachable := "#!/bin/sh\necho 'error: Failed to download cpython-" + blueclaw.HostPythonVersion + "' >&2\nexit 2\n"
	if errorValue := os.WriteFile(layout.BinaryPath(blueclaw.PackageResolverName), []byte(unreachable), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	script := "set -e\nrefuse() {\n  echo \"internkim: $1\" >&2\n  exit 1\n}\n" + hostSetupLines(layout.PythonSetupCommands()) + "\necho reached-the-services\n"
	command := exec.Command("/bin/sh", "-c", script)
	output, errorValue := command.CombinedOutput()
	if errorValue == nil || strings.Contains(string(output), "reached-the-services") {
		t.Fatalf("an install that could not fetch the interpreter carried on:\n%s", output)
	}
	for _, named := range []string{"Failed to download", "could not install CPython " + blueclaw.HostPythonVersion} {
		if !strings.Contains(string(output), named) {
			t.Fatalf("the failed install does not say %q:\n%s", named, output)
		}
	}
}

// Purge removes the configuration and keeps the state, which is not what purge usually
// means, so the message that says why is part of the contract.
func TestPurgeKeepsTheCompanyAndSaysSo(t *testing.T) {
	script := maintainerScript(debianPackageFormat, postRemoveScript)
	if !strings.Contains(script, "rm -rf "+blueclaw.CompanyHostConfigurationRoot) {
		t.Fatalf("purge does not remove %s", blueclaw.CompanyHostConfigurationRoot)
	}
	deletion := "sudo rm -rf " + strings.Join(keptStatePaths(), " ")
	if !strings.Contains(script, deletion) {
		t.Fatalf("purge keeps the company without naming the command that deletes it, %q", deletion)
	}
	for _, line := range strings.Split(script, "\n") {
		command := strings.TrimSpace(line)
		for _, path := range keptStatePaths() {
			if strings.HasPrefix(command, "rm ") && strings.Contains(command, path) {
				t.Fatalf("purge runs %q, and nothing in %s can be recovered", command, path)
			}
		}
	}
}

func TestRemovalNamesThePostgresqlRepositoryInstallShAddedAndKeepsIt(t *testing.T) {
	script := maintainerScript(debianPackageFormat, postRemoveScript)
	removal := "sudo rm -f " + strings.Join(postgresqlRepositoryPaths, " ") + " && sudo apt-get update"
	if !strings.Contains(script, removal) {
		t.Fatalf("removal does not name the command that takes PostgreSQL's repository away, %q:\n%s", removal, script)
	}
	for _, line := range strings.Split(script, "\n") {
		command := strings.TrimSpace(line)
		for _, path := range postgresqlRepositoryPaths {
			if strings.HasPrefix(command, "rm ") && strings.Contains(command, path) {
				t.Fatalf("removal runs %q, and the PostgreSQL that made the company's database updates from it", command)
			}
		}
	}
}

func TestThePostgresqlRepositoryPathsAreTheOnesInstallShWrites(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Join("..", "..", "web", "static", "install.sh"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, path := range postgresqlRepositoryPaths {
		if !strings.Contains(string(document), `="`+path+`"`) {
			t.Errorf("install.sh does not write %s, so removal would name a file nobody wrote", path)
		}
	}
}

func TestUpgradeAndRemovalTakeTheDeviceUsersSync(t *testing.T) {
	removal := "rm -f " + strings.Join(deviceUsersSyncPaths(), " ")
	for _, script := range []packageScript{postInstallScript, preRemoveScript} {
		rendered := maintainerScript(debianPackageFormat, script)
		if !strings.Contains(rendered, removal) {
			t.Fatalf("%s leaves the device users sync the first release wrote, %q", script, removal)
		}
		for _, unitName := range deviceUsersSyncUnitNames() {
			resetAt := strings.Index(rendered, "systemctl reset-failed ")
			if resetAt < strings.Index(rendered, removal) || !strings.Contains(rendered[resetAt:], unitName) {
				t.Fatalf("%s removes %s but leaves its failed record, so `systemctl list-units --state=failed` names a unit nothing ships", script, unitName)
			}
		}
	}
}

// An upgrade that unpacks a new binary and leaves the old process serving is the
// failure AGENTS.md is written around: dpkg's version, the file's mtime and apt's
// output all move, and the only thing that did not is the thing that matters. Step 5
// of the install rig reads the admin gateway's build id across an upgrade and caught
// this; here is the cheap half.
func TestConfiguringThePackageRestartsRatherThanStarts(t *testing.T) {
	script := maintainerScript(debianPackageFormat, postInstallScript)
	if strings.Contains(script, "systemctl start ") {
		t.Fatal("configuring the package starts its units, which is a no-op for a unit already " +
			"running, so an upgrade leaves the old process serving the new version's files")
	}
	if !strings.Contains(script, "systemctl restart ") {
		t.Fatal("configuring the package neither starts nor restarts its units")
	}
}

func TestPreRemoveStopsEveryUnitTheInstallStarted(t *testing.T) {
	script := maintainerScript(debianPackageFormat, preRemoveScript)
	for _, unit := range blueclaw.CompanyPackageUnits() {
		if !strings.Contains(script, unit.FileName()) {
			t.Fatalf("removing the package leaves %s running", unit.FileName())
		}
	}
}

// The admin gateway's health answer is the only place an upgrade can be seen to have
// moved the running process rather than the file on disk, and a package built without
// these answers `unknown` for both. The install rig asks the guest; this is the cheap
// tripwire that fails before a twenty-minute run has to.
func TestThePackagedBinariesCarryABuildIdentity(t *testing.T) {
	stamped := admindStampFlags("0.0.7", "abc1234")
	for _, required := range []string{
		"internal/admind.BuildID=0.0.7",
		"internal/admind.GitRevision=abc1234",
	} {
		if !strings.Contains(stamped, required) {
			t.Fatalf("a packaged binary built with %q answers `unknown` where %s belongs", stamped, required)
		}
	}
}

func TestTheArchitecturesAreArm64AndAmd64(t *testing.T) {
	if _, errorValue := packageTargetsNamed("riscv64"); errorValue == nil {
		t.Fatal("a package was accepted for an architecture nothing is built for")
	}
	chosen, errorValue := packageTargetsNamed("arm64")
	if errorValue != nil || len(chosen) != 1 || chosen[0].Architecture != "arm64" {
		t.Fatalf("--architecture arm64 chose %v (%v)", chosen, errorValue)
	}
	if len(packageTargets) != 2 {
		t.Fatalf("the package is built for %d architectures and the design names two", len(packageTargets))
	}
}

// tools/native_install_rig.py globs the package it judges out of a directory it names
// itself. The two halves were written on separate branches; this is what keeps them
// pointed at the same place.
func TestTheBuiltPackageLandsWhereTheInstallRigLooks(t *testing.T) {
	runner, readError := os.ReadFile("../../tools/test-native-install")
	if readError != nil {
		t.Skip("the install rig is not in this tree")
	}
	rig, readError := os.ReadFile("../../tools/native_install_rig.py")
	if readError != nil {
		t.Skip("the install rig is not in this tree")
	}
	if !strings.Contains(string(runner), defaultPackageDirectory) {
		t.Fatalf("internkim release packages writes %s and the install rig does not look there; "+
			"the rig would report that no package exists", defaultPackageDirectory)
	}
	if !strings.Contains(string(rig), `PACKAGE_NAME = "`+blueclaw.CompanyPackageName+`"`) {
		t.Fatalf("the install rig globs for a package not named %s", blueclaw.CompanyPackageName)
	}
}
