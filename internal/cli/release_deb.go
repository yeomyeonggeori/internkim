package cli

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/goreleaser/nfpm/v2"
	_ "github.com/goreleaser/nfpm/v2/deb"
	"github.com/goreleaser/nfpm/v2/files"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The company host as one Debian package. Everything it declares is read from
// internal/runtime/blueclaw: the dependency line from HostDebianDependsLine, the units
// from CompanyPackageUnits, the paths from the same constants those units name. Nothing
// about the package is written twice, and nothing about it is written in YAML — the
// declaration goes straight into nfpm.Info, so there is no configuration file to drift
// away from the code that would have generated it.
const (
	debPackageDescription = `internkim company host

The agent, the messenger relay and the screen a company signs into, as systemd
services on an ordinary Debian machine. The package installs the programs and
the units; ` + "`internkim install`" + ` gives it a company to run, and until it has one
every unit stays inactive rather than restarting into a failure.`

	// tools/native_install_rig.py looks here for the package it judges, so the two
	// halves meet without either naming the other.
	debDefaultOutputDirectory = ".artifacts/native-package"

	debPayloadCacheDirectory = ".dependency/host-payload"
	debReleaseLicense        = "Apache-2.0"
)

type debianTarget struct {
	DebianArchitecture string
	GoArchitecture     string
	BunTarget          string
	ELFMachine         string
}

var debianTargets = []debianTarget{
	{DebianArchitecture: "arm64", GoArchitecture: "arm64", BunTarget: "bun-linux-arm64", ELFMachine: "aarch64"},
	{DebianArchitecture: "amd64", GoArchitecture: "amd64", BunTarget: "bun-linux-x64", ELFMachine: "x86-64"},
}

// debPackagedFile is one path the package owns.
type debPackagedFile struct {
	SourcePath      string
	Destination     string
	Mode            os.FileMode
	IsConfiguration bool
	IsDirectoryTree bool
}

func runReleaseDeb(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	version := firstNonEmptyString(commandArgumentValue(arguments, "--version", ""), debVersionFromRepository(repositoryRootPath))
	outputDirectory := firstNonEmptyString(commandArgumentValue(arguments, "--out", ""), filepath.Join(repositoryRootPath, debDefaultOutputDirectory))
	requested := commandArgumentValue(arguments, "--architecture", "")
	targets, errorValue := debTargetsNamed(requested)
	if errorValue != nil {
		return errorValue
	}
	for _, target := range targets {
		builtPath, errorValue := buildDebianPackage(repositoryRootPath, target, version, outputDirectory, os.Stdout)
		if errorValue != nil {
			return errorValue
		}
		fmt.Fprintf(os.Stdout, "built %s\n", builtPath)
	}
	return nil
}

