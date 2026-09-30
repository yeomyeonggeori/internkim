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

	"github.com/goreleaser/nfpm/v2/files"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The company host's payload, which every package format carries. Everything it
// declares is read from internal/runtime/blueclaw: the dependencies from
// HostPackageDependsFor, the units from CompanyPackageUnits, the paths from the same constants those units name. Nothing
// about the package is written twice, and nothing about it is written in YAML — the
// declaration goes straight into nfpm.Info, so there is no configuration file to drift
// away from the code that would have generated it.
const (
	packageDescription = `internkim company host

The agent, the messenger relay and the screen a company signs into, as systemd
services on an ordinary Linux machine. The package installs the programs and
the units; ` + "`internkim install`" + ` gives it a company to run, and until it has one
every unit stays inactive rather than restarting into a failure.`

	// tools/native_install_rig.py looks here for the package it judges, so the two
	// halves meet without either naming the other.
	defaultPackageDirectory = ".artifacts/native-package"

	payloadCacheDirectory = ".dependency/host-payload"

	documentConversionLockPath = "assets/document-conversion/requirements.txt"

	// dpkg has read xz since 1.15, which predates every distribution the package
	// is for.
	debPayloadCompression = "xz"
	packageLicense        = "Apache-2.0"
)

type packageTarget struct {
	Architecture   string
	GoArchitecture string
	BunTarget      string
	ELFMachine     string
}

var packageTargets = []packageTarget{
	{Architecture: "arm64", GoArchitecture: "arm64", BunTarget: "bun-linux-arm64", ELFMachine: "aarch64"},
	{Architecture: "amd64", GoArchitecture: "amd64", BunTarget: "bun-linux-x64", ELFMachine: "x86-64"},
}

// packagedFile is one path the package owns.
type packagedFile struct {
	SourcePath      string
	Destination     string
	Mode            os.FileMode
	IsConfiguration bool
	IsDirectoryTree bool
	IsSymbolicLink  bool
}

func packageTargetsNamed(requested string) ([]packageTarget, error) {
	if requested == "" {
		return packageTargets, nil
	}
	chosen := []packageTarget{}
	for _, name := range strings.Split(requested, ",") {
		name = strings.TrimSpace(name)
		found := false
		for _, target := range packageTargets {
			if target.Architecture == name {
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

// packageVersionFromRepository turns the commit into something every package manager orders. A release
// that names --version gets that instead.
func packageVersionFromRepository(repositoryRootPath string) string {
	return "0.0.0+" + time.Now().UTC().Format("20060102") + "." + shortRevision(gitRevision(repositoryRootPath))
}

func packageContents(repositoryRootPath string, target packageTarget, version string, stagingPath string, output io.Writer) (files.Contents, error) {
	packaged := []packagedFile{}
	programs, errorValue := buildPackagedPrograms(repositoryRootPath, target, version, stagingPath, output)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, programs...)
	rendered, errorValue := writeRenderedFiles(stagingPath)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, rendered...)
	carried, errorValue := carriedTrees(repositoryRootPath)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, carried...)
	font, errorValue := carriedFont(repositoryRootPath, output)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, font...)
	return contentsFor(packaged), nil
}

// carriedFont is the Hangul face and the license that has to travel with it.
func carriedFont(repositoryRootPath string, output io.Writer) ([]packagedFile, error) {
	destinations := []string{blueclaw.CompanyPackageDocumentFontPath, blueclaw.CompanyPackageDocumentFontLicensePath}
	downloads := blueclaw.DocumentFontDownloads()
	packaged := []packagedFile{}
	for index, download := range downloads {
		downloadedPath, errorValue := fetchPinnedPayload(repositoryRootPath, blueclaw.HostPayloadDownload{
			ProgramName: download.Name, Version: "pinned", URL: download.URL, SHA256: download.SHA256,
		}, output)
		if errorValue != nil {
			return nil, errorValue
		}
		packaged = append(packaged, packagedFile{SourcePath: downloadedPath, Destination: destinations[index], Mode: 0o644})
	}
	return packaged, nil
}

