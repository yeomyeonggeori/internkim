package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/goreleaser/nfpm/v2"
	"github.com/goreleaser/nfpm/v2/files"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The dependency list lives once, in host_dependencies.go. A package that spelled its
// own would drift from the image the same declaration builds.
func TestThePackageDependsOnTheDeclaredListAndNothingElse(t *testing.T) {
	information := debianPackageInformation(debianTargets[0], "1.2.3", "", files.Contents{}, nfpm.Scripts{})
	declared := strings.Split(blueclaw.HostDebianDependsLine(""), ", ")
	if len(information.Depends) != len(declared) {
		t.Fatalf("the package declares %d dependencies and the host declares %d", len(information.Depends), len(declared))
	}
	for index, dependency := range declared {
		if information.Depends[index] != dependency {
			t.Fatalf("the package depends on %q where the host declares %q", information.Depends[index], dependency)
		}
	}
	if !strings.Contains(blueclaw.HostDebianDependsLine(""), "postgresql (>= 14)") {
		t.Fatal("the package does not ask apt for a PostgreSQL, so an install would leave a box with no database")
	}
}

// The package carries a venv of wheels built for one Python minor version, so it has
// to name the python3 it can be installed beside. The version comes out of the venv
// the build just resolved, which is the only value that cannot disagree with the
// wheels; a literal here would be a second account of the same fact.
func TestThePackageNamesThePythonItsInterpreterWasResolvedAgainst(t *testing.T) {
	information := debianPackageInformation(debianTargets[0], "1.2.3", "3.13.5", files.Contents{}, nfpm.Scripts{})
	bounded := strings.Join(information.Depends, ", ")
	for _, required := range []string{"python3 (>= 3.13)", "python3 (<< 3.14)"} {
		if !strings.Contains(bounded, required) {
			t.Fatalf("the package's Depends is %q and does not carry %q, so apt would install it "+
				"beside a python3 that imports none of the wheels it ships", bounded, required)
		}
	}
	unbounded := strings.Join(debianPackageInformation(debianTargets[0], "1.2.3", "", files.Contents{}, nfpm.Scripts{}).Depends, ", ")
	if strings.Contains(unbounded, "python3 (") {
		t.Fatalf("a package built with no interpreter bounds python3 anyway: %q", unbounded)
	}
}

// Every unit the package installs starts a program the package installs.
func TestThePackageShipsEveryProgramItsUnitsStart(t *testing.T) {
	for _, target := range debianTargets {
		shipped, errorValue := debShippedProgramNames(target.DebianArchitecture)
		if errorValue != nil {
			t.Fatalf("%s: %v", target.DebianArchitecture, errorValue)
		}
		shippedPaths := map[string]bool{blueclaw.CompanyPackagePreparePath: true}
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
						target.DebianArchitecture, program, unit.FileName())
				}
			}
		}
	}
}

// dpkg keeps a conffile's local edits across upgrades, and overwrites everything else.
// Anything an operator is expected to change has to be one.
func TestTheOperatorSettingsFileIsTheOnlyConfigurationFile(t *testing.T) {
	contents := debContentsFor([]debPackagedFile{
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
	for _, directory := range debOwnedDirectories() {
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
	for _, program := range debGoPrograms() {
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
func TestThePackageShipsTheControlCommandAndTheNameTheBareBinaryHad(t *testing.T) {
	installed := map[string]bool{}
	for _, program := range debGoPrograms() {
		installed[program.InstalledPath()] = true
	}
	controlPath := blueclaw.CompanyPackageBinaryPath(blueclaw.CompanyPackageName)
	if !installed[controlPath] {
		t.Fatalf("the package installs no %s", controlPath)
	}
	for _, link := range debSymbolicLinks() {
		if link.Destination == blueclaw.CompanyPackageBinaryPath(companyHostBinaryName) && link.SourcePath == controlPath {
			return
		}
	}
	t.Fatalf("nothing keeps %s working for a machine that still has it", companyHostBinaryName)
}

// A postinst that cannot do its job must fail naming what it was doing. A box with the
// package installed and no blueclaw user runs nothing and reports itself installed.
func TestThePostInstallRefusesEveryStepItCannotComplete(t *testing.T) {
	script := debPostInstallScript()
	for _, mustRefuse := range []string{
		"could not create the " + blueclaw.BlueclawUser + " user",
		"could not create the " + blueclaw.RelayUserName + " user",
		"could not create " + blueclaw.CompanyHostStateRoot,
		"could not make " + blueclaw.CompanyHostPOSIXHelperPath + " setuid",
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

// Purge removes the configuration and keeps the state, which is not what purge usually
// means, so the message that says why is part of the contract.
func TestPurgeKeepsTheCompanyAndSaysSo(t *testing.T) {
	script := debPostRemoveScript()
	if !strings.Contains(script, "rm -rf "+blueclaw.CompanyHostConfigurationRoot) {
		t.Fatalf("purge does not remove %s", blueclaw.CompanyHostConfigurationRoot)
	}
	if strings.Contains(script, "rm -rf "+blueclaw.CompanyHostStateRoot) {
		t.Fatalf("purge removes %s, and nothing in it can be recovered", blueclaw.CompanyHostStateRoot)
	}
	if !strings.Contains(script, blueclaw.CompanyHostStateRoot+" was kept") {
		t.Fatalf("purge keeps %s without saying so", blueclaw.CompanyHostStateRoot)
	}
}

// An upgrade that unpacks a new binary and leaves the old process serving is the
// failure AGENTS.md is written around: dpkg's version, the file's mtime and apt's
// output all move, and the only thing that did not is the thing that matters. Step 5
// of the install rig reads the admin gateway's build id across an upgrade and caught
// this; here is the cheap half.
func TestConfiguringThePackageRestartsRatherThanStarts(t *testing.T) {
	script := debPostInstallScript()
	if strings.Contains(script, "deb-systemd-invoke start ") {
		t.Fatal("configuring the package starts its units, which is a no-op for a unit already " +
			"running, so an upgrade leaves the old process serving the new version's files")
	}
	if !strings.Contains(script, "deb-systemd-invoke restart ") {
		t.Fatal("configuring the package neither starts nor restarts its units")
	}
}

func TestPreRemoveStopsEveryUnitTheInstallStarted(t *testing.T) {
	script := debPreRemoveScript()
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
	if _, errorValue := debTargetsNamed("riscv64"); errorValue == nil {
		t.Fatal("a package was accepted for an architecture nothing is built for")
	}
	chosen, errorValue := debTargetsNamed("arm64")
	if errorValue != nil || len(chosen) != 1 || chosen[0].DebianArchitecture != "arm64" {
		t.Fatalf("--architecture arm64 chose %v (%v)", chosen, errorValue)
	}
	if len(debianTargets) != 2 {
		t.Fatalf("the package is built for %d architectures and the design names two", len(debianTargets))
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
	if !strings.Contains(string(runner), debDefaultOutputDirectory) {
		t.Fatalf("internkim release deb writes %s and the install rig does not look there; "+
			"the rig would report that no package exists", debDefaultOutputDirectory)
	}
	if !strings.Contains(string(rig), `PACKAGE_NAME = "`+blueclaw.CompanyPackageName+`"`) {
		t.Fatalf("the install rig globs for a package not named %s", blueclaw.CompanyPackageName)
	}
}