func debTargetsNamed(requested string) ([]debianTarget, error) {
	if requested == "" {
		return debianTargets, nil
	}
	chosen := []debianTarget{}
	for _, name := range strings.Split(requested, ",") {
		name = strings.TrimSpace(name)
		found := false
		for _, target := range debianTargets {
			if target.DebianArchitecture == name {
				chosen = append(chosen, target)
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("no package is built for %q; the architectures are arm64 and amd64", name)
		}
	}
	return chosen, nil
}

// debVersionFromRepository turns the commit into something dpkg orders. A release
// that names --version gets that instead.
func debVersionFromRepository(repositoryRootPath string) string {
	return "0.0.0+" + time.Now().UTC().Format("20060102") + "." + shortRevision(gitRevision(repositoryRootPath))
}

func buildDebianPackage(repositoryRootPath string, target debianTarget, version string, outputDirectory string, output io.Writer) (string, error) {
	stagingPath, errorValue := os.MkdirTemp("", "internkim-deb-*")
	if errorValue != nil {
		return "", errorValue
	}
	defer os.RemoveAll(stagingPath)
	contents, errorValue := debPackageContents(repositoryRootPath, target, stagingPath, output)
	if errorValue != nil {
		return "", errorValue
	}
	scripts, errorValue := writeDebMaintainerScripts(stagingPath)
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := os.MkdirAll(outputDirectory, 0o755); errorValue != nil {
		return "", errorValue
	}
	packagePath := filepath.Join(outputDirectory, fmt.Sprintf("%s_%s_%s.deb", blueclaw.CompanyPackageName, version, target.DebianArchitecture))
	return packagePath, writeDebianPackage(debianPackageInformation(target, version, contents, scripts), packagePath)
}

func debianPackageInformation(target debianTarget, version string, contents files.Contents, scripts nfpm.Scripts) *nfpm.Info {
	return &nfpm.Info{
		Name:        blueclaw.CompanyPackageName,
		Arch:        target.DebianArchitecture,
		Platform:    "linux",
		Version:     version,
		Section:     blueclaw.CompanyPackageSection,
		Priority:    "optional",
		Maintainer:  blueclaw.CompanyPackageMaintainer,
		Description: debPackageDescription,
		Vendor:      blueclaw.CompanyPackageVendor,
		Homepage:    blueclaw.CompanyPackageHomepage,
		License:     debReleaseLicense,
		Overridables: nfpm.Overridables{
			Depends:  strings.Split(blueclaw.HostDebianDependsLine(), ", "),
			Contents: contents,
			Scripts:  scripts,
		},
	}
}

func writeDebianPackage(information *nfpm.Info, packagePath string) error {
	packager, errorValue := nfpm.Get("deb")
	if errorValue != nil {
		return errorValue
	}
	information = nfpm.WithDefaults(information)
	if errorValue := nfpm.Validate(information); errorValue != nil {
		return errorValue
	}
	file, errorValue := os.Create(packagePath)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	return packager.Package(information, file)
}

func debPackageContents(repositoryRootPath string, target debianTarget, stagingPath string, output io.Writer) (files.Contents, error) {
	packaged := []debPackagedFile{}
	programs, errorValue := buildDebPrograms(repositoryRootPath, target, stagingPath, output)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, programs...)
	rendered, errorValue := writeDebRenderedFiles(stagingPath)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, rendered...)
	carried, errorValue := debCarriedTrees(repositoryRootPath)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, carried...)
	return debContentsFor(packaged), nil
}

func debContentsFor(packaged []debPackagedFile) files.Contents {
	contents := files.Contents{}
	for _, file := range packaged {
		content := &files.Content{
			Source:      file.SourcePath,
			Destination: file.Destination,
			FileInfo:    &files.ContentFileInfo{Owner: "root", Group: "root", Mode: file.Mode},
		}
		switch {
		case file.IsConfiguration:
			content.Type = files.TypeConfigNoReplace
		case file.IsDirectoryTree:
			content.Type = files.TypeTree
		}
		contents = append(contents, content)
	}
	for _, directory := range debOwnedDirectories() {
		contents = append(contents, &files.Content{
			Destination: directory.Destination,
			Type:        files.TypeDir,
			FileInfo:    &files.ContentFileInfo{Owner: "root", Group: "root", Mode: directory.Mode},
		})
	}
	return contents
}

// debOwnedDirectories are the trees §5 puts on disk. The state root is 0700 because
// what lands under it is a company's identity on the plane and the seed that signs a
// message under a person's own name, and the bucket directory is 0750 root:root to
// keep attachments away from the unprivileged task users sharing the box. The bucket
// is a directory because versitygw's posix backend serves one as a bucket; it is what
// `mc mb` used to do.
func debOwnedDirectories() []debPackagedFile {
	return []debPackagedFile{
		{Destination: blueclaw.CompanyHostStateRoot, Mode: 0o700},
		{Destination: blueclaw.CompanyHostCompaniesRoot, Mode: 0o700},
		{Destination: blueclaw.CompanyHostMediaRootPath, Mode: 0o750},
		{Destination: blueclaw.CompanyHostMediaBucketPath, Mode: 0o750},
		{Destination: blueclaw.CompanyHostConfigurationRoot, Mode: 0o755},
		{Destination: blueclaw.CompanyPackageHelperRoot, Mode: 0o755},
	}
}

