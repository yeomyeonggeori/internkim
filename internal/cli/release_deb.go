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

	// xz, because the payload carries a CPython tree and gzip leaves a third more
	// on the wire. dpkg has read xz since 1.15, which predates every distribution
	// the package is for.
	debPayloadCompression = "xz"
	debReleaseLicense     = "Apache-2.0"
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
	IsSymbolicLink  bool
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
	built, errorValue := buildLinuxPackages(repositoryRootPath, target, version, outputDirectory, []linuxPackageFormat{debianPackageFormat}, output)
	if errorValue != nil {
		return "", errorValue
	}
	return built[0], nil
}

func debianPackageInformation(target debianTarget, version string, contents files.Contents, scripts nfpm.Scripts) *nfpm.Info {
	return linuxPackageInformation(debianPackageFormat, target, version, contents, scripts)
}

func writeDebianPackage(information *nfpm.Info, packagePath string) error {
	return writeLinuxPackage(debianPackageFormat, information, packagePath)
}

func debPackageContents(repositoryRootPath string, target debianTarget, version string, stagingPath string, output io.Writer) (files.Contents, error) {
	packaged := []debPackagedFile{}
	programs, errorValue := buildDebPrograms(repositoryRootPath, target, version, stagingPath, output)
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
	interpreter, errorValue := buildDocumentInterpreter(repositoryRootPath, target, stagingPath, output)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, interpreter...)
	font, errorValue := debCarriedFont(repositoryRootPath, output)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, font...)
	return debContentsFor(packaged), nil
}

// debCarriedFont is the Hangul face and the license that has to travel with it.
func debCarriedFont(repositoryRootPath string, output io.Writer) ([]debPackagedFile, error) {
	cachePath := filepath.Join(repositoryRootPath, debPayloadCacheDirectory)
	if errorValue := os.MkdirAll(cachePath, 0o755); errorValue != nil {
		return nil, errorValue
	}
	destinations := []string{blueclaw.CompanyPackageDocumentFontPath, blueclaw.CompanyPackageDocumentFontLicensePath}
	downloads := blueclaw.DocumentFontDownloads()
	packaged := []debPackagedFile{}
	for index, download := range downloads {
		downloadedPath, errorValue := fetchPinnedPayload(blueclaw.HostPayloadDownload{
			ProgramName: download.Name, Version: "pinned", URL: download.URL, SHA256: download.SHA256,
		}, cachePath, output)
		if errorValue != nil {
			return nil, errorValue
		}
		packaged = append(packaged, debPackagedFile{SourcePath: downloadedPath, Destination: destinations[index], Mode: 0o644})
	}
	return packaged, nil
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
		case file.IsSymbolicLink:
			content.Type = files.TypeSymlink
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
		{Destination: blueclaw.CompanyHostStateRoot, Mode: blueclaw.CompanyHostStateRootMode},
		{Destination: blueclaw.CompanyHostCompaniesRoot, Mode: blueclaw.CompanyHostStateRootMode},
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
	Name        string
	ModuleRoot  string
	Package     string
	Mode        os.FileMode
	Destination string
}

// InstalledPath is /usr/bin unless the program says otherwise, because that is
// where a program a person types belongs and where dpkg is allowed to write.
func (program debGoProgram) InstalledPath() string {
	if program.Destination != "" {
		return program.Destination
	}
	return blueclaw.CompanyPackageBinaryPath(program.Name)
}

func debGoPrograms() []debGoProgram {
	return []debGoProgram{
		// The control command the package is named after. `internkim install`
		// gives this box a company; until it has one, every unit's condition is
		// unmet and nothing runs.
		{Name: blueclaw.CompanyPackageName, Package: "./cmd/internkim-host", Mode: 0o755},
		{Name: blueclaw.CapabilitydName, Package: "./cmd/internkim-capabilityd", Mode: 0o755},
		{Name: blueclaw.AdmindName, Package: "./cmd/internkim-admind", Mode: 0o755},
		{Name: blueclaw.MaildName, Package: "./cmd/internkim-maild", Mode: 0o755},
		{Name: blueclaw.BuzzMigrateName, Package: "./cmd/buzz-migrate", Mode: 0o755},
		{Name: blueclaw.BlueclawName, ModuleRoot: blueclaw.BlueclawSubmodulePath, Package: "./cmd/blueclaw", Mode: 0o755},
		// The helper is what lets an unprivileged blueclaw act as the person who
		// asked. It is the one setuid file in the package, and the one program
		// that does not go in /usr/bin: nobody runs it from a shell, and the
		// runtime document the prepare script renders names where it is.
		{
			Name:        blueclaw.POSIXHelperProgramName,
			ModuleRoot:  blueclaw.BlueclawSubmodulePath,
			Package:     "./cmd/blueclaw-posix-helper",
			Mode:        os.ModeSetuid | 0o755,
			Destination: blueclaw.CompanyHostPOSIXHelperPath,
		},
	}
}

