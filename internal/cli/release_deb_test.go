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
	information := debianPackageInformation(debianTargets[0], "1.2.3", files.Contents{}, nfpm.Scripts{})
	declared := strings.Split(blueclaw.HostDebianDependsLine(), ", ")
	if len(information.Depends) != len(declared) {
		t.Fatalf("the package declares %d dependencies and the host declares %d", len(information.Depends), len(declared))
	}
	for index, dependency := range declared {
		if information.Depends[index] != dependency {
			t.Fatalf("the package depends on %q where the host declares %q", information.Depends[index], dependency)
		}
	}
	if !strings.Contains(blueclaw.HostDebianDependsLine(), "postgresql (>= 14)") {
		t.Fatal("the package does not ask apt for a PostgreSQL, so an install would leave a box with no database")
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
func TestTheHelperIsShippedSetuid(t *testing.T) {
	for _, program := range debGoPrograms() {
		if !strings.HasSuffix(program.Name, "posix-helper") {
			continue
		}
		if program.Mode&os.ModeSetuid == 0 {
			t.Fatalf("%s is shipped %o rather than setuid", program.Name, program.Mode)
		}
		return
	}
	t.Fatal("the package ships no POSIX helper, so the agent can act as nobody")
}

// A postinst that cannot do its job must fail naming what it was doing. A box with the
// package installed and no blueclaw user runs nothing and reports itself installed.
func TestThePostInstallRefusesEveryStepItCannotComplete(t *testing.T) {
	script := debPostInstallScript()
	for _, mustRefuse := range []string{
		"could not create the " + blueclaw.BlueclawUser + " user",
		"could not create the " + blueclaw.RelayUserName + " user",
		"could not create " + blueclaw.CompanyHostStateRoot,
		"could not make " + blueclaw.CompanyPackageBinaryPath("blueclaw-posix-helper") + " setuid",
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

func TestPreRemoveStopsEveryUnitTheInstallStarted(t *testing.T) {
	script := debPreRemoveScript()
	for _, unit := range blueclaw.CompanyPackageUnits() {
		if !strings.Contains(script, unit.FileName()) {
			t.Fatalf("removing the package leaves %s running", unit.FileName())
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