// debGoPrograms are this repository's own binaries and blueclaw's, each named by the
// constant the units use, so a rename cannot leave the package shipping a program no
// unit starts.
type debGoProgram struct {
	Name       string
	ModuleRoot string
	Package    string
	Mode       os.FileMode
}

func debGoPrograms() []debGoProgram {
	return []debGoProgram{
		{Name: blueclaw.CapabilitydName, Package: "./cmd/internkim-capabilityd", Mode: 0o755},
		{Name: blueclaw.AdmindName, Package: "./cmd/internkim-admind", Mode: 0o755},
		{Name: blueclaw.MaildName, Package: "./cmd/internkim-maild", Mode: 0o755},
		{Name: blueclaw.BuzzMigrateName, Package: "./cmd/buzz-migrate", Mode: 0o755},
		{Name: blueclaw.BlueclawName, ModuleRoot: blueclaw.BlueclawSubmodulePath, Package: "./cmd/blueclaw", Mode: 0o755},
		// The helper is what lets an unprivileged blueclaw act as the person who
		// asked. It is the one setuid file in the package.
		{Name: "blueclaw-posix-helper", ModuleRoot: blueclaw.BlueclawSubmodulePath, Package: "./cmd/blueclaw-posix-helper", Mode: os.ModeSetuid | 0o755},
	}
}

type debBunProgram struct {
	Name           string
	WorkingRoot    string
	EntryPoint     string
	InstallFilter  string
	InstallWorking string
}

func debBunPrograms() []debBunProgram {
	return []debBunProgram{
		{
			Name:           blueclaw.RelayName,
			WorkingRoot:    "host/relay",
			EntryPoint:     "relay.ts",
			InstallWorking: "host/relay",
		},
		{
			Name:           blueclaw.ChatdName,
			WorkingRoot:    blueclaw.BlueclawSubmodulePath + "/chatd",
			EntryPoint:     "src/main.ts",
			InstallFilter:  "./chatd",
			InstallWorking: blueclaw.BlueclawSubmodulePath,
		},
	}
}

func buildDebPrograms(repositoryRootPath string, target debianTarget, stagingPath string, output io.Writer) ([]debPackagedFile, error) {
	packaged := []debPackagedFile{}
	for _, program := range debGoPrograms() {
		builtPath := filepath.Join(stagingPath, program.Name)
		if errorValue := crossCompileDebProgram(repositoryRootPath, program, target, builtPath); errorValue != nil {
			return nil, errorValue
		}
		fmt.Fprintf(output, "  compiled %s for %s\n", program.Name, target.DebianArchitecture)
		packaged = append(packaged, debPackagedFile{
			SourcePath:  builtPath,
			Destination: blueclaw.CompanyPackageBinaryPath(program.Name),
			Mode:        program.Mode,
		})
	}
	for _, program := range debBunPrograms() {
		builtPath := filepath.Join(stagingPath, program.Name)
		if errorValue := compileDebBunProgram(repositoryRootPath, program, target, builtPath); errorValue != nil {
			return nil, errorValue
		}
		fmt.Fprintf(output, "  compiled %s for %s\n", program.Name, target.DebianArchitecture)
		packaged = append(packaged, debPackagedFile{
			SourcePath:  builtPath,
			Destination: blueclaw.CompanyPackageBinaryPath(program.Name),
			Mode:        0o755,
		})
	}
	messenger, errorValue := debMessengerPrograms(repositoryRootPath, target)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, messenger...)
	vendored, errorValue := debVendoredPrograms(repositoryRootPath, target, stagingPath, output)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, vendored...)
	packaged = append(packaged, debPackagedFile{
		SourcePath:  filepath.Join(repositoryRootPath, "tools", blueclaw.RenderCompanyRuntimeName),
		Destination: blueclaw.CompanyPackageBinaryPath(blueclaw.RenderCompanyRuntimeName),
		Mode:        0o755,
	})
	return packaged, nil
}

