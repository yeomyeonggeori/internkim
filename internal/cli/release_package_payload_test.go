package cli

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
			packageLayout.PrepareScriptPath():   true,
			packageLayout.DataServicePath():     true,
			packageLayout.EmbeddingServerPath(): true,
		}
		for _, name := range shipped {
			shippedPaths[packageLayout.BinaryPath(name)] = true
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
func TestTheHelperIsShippedSetuid(t *testing.T) {
	for _, program := range packagedGoPrograms() {
		if program.Name != blueclaw.POSIXHelperProgramName {
			continue
		}
		if program.Mode&os.ModeSetuid == 0 {
			t.Fatalf("%s is shipped %o rather than setuid", program.Name, program.Mode)
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
	controlPath := packageLayout.BinaryPath(blueclaw.CompanyPackageName)
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
		for _, command := range packageLayout.InstallStepCommands() {
			position := strings.Index(script, shellWords(command.Arguments)+" || refuse")
			if position < 0 || position > restart {
				t.Fatalf("the %s postinst does not %s before it restarts the services:\n%s", format.Name, command.Purpose, script)
			}
		}
	}
}

// Every upgrade runs the postinst, so a release whose skills need something new
// prepares it before the agent comes back, and nobody's first use installs it.
func TestThePostInstallPreparesTheSkillsOnThePythonItJustInstalled(t *testing.T) {
	layout := blueclaw.LinuxCompanyHostLayout()
	for _, format := range linuxPackageFormats() {
		script := maintainerScript(format, postInstallScript)
		preparation := strings.Index(script, shellWords(layout.SkillPreparationCommand().Arguments)+" || refuse")
		if preparation < 0 {
			t.Fatalf("the %s postinst does not prepare the bundled skills:\n%s", format.Name, script)
		}
		for _, command := range layout.PythonSetupCommands() {
			if strings.Index(script, shellWords(command.Arguments)) > preparation {
				t.Fatalf("the %s postinst prepares the skills before it can %s:\n%s", format.Name, command.Purpose, script)
			}
		}
	}
}

func TestRemovalTakesWhatTheSkillsSetupKeptBesideThem(t *testing.T) {
	layout := blueclaw.LinuxCompanyHostLayout()
	script := maintainerScript(debianPackageFormat, postRemoveScript)
	for _, line := range strings.Split(script, "\n") {
		command := strings.TrimSpace(line)
		if strings.HasPrefix(command, "rm -rf "+layout.PythonRoot()+" ") {
			if !slices.Contains(strings.Fields(command), layout.SkillsPath()) {
				t.Fatalf("removal leaves %s, where each skill's setup kept what the package manager does not own: %s", layout.SkillsPath(), command)
			}
			return
		}
	}
	t.Fatalf("removal no longer deletes %s:\n%s", layout.PythonRoot(), script)
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

func TestEveryHostCarriesTheAgentPictureWhereAdmindIsToldToFindIt(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	for _, layout := range []blueclaw.CompanyHostLayout{packageLayout, blueclaw.MacCompanyHostLayout("/opt/homebrew")} {
		carried := slices.IndexFunc(carriedLibraryFiles(layout), func(file packagedFile) bool {
			return file.Destination == layout.AgentProfilePicturePath()
		})
		if carried < 0 {
			t.Fatalf("the host rooted at %s carries no agent picture at %s", layout.LibraryRoot, layout.AgentProfilePicturePath())
		}
		source := filepath.Join(repositoryRootPath, carriedLibraryFiles(layout)[carried].SourcePath)
		if _, errorValue := os.Stat(source); errorValue != nil {
			t.Fatalf("the agent picture the package carries is not at %s: %v", source, errorValue)
		}
		service, isBundled := blueclaw.CompanyHostServiceNamed(layout, blueclaw.AdmindServiceName)
		if !isBundled {
			t.Fatal("the host runs no admind")
		}
		named := slices.Index(service.Command, "-agent-profile-picture")
		if named < 0 || named+1 >= len(service.Command) || service.Command[named+1] != layout.AgentProfilePicturePath() {
			t.Fatalf("admind is started as %v and not told the agent picture is at %s", service.Command, layout.AgentProfilePicturePath())
		}
	}
}

func writeTarball(t *testing.T, entries []tar.Header, contents map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "payload.tar.gz")
	file, errorValue := os.Create(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer file.Close()
	compressed := gzip.NewWriter(file)
	defer compressed.Close()
	archive := tar.NewWriter(compressed)
	defer archive.Close()
	for _, header := range entries {
		header.Size = int64(len(contents[header.Name]))
		if errorValue := archive.WriteHeader(&header); errorValue != nil {
			t.Fatal(errorValue)
		}
		if _, errorValue := archive.Write([]byte(contents[header.Name])); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	return archivePath
}

func TestADirectoryPayloadKeepsItsSymlinksAndModesAndLeavesOtherEntriesBehind(t *testing.T) {
	archivePath := writeTarball(t, []tar.Header{
		{Name: "runtime-1", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "runtime-1/server", Typeflag: tar.TypeReg, Mode: 0o755},
		{Name: "runtime-1/libone.so.1", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "runtime-1/libone.so", Typeflag: tar.TypeSymlink, Linkname: "libone.so.1", Mode: 0o777},
		{Name: "elsewhere/readme", Typeflag: tar.TypeReg, Mode: 0o644},
	}, map[string]string{"runtime-1/server": "program", "runtime-1/libone.so.1": "library", "elsewhere/readme": "x"})
	destinationPath := filepath.Join(t.TempDir(), "runtime")

	if errorValue := extractDirectoryFromGzippedTar(archivePath, "runtime-1", destinationPath); errorValue != nil {
		t.Fatal(errorValue)
	}

	linkTarget, errorValue := os.Readlink(filepath.Join(destinationPath, "libone.so"))
	if errorValue != nil || linkTarget != "libone.so.1" {
		t.Fatalf("the symlink was not kept: %q, %v", linkTarget, errorValue)
	}
	information, errorValue := os.Stat(filepath.Join(destinationPath, "server"))
	if errorValue != nil || information.Mode().Perm() != 0o755 {
		t.Fatalf("the program lost its mode: %v, %v", information, errorValue)
	}
	if _, errorValue := os.Stat(filepath.Join(destinationPath, "readme")); errorValue == nil {
		t.Fatal("an entry outside the pinned directory was extracted")
	}
}

func TestADirectoryPayloadRefusesALinkThatLeavesTheDirectory(t *testing.T) {
	archivePath := writeTarball(t, []tar.Header{
		{Name: "runtime-1/escape", Typeflag: tar.TypeSymlink, Linkname: "../../etc/passwd", Mode: 0o777},
	}, nil)

	errorValue := extractDirectoryFromGzippedTar(archivePath, "runtime-1", filepath.Join(t.TempDir(), "runtime"))

	if errorValue == nil || !strings.Contains(errorValue.Error(), "leaves the directory") {
		t.Fatalf("expected a refusal, got %v", errorValue)
	}
}

func TestADirectoryPayloadThatIsNotInTheArchiveIsAnError(t *testing.T) {
	archivePath := writeTarball(t, []tar.Header{{Name: "other/file", Typeflag: tar.TypeReg, Mode: 0o644}}, nil)

	if errorValue := extractDirectoryFromGzippedTar(archivePath, "runtime-1", filepath.Join(t.TempDir(), "runtime")); errorValue == nil {
		t.Fatal("an archive without the pinned directory extracted nothing and reported success")
	}
}

func TestTheEmbeddingServerIsNotAProgramInTheBinaryRoot(t *testing.T) {
	for _, target := range packageTargets {
		shipped, errorValue := shippedProgramNames(target.Architecture)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if slices.Contains(shipped, blueclaw.EmbeddingServerProgramName) {
			t.Fatalf("%s lists %s as a program in %s, and it is installed under %s", target.Architecture,
				blueclaw.EmbeddingServerProgramName, packageLayout.BinaryRoot, packageLayout.EmbeddingServerDirectory())
		}
	}
}