func contentsFor(packaged []packagedFile) files.Contents {
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
	for _, directory := range ownedDirectories() {
		contents = append(contents, &files.Content{
			Destination: directory.Destination,
			Type:        files.TypeDir,
			FileInfo:    &files.ContentFileInfo{Owner: "root", Group: "root", Mode: directory.Mode},
		})
	}
	return contents
}

// ownedDirectories are the trees §5 puts on disk. The state root is 0700 because
// what lands under it is a company's identity on the plane and the seed that signs a
// message under a person's own name, and the bucket directory is 0750 root:root to
// keep attachments away from the unprivileged task users sharing the box. The bucket
// is a directory because versitygw's posix backend serves one as a bucket; it is what
// `mc mb` used to do.
func ownedDirectories() []packagedFile {
	return []packagedFile{
		{Destination: blueclaw.CompanyHostStateRoot, Mode: blueclaw.CompanyHostStateRootMode},
		{Destination: blueclaw.CompanyHostCompaniesRoot, Mode: blueclaw.CompanyHostStateRootMode},
		{Destination: blueclaw.CompanyHostMediaRootPath, Mode: 0o750},
		{Destination: blueclaw.CompanyHostMediaBucketPath, Mode: 0o750},
		{Destination: blueclaw.CompanyHostConfigurationRoot, Mode: 0o755},
		{Destination: blueclaw.CompanyPackageHelperRoot, Mode: 0o755},
	}
}

// packagedGoPrograms are this repository's own binaries and blueclaw's, each named by the
// constant the units use, so a rename cannot leave the package shipping a program no
// unit starts.
type packagedGoProgram struct {
	Name        string
	ModuleRoot  string
	Package     string
	Mode        os.FileMode
	Destination string
}

// InstalledPath is /usr/bin unless the program says otherwise, because that is
// where a program a person types belongs and where a package manager is allowed to write.
func (program packagedGoProgram) InstalledPath() string {
	if program.Destination != "" {
		return program.Destination
	}
	return blueclaw.CompanyPackageBinaryPath(program.Name)
}