func crossCompileDebProgram(repositoryRootPath string, program debGoProgram, target debianTarget, outputPath string) error {
	command := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", outputPath, program.Package)
	command.Dir = filepath.Join(repositoryRootPath, program.ModuleRoot)
	command.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+target.GoArchitecture, "CGO_ENABLED=0")
	commandOutput, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("compile %s for %s: %s", program.Name, target.DebianArchitecture, strings.TrimSpace(string(commandOutput)))
	}
	return nil
}

func compileDebBunProgram(repositoryRootPath string, program debBunProgram, target debianTarget, outputPath string) error {
	installArguments := []string{"install", "--frozen-lockfile"}
	if program.InstallFilter != "" {
		installArguments = append(installArguments, "--filter", program.InstallFilter)
	}
	install := exec.Command("bun", installArguments...)
	install.Dir = filepath.Join(repositoryRootPath, program.InstallWorking)
	if commandOutput, errorValue := install.CombinedOutput(); errorValue != nil {
		return fmt.Errorf("resolve %s dependencies: %s", program.Name, strings.TrimSpace(string(commandOutput)))
	}
	build := exec.Command("bun", "build", "--compile", "--target="+target.BunTarget, "--outfile", outputPath, program.EntryPoint)
	build.Dir = filepath.Join(repositoryRootPath, program.WorkingRoot)
	if commandOutput, errorValue := build.CombinedOutput(); errorValue != nil {
		return fmt.Errorf("compile %s for %s: %s", program.Name, target.DebianArchitecture, strings.TrimSpace(string(commandOutput)))
	}
	return nil
}

// The messenger is built by tools/prepare-buzz-relay from a pinned upstream revision
// and lands in .dependency/buzz-relay as linux binaries. The package carries those
// files; it does not build Rust. A package for an architecture that artifact does not
// cover is refused rather than shipped without a messenger.
func debMessengerPrograms(repositoryRootPath string, target debianTarget) ([]debPackagedFile, error) {
	packaged := []debPackagedFile{}
	for _, name := range []string{blueclaw.BuzzRelayName, blueclaw.BuzzAdminName} {
		sourcePath := filepath.Join(repositoryRootPath, blueclaw.BuzzRelayArtifactPath, name)
		if errorValue := requireELFFor(sourcePath, target, name); errorValue != nil {
			return nil, errorValue
		}
		packaged = append(packaged, debPackagedFile{
			SourcePath:  sourcePath,
			Destination: blueclaw.CompanyPackageBinaryPath(name),
			Mode:        0o755,
		})
	}
	return packaged, nil
}

func requireELFFor(sourcePath string, target debianTarget, name string) error {
	information, errorValue := os.Stat(sourcePath)
	if errorValue != nil {
		return fmt.Errorf("%s is not at %s; build it with tools/prepare-buzz-relay: %w", name, sourcePath, errorValue)
	}
	if information.Size() == 0 {
		return fmt.Errorf("%s at %s is empty; rebuild it with tools/prepare-buzz-relay", name, sourcePath)
	}
	machine, errorValue := elfMachineOf(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	if machine != target.ELFMachine {
		return fmt.Errorf(
			"%s at %s is a %s binary and this package is %s; tools/prepare-buzz-relay builds one architecture at a time",
			name, sourcePath, machine, target.DebianArchitecture)
	}
	return nil
}

// elfMachineOf reads e_machine out of the header rather than shelling out to file(1),
// which is not on every build host.
func elfMachineOf(sourcePath string) (string, error) {
	file, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return "", errorValue
	}
	defer file.Close()
	header := make([]byte, 20)
	if _, errorValue := io.ReadFull(file, header); errorValue != nil {
		return "", fmt.Errorf("read the header of %s: %w", sourcePath, errorValue)
	}
	if string(header[:4]) != "\x7fELF" {
		return "", fmt.Errorf("%s is not a Linux binary", sourcePath)
	}
	switch uint16(header[18]) | uint16(header[19])<<8 {
	case 0xB7:
		return "aarch64", nil
	case 0x3E:
		return "x86-64", nil
	default:
		return "", fmt.Errorf("%s is built for a machine this package does not target", sourcePath)
	}
}