// debSymbolicLinks keeps the name the published bare binary had working for the
// one release in which a machine may still be carrying it.
func debSymbolicLinks() []debPackagedFile {
	return []debPackagedFile{
		{
			SourcePath:     blueclaw.CompanyPackageBinaryPath(blueclaw.CompanyPackageName),
			Destination:    blueclaw.CompanyPackageBinaryPath(companyHostBinaryName),
			IsSymbolicLink: true,
		},
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

func buildDebPrograms(repositoryRootPath string, target debianTarget, version string, stagingPath string, output io.Writer) ([]debPackagedFile, error) {
	packaged := []debPackagedFile{}
	for _, program := range debGoPrograms() {
		builtPath := filepath.Join(stagingPath, program.Name)
		if errorValue := crossCompileDebProgram(repositoryRootPath, program, target, version, builtPath); errorValue != nil {
			return nil, errorValue
		}
		fmt.Fprintf(output, "  compiled %s for %s\n", program.Name, target.DebianArchitecture)
		packaged = append(packaged, debPackagedFile{
			SourcePath:  builtPath,
			Destination: program.InstalledPath(),
			Mode:        program.Mode,
		})
	}
	packaged = append(packaged, debSymbolicLinks()...)
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
	if errorValue := requirePackagedProgramsFit(packaged, target); errorValue != nil {
		return nil, errorValue
	}
	return packaged, nil
}

// Two packages built from one tree carry one revision, so the build id is the package
// version: it is what dpkg moved, and it is what says whether the process answering
// after an upgrade is the process the upgrade installed.
func crossCompileDebProgram(repositoryRootPath string, program debGoProgram, target debianTarget, version string, outputPath string) error {
	stamped := "-s -w " + admindStampFlags(version, releaseBinaryRevision(repositoryRootPath))
	command := exec.Command("go", "build", "-trimpath", "-ldflags", stamped, "-o", outputPath, program.Package)
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
// and lands in a directory per architecture, which blueclaw.BuzzRelayArtifactPathFor
// names. The package carries those files; it does not build Rust. A package for an
// architecture whose directory is missing or holds another one's binaries is refused
// rather than shipped without a messenger.
func debMessengerPrograms(repositoryRootPath string, target debianTarget) ([]debPackagedFile, error) {
	packaged := []debPackagedFile{}
	for _, name := range []string{blueclaw.BuzzRelayName, blueclaw.BuzzAdminName} {
		sourcePath := filepath.Join(repositoryRootPath, blueclaw.BuzzRelayArtifactPathFor(target.DebianArchitecture), name)
		if errorValue := requireMessengerBinary(sourcePath, name, target); errorValue != nil {
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
			programPath, errorValue = extractProgram(downloadedPath, download.PathInsideArchive, filepath.Join(stagingPath, download.ProgramName))
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

// Upstream decides what kind of archive it publishes, and the file name is where it
// says so: moli, versitygw and uv ship gzipped tarballs and bun ships a zip.
func extractProgram(archivePath string, wantedPath string, outputPath string) (string, error) {
	switch {
	case strings.HasSuffix(archivePath, ".tar.gz"):
		return extractFromGzippedTar(archivePath, wantedPath, outputPath)
	case strings.HasSuffix(archivePath, ".zip"):
		return extractPinnedFromZip(archivePath, wantedPath, outputPath)
	}
	return "", fmt.Errorf("%s is an archive kind this package does not know how to open", archivePath)
}

// The zip reader this shares with the unpackaged path matches on an entry's own name
// rather than its path, which is exact here because the archive's bytes were verified
// against the pin before this opened it.
func extractPinnedFromZip(archivePath string, wantedPath string, outputPath string) (string, error) {
	archive, errorValue := os.Open(archivePath)
	if errorValue != nil {
		return "", errorValue
	}
	defer archive.Close()
	held, errorValue := archive.Stat()
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := extractFromZip(archive, held.Size(), outputPath, filepath.Base(wantedPath)); errorValue != nil {
		return "", errorValue
	}
	return outputPath, nil
}

func copyProgram(held io.Reader, outputPath string) (string, error) {
	extracted, errorValue := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if errorValue != nil {
		return "", errorValue
	}
	defer extracted.Close()
	if _, errorValue := io.Copy(extracted, held); errorValue != nil {
		return "", errorValue
	}
	return outputPath, nil
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
		return copyProgram(archive, outputPath)
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
	dataServicePath := filepath.Join(stagingPath, blueclaw.CompanyHostDataServiceProgramName)
	if errorValue := os.WriteFile(dataServicePath, []byte(blueclaw.CompanyHostDataServiceScript()), blueclaw.CompanyHostDataServiceMode); errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, debPackagedFile{
		SourcePath:  dataServicePath,
		Destination: blueclaw.DebianCompanyHostLayout().DataServicePath(),
		Mode:        blueclaw.CompanyHostDataServiceMode,
	})
	for _, declaration := range []struct {
		name        string
		destination string
		contents    string
	}{
		{"declared-accounts", blueclaw.CompanyPackageSysusersPath, blueclaw.CompanyHostSysusersFile()},
		{"declared-directories", blueclaw.CompanyPackageTmpfilesPath, blueclaw.CompanyHostTmpfilesFile()},
	} {
		declarationPath := filepath.Join(stagingPath, declaration.name)
		if errorValue := os.WriteFile(declarationPath, []byte(declaration.contents), 0o644); errorValue != nil {
			return nil, errorValue
		}
		packaged = append(packaged, debPackagedFile{SourcePath: declarationPath, Destination: declaration.destination, Mode: 0o644})
	}
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

// The document conversion imports wheels, and a wheel is built for one operating system,
// one processor and one Python minor version. The package carries the interpreter the
// wheels are built for, so the build resolves them inside a throwaway guest of the
// target architecture with that interpreter mounted at the path it will have on a
// customer's machine: the venv's pyvenv.cfg and its bin/python link then name a place
// that exists there. The guest runs the oldest glibc the package supports, which is
// what makes a wheel tagged manylinux for it load on every newer one.
const (
	documentVenvBuildImage   = "ubuntu:22.04"
	documentVenvBuildRuntime = "container"
)

func buildDocumentInterpreter(repositoryRootPath string, target debianTarget, stagingPath string, output io.Writer) ([]debPackagedFile, error) {
	if _, errorValue := exec.LookPath(documentVenvBuildRuntime); errorValue != nil {
		return nil, fmt.Errorf(
			"the document interpreter is resolved inside a %s guest of the target architecture, and `%s` is not on PATH. "+
				"Build the release on a machine that has it, or on a %s machine of that architecture",
			documentVenvBuildImage, documentVenvBuildRuntime, target.DebianArchitecture)
	}
	workPath := filepath.Join(stagingPath, "document-venv-build")
	if errorValue := os.MkdirAll(workPath, 0o755); errorValue != nil {
		return nil, errorValue
	}
	interpreterPath, errorValue := carryDocumentInterpreterFor(repositoryRootPath, target, workPath, output)
	if errorValue != nil {
		return nil, errorValue
	}
	requirements, errorValue := documentConversionRequirements(repositoryRootPath)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := os.WriteFile(filepath.Join(workPath, "requirements.txt"), []byte(requirements), 0o644); errorValue != nil {
		return nil, errorValue
	}
	resolverPath := filepath.Join(stagingPath, blueclaw.PackageResolverName)
	if _, errorValue := os.Stat(resolverPath); errorValue != nil {
		return nil, fmt.Errorf("the package resolver is staged at %s before the interpreter is built: %w", resolverPath, errorValue)
	}
	if errorValue := os.Link(resolverPath, filepath.Join(workPath, blueclaw.PackageResolverName)); errorValue != nil {
		return nil, errorValue
	}

	fmt.Fprintf(output, "  resolving the document interpreter for %s\n", target.DebianArchitecture)
	build := exec.Command(documentVenvBuildRuntime, "run", "--rm",
		"--platform", "linux/"+target.DebianArchitecture,
		"--volume", workPath+":/work",
		"--volume", interpreterPath+":"+blueclaw.CompanyPackageInterpreterPath+":ro",
		documentVenvBuildImage, "sh", "-c", documentVenvBuildScript())
	build.Stdout = output
	build.Stderr = output
	if errorValue := build.Run(); errorValue != nil {
		return nil, fmt.Errorf("resolving the document interpreter for %s: %w", target.DebianArchitecture, errorValue)
	}
	builtPath := filepath.Join(workPath, "document-venv")
	if _, errorValue := os.Lstat(filepath.Join(builtPath, "bin", "python")); errorValue != nil {
		return nil, fmt.Errorf("the resolved interpreter has no %s/bin/python", builtPath)
	}
	for _, tree := range []string{interpreterPath, builtPath} {
		if errorValue := requireELFTreeFits(tree, target); errorValue != nil {
			return nil, errorValue
		}
	}
	return []debPackagedFile{
		{SourcePath: interpreterPath, Destination: blueclaw.CompanyPackageInterpreterPath, Mode: 0o755, IsDirectoryTree: true},
		{SourcePath: builtPath, Destination: blueclaw.CompanyPackageDocumentVenvPath, Mode: 0o755, IsDirectoryTree: true},
	}, nil
}

// debSkippedInterpreterDirectories is what of the interpreter's tree the package leaves
// behind: share/ holds terminfo entries and manual pages, and two terminfo names differ
// only in case, which a case-insensitive build machine cannot hold and which no document
// skill reads.
var debSkippedInterpreterDirectories = []string{"share"}

// carryDocumentInterpreterFor unpacks the pinned CPython for this target, the same pin
// the Mac keg reads.
func carryDocumentInterpreterFor(repositoryRootPath string, target debianTarget, workPath string, output io.Writer) (string, error) {
	payloadTarget, errorValue := blueclaw.HostPayloadTargetForDebianArchitecture(target.DebianArchitecture)
	if errorValue != nil {
		return "", errorValue
	}
	pin, errorValue := blueclaw.DocumentInterpreterForTarget(payloadTarget)
	if errorValue != nil {
		return "", errorValue
	}
	cachePath := filepath.Join(repositoryRootPath, debPayloadCacheDirectory)
	if errorValue := os.MkdirAll(cachePath, 0o755); errorValue != nil {
		return "", errorValue
	}
	archivePath, errorValue := fetchPinnedPayload(blueclaw.HostPayloadDownload{
		ProgramName: "python", Version: pin.Version, URL: pin.URL, SHA256: pin.SHA256,
	}, cachePath, output)
	if errorValue != nil {
		return "", errorValue
	}
	interpreterPath := filepath.Join(workPath, pin.DirectoryInsideArchive)
	if errorValue := extractGzippedTarTree(archivePath, pin.DirectoryInsideArchive, interpreterPath, debSkippedInterpreterDirectories...); errorValue != nil {
		return "", errorValue
	}
	return interpreterPath, nil
}

func documentVenvBuildScript() string {
	interpreter := blueclaw.CompanyPackageInterpreterPath + "/bin/python" + blueclaw.DocumentInterpreterMinor
	resolver := "/work/" + blueclaw.PackageResolverName
	return strings.Join([]string{
		"set -eu",
		"export DEBIAN_FRONTEND=noninteractive",
		"apt-get update -qq >/dev/null",
		"apt-get install -y -qq --no-install-recommends ca-certificates >/dev/null",
		resolver + " venv --python " + interpreter + " " + blueclaw.CompanyPackageDocumentVenvPath + " >/dev/null",
		resolver + " pip install --quiet --python " + blueclaw.CompanyPackageDocumentPythonPath + " --requirements /work/requirements.txt",
		blueclaw.CompanyPackageDocumentPythonPath + " -c " + shellQuoted(blueclaw.DocumentModulesTheConversionImports()),
		"rm -rf /work/document-venv",
		"cp -a " + blueclaw.CompanyPackageDocumentVenvPath + " /work/document-venv",
	}, "\n")
}

func shellQuoted(text string) string {
	return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
}

func documentConversionRequirements(repositoryRootPath string) (string, error) {
	path := filepath.Join(repositoryRootPath, "assets/document-conversion/requirements.txt")
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return "", fmt.Errorf("the document interpreter is resolved from %s: %w", path, errorValue)
	}
	return strings.TrimSpace(string(document)) + "\n", nil
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

// debShippedProgramNames is every program the package puts in /usr/bin, derived from
// the same four declarations buildDebPrograms builds from, so a unit that starts a
// program nothing produces is a test failure rather than a box that does not come up.
func debShippedProgramNames(debianArchitecture string) ([]string, error) {
	names := []string{blueclaw.RenderCompanyRuntimeName, blueclaw.BuzzRelayName, blueclaw.BuzzAdminName}
	for _, program := range debGoPrograms() {
		if program.Destination != "" {
			continue
		}
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