func packagedGoPrograms() []packagedGoProgram {
	return []packagedGoProgram{
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

// packagedSymbolicLinks keeps the name the published bare binary had working for the
// one release in which a machine may still be carrying it.
func packagedSymbolicLinks() []packagedFile {
	return []packagedFile{
		{
			SourcePath:     blueclaw.CompanyPackageBinaryPath(blueclaw.CompanyPackageName),
			Destination:    blueclaw.CompanyPackageBinaryPath(companyHostBinaryName),
			IsSymbolicLink: true,
		},
	}
}

type packagedBunProgram struct {
	Name           string
	WorkingRoot    string
	EntryPoint     string
	InstallFilter  string
	InstallWorking string
}

func packagedBunPrograms() []packagedBunProgram {
	return []packagedBunProgram{
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

func buildPackagedPrograms(repositoryRootPath string, target packageTarget, version string, stagingPath string, output io.Writer) ([]packagedFile, error) {
	packaged := []packagedFile{}
	for _, program := range packagedGoPrograms() {
		builtPath := filepath.Join(stagingPath, program.Name)
		if errorValue := crossCompilePackagedProgram(repositoryRootPath, program, target, version, builtPath); errorValue != nil {
			return nil, errorValue
		}
		fmt.Fprintf(output, "  compiled %s for %s\n", program.Name, target.Architecture)
		packaged = append(packaged, packagedFile{
			SourcePath:  builtPath,
			Destination: program.InstalledPath(),
			Mode:        program.Mode,
		})
	}
	packaged = append(packaged, packagedSymbolicLinks()...)
	for _, program := range packagedBunPrograms() {
		builtPath := filepath.Join(stagingPath, program.Name)
		if errorValue := compilePackagedBunProgram(repositoryRootPath, program, target, builtPath); errorValue != nil {
			return nil, errorValue
		}
		fmt.Fprintf(output, "  compiled %s for %s\n", program.Name, target.Architecture)
		packaged = append(packaged, packagedFile{
			SourcePath:  builtPath,
			Destination: blueclaw.CompanyPackageBinaryPath(program.Name),
			Mode:        0o755,
		})
	}
	messenger, errorValue := messengerPrograms(repositoryRootPath, target)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, messenger...)
	vendored, errorValue := vendoredPrograms(repositoryRootPath, target, stagingPath, output)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, vendored...)
	packaged = append(packaged, packagedFile{
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
// version: it is what the package manager moved, and it is what says whether the process answering
// after an upgrade is the process the upgrade installed.
func crossCompilePackagedProgram(repositoryRootPath string, program packagedGoProgram, target packageTarget, version string, outputPath string) error {
	stamped := "-s -w " + admindStampFlags(version, releaseBinaryRevision(repositoryRootPath))
	command := exec.Command("go", "build", "-trimpath", "-ldflags", stamped, "-o", outputPath, program.Package)
	command.Dir = filepath.Join(repositoryRootPath, program.ModuleRoot)
	command.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+target.GoArchitecture, "CGO_ENABLED=0")
	commandOutput, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("compile %s for %s: %s", program.Name, target.Architecture, strings.TrimSpace(string(commandOutput)))
	}
	return nil
}

func compilePackagedBunProgram(repositoryRootPath string, program packagedBunProgram, target packageTarget, outputPath string) error {
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
		return fmt.Errorf("compile %s for %s: %s", program.Name, target.Architecture, strings.TrimSpace(string(commandOutput)))
	}
	return nil
}

// The messenger is built by tools/prepare-buzz-relay from a pinned upstream revision
// and lands in a directory per architecture, which blueclaw.BuzzRelayArtifactPathFor
// names. The package carries those files; it does not build Rust. A package for an
// architecture whose directory is missing or holds another one's binaries is refused
// rather than shipped without a messenger.
func messengerPrograms(repositoryRootPath string, target packageTarget) ([]packagedFile, error) {
	packaged := []packagedFile{}
	for _, name := range []string{blueclaw.BuzzRelayName, blueclaw.BuzzAdminName} {
		sourcePath := filepath.Join(repositoryRootPath, blueclaw.BuzzRelayArtifactPathFor(target.Architecture), name)
		if errorValue := requireMessengerBinary(sourcePath, name, target); errorValue != nil {
			return nil, errorValue
		}
		packaged = append(packaged, packagedFile{
			SourcePath:  sourcePath,
			Destination: blueclaw.CompanyPackageBinaryPath(name),
			Mode:        0o755,
		})
	}
	return packaged, nil
}

func vendoredPrograms(repositoryRootPath string, target packageTarget, stagingPath string, output io.Writer) ([]packagedFile, error) {
	downloads, errorValue := blueclaw.HostPayloadDownloads(target.Architecture)
	if errorValue != nil {
		return nil, errorValue
	}
	packaged := []packagedFile{}
	for _, download := range downloads {
		programPath, errorValue := fetchVendoredProgram(repositoryRootPath, download, stagingPath, output)
		if errorValue != nil {
			return nil, errorValue
		}
		packaged = append(packaged, packagedFile{
			SourcePath:  programPath,
			Destination: blueclaw.CompanyPackageBinaryPath(download.ProgramName),
			Mode:        0o755,
		})
	}
	return packaged, nil
}

// fetchVendoredProgram is the pinned program's path: the download itself, or
// the program extracted from it into directoryPath.
func fetchVendoredProgram(repositoryRootPath string, download blueclaw.HostPayloadDownload, directoryPath string, output io.Writer) (string, error) {
	downloadedPath, errorValue := fetchPinnedPayload(repositoryRootPath, download, output)
	if errorValue != nil || download.PathInsideArchive == "" {
		return downloadedPath, errorValue
	}
	return extractProgram(downloadedPath, download.PathInsideArchive, filepath.Join(directoryPath, download.ProgramName))
}

func fetchPinnedPayload(repositoryRootPath string, download blueclaw.HostPayloadDownload, output io.Writer) (string, error) {
	cachePath := filepath.Join(repositoryRootPath, payloadCacheDirectory)
	if errorValue := os.MkdirAll(cachePath, 0o755); errorValue != nil {
		return "", errorValue
	}
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

// The zip reader this shares with binary_download.go matches on an entry's own name
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

func writeRenderedFiles(stagingPath string) ([]packagedFile, error) {
	packaged := []packagedFile{}
	for _, unit := range blueclaw.CompanyPackageUnits() {
		unitPath := filepath.Join(stagingPath, unit.FileName())
		if errorValue := os.WriteFile(unitPath, []byte(unit.Contents), 0o644); errorValue != nil {
			return nil, errorValue
		}
		packaged = append(packaged, packagedFile{
			SourcePath:  unitPath,
			Destination: unit.InstalledPath(),
			Mode:        0o644,
		})
	}
	preparePath := filepath.Join(stagingPath, "prepare-company-host")
	if errorValue := os.WriteFile(preparePath, []byte(blueclaw.CompanyHostPrepareScript()), 0o755); errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, packagedFile{
		SourcePath:  preparePath,
		Destination: blueclaw.CompanyPackagePreparePath,
		Mode:        0o755,
	})
	dataServicePath := filepath.Join(stagingPath, blueclaw.CompanyHostDataServiceProgramName)
	if errorValue := os.WriteFile(dataServicePath, []byte(blueclaw.CompanyHostDataServiceScript()), blueclaw.CompanyHostDataServiceMode); errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, packagedFile{
		SourcePath:  dataServicePath,
		Destination: blueclaw.LinuxCompanyHostLayout().DataServicePath(),
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
		packaged = append(packaged, packagedFile{SourcePath: declarationPath, Destination: declaration.destination, Mode: 0o644})
	}
	settingsPath := filepath.Join(stagingPath, "company-host.env")
	if errorValue := os.WriteFile(settingsPath, []byte(blueclaw.CompanyHostSettingsFile()), 0o644); errorValue != nil {
		return nil, errorValue
	}
	packaged = append(packaged, packagedFile{
		SourcePath:      settingsPath,
		Destination:     blueclaw.CompanyHostSettingsPath,
		Mode:            0o644,
		IsConfiguration: true,
	})
	return packaged, nil
}

// carriedTrees are the files the image copies in unchanged. A missing one is a
// refusal: a box whose skills directory is empty answers and does nothing.
func carriedTrees(repositoryRootPath string) ([]packagedFile, error) {
	carried := []packagedFile{
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
		{
			SourcePath:  filepath.Join(repositoryRootPath, documentConversionLockPath),
			Destination: blueclaw.LinuxCompanyHostLayout().DocumentRequirementsPath(),
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

// shippedProgramNames is every program the package puts in /usr/bin, derived from
// the same four declarations buildPackagedPrograms builds from, so a unit that starts a
// program nothing produces is a test failure rather than a box that does not come up.
func shippedProgramNames(architecture string) ([]string, error) {
	names := []string{blueclaw.RenderCompanyRuntimeName, blueclaw.BuzzRelayName, blueclaw.BuzzAdminName}
	for _, program := range packagedGoPrograms() {
		if program.Destination != "" {
			continue
		}
		names = append(names, program.Name)
	}
	for _, program := range packagedBunPrograms() {
		names = append(names, program.Name)
	}
	downloads, errorValue := blueclaw.HostPayloadDownloads(architecture)
	if errorValue != nil {
		return nil, errorValue
	}
	for _, download := range downloads {
		names = append(names, download.ProgramName)
	}
	return names, nil
}