func debVendoredPrograms(repositoryRootPath string, target debianTarget, stagingPath string, output io.Writer) ([]debPackagedFile, error) {
	downloads, errorValue := blueclaw.HostPayloadDownloads(target.DebianArchitecture)
	if errorValue != nil {
		return nil, errorValue
	}
	cachePath := filepath.Join(repositoryRootPath, debPayloadCacheDirectory)
	if errorValue := os.MkdirAll(cachePath, 0o755); errorValue != nil {
		return nil, errorValue
	}
	packaged := []debPackagedFile{}
	for _, download := range downloads {
		downloadedPath, errorValue := fetchPinnedPayload(download, cachePath, output)
		if errorValue != nil {
			return nil, errorValue
		}
		programPath := downloadedPath
		if download.PathInsideArchive != "" {
			programPath, errorValue = extractFromGzippedTar(downloadedPath, download.PathInsideArchive, filepath.Join(stagingPath, download.ProgramName))
			if errorValue != nil {
				return nil, errorValue
			}
		}
		packaged = append(packaged, debPackagedFile{
			SourcePath:  programPath,
			Destination: blueclaw.CompanyPackageBinaryPath(download.ProgramName),
			Mode:        0o755,
		})
	}
	return packaged, nil
}

func fetchPinnedPayload(download blueclaw.HostPayloadDownload, cachePath string, output io.Writer) (string, error) {
	cachedPath := filepath.Join(cachePath, filepath.Base(download.URL))
	if checksum, errorValue := checksumOf(cachedPath); errorValue == nil && checksum == download.SHA256 {
		return cachedPath, nil
	}
	fmt.Fprintf(output, "  fetching %s %s\n", download.ProgramName, download.Version)
	response, errorValue := http.Get(download.URL)
	if errorValue != nil {
		return "", fmt.Errorf("fetch %s %s: %w", download.ProgramName, download.Version, errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s %s: %s answered %s", download.ProgramName, download.Version, download.URL, response.Status)
	}
	file, errorValue := os.Create(cachedPath)
	if errorValue != nil {
		return "", errorValue
	}
	if _, errorValue := io.Copy(file, response.Body); errorValue != nil {
		file.Close()
		return "", errorValue
	}
	file.Close()
	checksum, errorValue := checksumOf(cachedPath)
	if errorValue != nil {
		return "", errorValue
	}
	if checksum != download.SHA256 {
		os.Remove(cachedPath)
		return "", fmt.Errorf(
			"%s %s from %s hashes to %s and the pin says %s",
			download.ProgramName, download.Version, download.URL, checksum, download.SHA256)
	}
	return cachedPath, nil
}

func checksumOf(path string) (string, error) {
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return "", errorValue
	}
	defer file.Close()
	digest := sha256.New()
	if _, errorValue := io.Copy(digest, file); errorValue != nil {
		return "", errorValue
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func extractFromGzippedTar(archivePath string, wantedPath string, outputPath string) (string, error) {
	file, errorValue := os.Open(archivePath)
	if errorValue != nil {
		return "", errorValue
	}
	defer file.Close()
	decompressed, errorValue := gzip.NewReader(file)
	if errorValue != nil {
		return "", errorValue
	}
	defer decompressed.Close()
	archive := tar.NewReader(decompressed)
	for {
		header, errorValue := archive.Next()
		if errorValue == io.EOF {
			return "", fmt.Errorf("%s holds no %s", archivePath, wantedPath)
		}
		if errorValue != nil {
			return "", errorValue
		}
		if filepath.Clean(header.Name) != wantedPath {
			continue
		}
		extracted, errorValue := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
		if errorValue != nil {
			return "", errorValue
		}
		defer extracted.Close()
		if _, errorValue := io.Copy(extracted, archive); errorValue != nil {
			return "", errorValue
		}
		return outputPath, nil
	}
}

func writeDebRenderedFiles(stagingPath string) ([]debPackagedFile, error) {
	packaged := []debPackagedFile{}
	for _, unit := range blueclaw.CompanyPackageUnits() {
		unitPath := filepath.Join(stagingPath, unit.FileName())
		if errorValue := os.WriteFile(unitPath, []byte(unit.Contents), 0o644); errorValue != nil {
			return nil, errorValue
		}
		packaged = append(packaged, debPackagedFile{
			SourcePath:  unitPath,
			Destination: unit.InstalledPath(),
			Mode:        0o644,
		})
	}
	preparePath := filepath.Join(stagingPath, "prepare-company-host")
	if errorValue := os.WriteFile(preparePath, []byte(blueclaw.CompanyHostPrepareScript()), 0o755); errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, debPackagedFile{
		SourcePath:  preparePath,
		Destination: blueclaw.CompanyPackagePreparePath,
		Mode:        0o755,
	})
	settingsPath := filepath.Join(stagingPath, "company-host.env")
	if errorValue := os.WriteFile(settingsPath, []byte(blueclaw.CompanyHostSettingsFile()), 0o644); errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, debPackagedFile{
		SourcePath:      settingsPath,
		Destination:     blueclaw.CompanyHostSettingsPath,
		Mode:            0o644,
		IsConfiguration: true,
	})
	return packaged, nil
}

// debCarriedTrees are the files the image copies in unchanged. A missing one is a
// refusal: a box whose skills directory is empty answers and does nothing.
func debCarriedTrees(repositoryRootPath string) ([]debPackagedFile, error) {
	carried := []debPackagedFile{
		{
			SourcePath:      filepath.Join(repositoryRootPath, ".dependency/internkim-plugin/skills"),
			Destination:     blueclaw.CompanyPackageSkillsPath,
			Mode:            0o755,
			IsDirectoryTree: true,
		},
		{
			SourcePath:      filepath.Join(repositoryRootPath, blueclaw.BlueclawSubmodulePath, "migrations"),
			Destination:     blueclaw.CompanyPackageMigrationPath,
			Mode:            0o755,
			IsDirectoryTree: true,
		},
		{
			SourcePath:  filepath.Join(repositoryRootPath, "host/runtime.template.json"),
			Destination: blueclaw.CompanyPackageTemplatePath,
			Mode:        0o644,
		},
	}
	for _, file := range carried {
		if _, errorValue := os.Stat(file.SourcePath); errorValue != nil {
			return nil, fmt.Errorf("the package carries %s and it is not at %s: %w", file.Destination, file.SourcePath, errorValue)
		}
	}
	return carried, nil
}

func writeDebMaintainerScripts(stagingPath string) (nfpm.Scripts, error) {
	scripts := map[string]string{
		"postinst": debPostInstallScript(),
		"prerm":    debPreRemoveScript(),
		"postrm":   debPostRemoveScript(),
	}
	written := nfpm.Scripts{}
	for name, body := range scripts {
		path := filepath.Join(stagingPath, name)
		if errorValue := os.WriteFile(path, []byte(body), 0o755); errorValue != nil {
			return nfpm.Scripts{}, errorValue
		}
		switch name {
		case "postinst":
			written.PostInstall = path
		case "prerm":
			written.PreRemove = path
		case "postrm":
			written.PostRemove = path
		}
	}
	return written, nil
}

// debShippedProgramNames is every program the package puts in /usr/bin, derived from
// the same four declarations buildDebPrograms builds from, so a unit that starts a
// program nothing produces is a test failure rather than a box that does not come up.
func debShippedProgramNames(debianArchitecture string) ([]string, error) {
	names := []string{blueclaw.RenderCompanyRuntimeName, blueclaw.BuzzRelayName, blueclaw.BuzzAdminName}
	for _, program := range debGoPrograms() {
		names = append(names, program.Name)
	}
	for _, program := range debBunPrograms() {
		names = append(names, program.Name)
	}
	downloads, errorValue := blueclaw.HostPayloadDownloads(debianArchitecture)
	if errorValue != nil {
		return nil, errorValue
	}
	for _, download := range downloads {
		names = append(names, download.ProgramName)
	}
	return names, nil
}
