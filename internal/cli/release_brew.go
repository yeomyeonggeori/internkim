package cli

import (
	"archive/tar"
	"compress/gzip"
	"debug/macho"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The company host as a Homebrew bottle, built the way the Linux packages are:
// every path, dependency and program name is read from internal/runtime/blueclaw,
// and nothing about the formula is written twice. The bottle, the tarball the
// formula's url names and the formula itself are assets of the same GitHub
// Release as the Linux packages.
//
// Three things differ from the Linux packages, and each is Homebrew's rule rather than a
// choice. The keg holds no service definitions, because the plists carry baked
// environment and are written by `internkim install` once there is a company to
// bake. It holds no setuid helper bit, because a bottle is a tar extracted as an
// ordinary user and extraction drops the bit. And the messenger is built on this
// Mac rather than in a container, because `tools/prepare-buzz-relay
// --target darwin-arm64` is a native build.

const brewMessengerArtifactPath = ".dependency/buzz-relay-darwin-arm64"

// One timestamp for every entry, so two releases cut from one tree publish the
// same bytes and a bottle's checksum is a fact about the tree rather than about
// the minute it was built in.
var homebrewArchiveTime = time.Unix(0, 0).UTC()

// buildHomebrewRelease writes the bottle, the tarball and the formula into the
// release directory. The programs are cross-compiled, but the messenger is a
// native build and the bottle's tag is asked of Homebrew, so it runs on an
// Apple-silicon Mac.
func buildHomebrewRelease(repositoryRootPath string, version string, outputDirectory string, output io.Writer) error {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return fmt.Errorf(
			"the Homebrew bottle is built on an Apple-silicon Mac: its messenger is a native build "+
				"and Homebrew names its tag. This is %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	kegPath, errorValue := os.MkdirTemp("", "internkim-keg-*")
	if errorValue != nil {
		return errorValue
	}
	defer os.RemoveAll(kegPath)
	if errorValue := buildHomebrewKeg(repositoryRootPath, kegPath, version, output); errorValue != nil {
		return errorValue
	}
	bottleTag, errorValue := homebrewBottleTagFor(kegPath)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(outputDirectory, 0o755); errorValue != nil {
		return errorValue
	}
	sourceChecksum, errorValue := writeGzippedTar(filepath.Join(outputDirectory, blueclaw.HomebrewSourceTarballName()), kegPath, "")
	if errorValue != nil {
		return errorValue
	}
	bottleName := blueclaw.HomebrewBottleFileName(version, bottleTag)
	bottleChecksum, errorValue := writeGzippedTar(filepath.Join(outputDirectory, bottleName), kegPath, blueclaw.CompanyPackageName+"/"+version)
	if errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(output, "built %s and %s\n", blueclaw.HomebrewSourceTarballName(), bottleName)
	formula, errorValue := homebrewReleaseFormula(version, sourceChecksum, homebrewBottle(bottleTag, bottleChecksum))
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(filepath.Join(outputDirectory, blueclaw.HomebrewFormulaAssetName()), []byte(formula), 0o644)
}

func homebrewBottle(tag string, checksum string) blueclaw.HomebrewBottle {
	return blueclaw.HomebrewBottle{Tag: tag, Cellar: ":any_skip_relocation", SHA256: checksum}
}

// homebrewReleaseFormula points the formula at the release it is uploaded with:
// Homebrew fetches root_url/<bottle name>, which is the bottle's asset URL.
func homebrewReleaseFormula(version string, sourceChecksum string, bottle blueclaw.HomebrewBottle) (string, error) {
	return blueclaw.HomebrewFormula(blueclaw.HomebrewFormulaRequest{
		Version:          version,
		SourceTarballURL: hostReleaseDownloadURL(version) + "/" + blueclaw.HomebrewSourceTarballName(),
		SourceSHA256:     sourceChecksum,
		BottleRootURL:    hostReleaseDownloadURL(version),
		Bottles:          []blueclaw.HomebrewBottle{bottle},
	})
}

// homebrewBottleTagFor names the bottle after the oldest macOS every program in
// the keg runs on, which Homebrew pours on that version and every later one. A
// tag asked of the machine that builds it would make the release depend on which
// Mac cut it, and a bottle cut on a newer macOS than a customer's is never poured.
func homebrewBottleTagFor(kegPath string) (string, error) {
	minimum, errorValue := kegMinimumMacOSVersion(kegPath)
	if errorValue != nil {
		return "", errorValue
	}
	symbol, errorValue := exec.Command("brew", "ruby", "-e", fmt.Sprintf("puts MacOSVersion.new(%q).to_sym", strconv.Itoa(minimum))).Output()
	if errorValue != nil {
		return "", fmt.Errorf("ask Homebrew what it calls macOS %d: %w", minimum, errorValue)
	}
	name := strings.TrimSpace(string(symbol))
	if name == "" {
		return "", fmt.Errorf("Homebrew has no name for macOS %d", minimum)
	}
	return "arm64_" + name, nil
}

// kegMinimumMacOSVersion is the newest major version any Mach-O program in the
// keg names as its minimum, read from LC_BUILD_VERSION or LC_VERSION_MIN_MACOSX.
// Files that are not Mach-O, such as the skills and the runtime template, name
// none.
func kegMinimumMacOSVersion(kegPath string) (int, error) {
	minimum := 0
	errorValue := filepath.Walk(kegPath, func(path string, information fs.FileInfo, walkError error) error {
		if walkError != nil || !information.Mode().IsRegular() {
			return walkError
		}
		major, errorValue := machOMinimumMacOSVersion(path)
		if errorValue != nil {
			return fmt.Errorf("read the minimum macOS of %s: %w", strings.TrimPrefix(path, kegPath+"/"), errorValue)
		}
		minimum = max(minimum, major)
		return nil
	})
	if errorValue != nil {
		return 0, errorValue
	}
	if minimum == 0 {
		return 0, fmt.Errorf("no program in the keg at %s names the macOS it needs", kegPath)
	}
	return minimum, nil
}

const (
	loadCommandBuildVersion  = 0x32
	loadCommandVersionMinMac = 0x24
	machOPlatformMacOS       = 1
)

// machOMinimumMacOSVersion answers 0 for a file that is not Mach-O, and for a
// universal binary reads its arm64 slice, the one an Apple-silicon Mac runs.
func machOMinimumMacOSVersion(path string) (int, error) {
	universal, errorValue := macho.OpenFat(path)
	if errorValue == nil {
		defer universal.Close()
		for _, architecture := range universal.Arches {
			if architecture.Cpu == macho.CpuArm64 {
				return minimumMacOSVersionOf(architecture.File), nil
			}
		}
		return 0, fmt.Errorf("the universal binary carries no arm64 slice")
	}
	var formatError *macho.FormatError
	if !errors.Is(errorValue, macho.ErrNotFat) && errors.As(errorValue, &formatError) {
		return 0, nil
	}
	if !errors.Is(errorValue, macho.ErrNotFat) {
		return 0, errorValue
	}
	file, errorValue := macho.Open(path)
	if errorValue != nil {
		return 0, errorValue
	}
	defer file.Close()
	if file.Cpu != macho.CpuArm64 {
		return 0, fmt.Errorf("it is built for %s and the keg is arm64", file.Cpu)
	}
	return minimumMacOSVersionOf(file), nil
}

func minimumMacOSVersionOf(file *macho.File) int {
	for _, load := range file.Loads {
		raw := load.Raw()
		if len(raw) < 16 {
			continue
		}
		switch file.ByteOrder.Uint32(raw[0:4]) {
		case loadCommandBuildVersion:
			if file.ByteOrder.Uint32(raw[8:12]) == machOPlatformMacOS {
				return int(file.ByteOrder.Uint32(raw[12:16]) >> 16)
			}
		case loadCommandVersionMinMac:
			return int(file.ByteOrder.Uint32(raw[8:12]) >> 16)
		}
	}
	return 0
}

func buildHomebrewKeg(repositoryRootPath string, kegPath string, version string, output io.Writer) error {
	binaryPath := filepath.Join(kegPath, "bin")
	libraryPath := filepath.Join(kegPath, "libexec")
	for _, path := range []string{binaryPath, libraryPath} {
		if errorValue := os.MkdirAll(path, 0o755); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := copyBrewMessengerPrograms(repositoryRootPath, libraryPath, output); errorValue != nil {
		return errorValue
	}
	if errorValue := buildBrewGoPrograms(repositoryRootPath, binaryPath, libraryPath, version, output); errorValue != nil {
		return errorValue
	}
	if errorValue := buildBrewBunPrograms(repositoryRootPath, libraryPath, output); errorValue != nil {
		return errorValue
	}
	if errorValue := fetchBrewVendoredPrograms(repositoryRootPath, libraryPath, output); errorValue != nil {
		return errorValue
	}
	if errorValue := copyFile(
		filepath.Join(repositoryRootPath, "tools", blueclaw.RenderCompanyRuntimeName),
		filepath.Join(libraryPath, blueclaw.RenderCompanyRuntimeName), 0o755); errorValue != nil {
		return errorValue
	}
	return copyBrewCarriedTrees(repositoryRootPath, libraryPath)
}

func buildBrewGoPrograms(repositoryRootPath string, binaryPath string, libraryPath string, version string, output io.Writer) error {
	for _, program := range packagedGoPrograms() {
		destination := filepath.Join(libraryPath, program.Name)
		if program.Name == blueclaw.CompanyPackageName {
			destination = filepath.Join(binaryPath, program.Name)
		}
		stamped := "-s -w " + admindStampFlags(version, releaseBinaryRevision(repositoryRootPath))
		command := exec.Command("go", "build", "-trimpath", "-ldflags", stamped, "-o", destination, program.Package)
		command.Dir = filepath.Join(repositoryRootPath, program.ModuleRoot)
		command.Env = append(os.Environ(), "GOOS=darwin", "GOARCH=arm64", "CGO_ENABLED=0")
		if commandOutput, errorValue := command.CombinedOutput(); errorValue != nil {
			return fmt.Errorf("compile %s for darwin/arm64: %s", program.Name, strings.TrimSpace(string(commandOutput)))
		}
		fmt.Fprintf(output, "  compiled %s\n", program.Name)
	}
	return nil
}

func buildBrewBunPrograms(repositoryRootPath string, libraryPath string, output io.Writer) error {
	for _, program := range packagedBunPrograms() {
		installArguments := []string{"install", "--frozen-lockfile"}
		if program.InstallFilter != "" {
			installArguments = append(installArguments, "--filter", program.InstallFilter)
		}
		install := exec.Command("bun", installArguments...)
		install.Dir = filepath.Join(repositoryRootPath, program.InstallWorking)
		if commandOutput, errorValue := install.CombinedOutput(); errorValue != nil {
			return fmt.Errorf("resolve %s dependencies: %s", program.Name, strings.TrimSpace(string(commandOutput)))
		}
		build := exec.Command("bun", "build", "--compile", "--target=bun-darwin-arm64",
			"--outfile", filepath.Join(libraryPath, program.Name), program.EntryPoint)
		build.Dir = filepath.Join(repositoryRootPath, program.WorkingRoot)
		if commandOutput, errorValue := build.CombinedOutput(); errorValue != nil {
			return fmt.Errorf("compile %s for darwin/arm64: %s", program.Name, strings.TrimSpace(string(commandOutput)))
		}
		fmt.Fprintf(output, "  compiled %s\n", program.Name)
	}
	return nil
}

// The messenger is built by tools/prepare-buzz-relay from a pinned upstream
// revision, on this Mac rather than in a container. A keg for an architecture
// that artifact does not cover is refused rather than shipped without a
// messenger.
func copyBrewMessengerPrograms(repositoryRootPath string, libraryPath string, output io.Writer) error {
	for _, name := range []string{blueclaw.BuzzRelayName, blueclaw.BuzzAdminName} {
		sourcePath := filepath.Join(repositoryRootPath, brewMessengerArtifactPath, name)
		if errorValue := requireMachOArm64(sourcePath, name); errorValue != nil {
			return errorValue
		}
		if errorValue := copyFile(sourcePath, filepath.Join(libraryPath, name), 0o755); errorValue != nil {
			return errorValue
		}
		fmt.Fprintf(output, "  carried %s\n", name)
	}
	return nil
}

// requireMachOArm64 reads the header rather than shelling out to file(1), the
// same way requireELFFits does for the Linux packages. A Linux binary in the keg
// would install and never start.
func requireMachOArm64(sourcePath string, name string) error {
	information, errorValue := os.Stat(sourcePath)
	if errorValue != nil {
		return fmt.Errorf(
			"%s is not at %s; build it with `tools/prepare-buzz-relay --target darwin-arm64`: %w", name, sourcePath, errorValue)
	}
	if information.Size() == 0 {
		return fmt.Errorf("%s at %s is empty; rebuild it with `tools/prepare-buzz-relay --target darwin-arm64`", name, sourcePath)
	}
	file, errorValue := macho.Open(sourcePath)
	if errorValue != nil {
		return fmt.Errorf("%s at %s is not a Mach-O binary: %w", name, sourcePath, errorValue)
	}
	defer file.Close()
	if file.Cpu != macho.CpuArm64 {
		return fmt.Errorf("%s at %s is built for %s and this keg is arm64", name, sourcePath, file.Cpu)
	}
	return nil
}

func fetchBrewVendoredPrograms(repositoryRootPath string, libraryPath string, output io.Writer) error {
	downloads, errorValue := blueclaw.HostPayloadDownloadsForTarget(blueclaw.HostPayloadDarwinArm64)
	if errorValue != nil {
		return errorValue
	}
	for _, download := range downloads {
		programPath, errorValue := fetchVendoredProgram(repositoryRootPath, download, libraryPath, output)
		if errorValue != nil {
			return errorValue
		}
		destination := filepath.Join(libraryPath, download.ProgramName)
		if programPath == destination {
			continue
		}
		if errorValue := copyFile(programPath, destination, 0o755); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func copyBrewCarriedTrees(repositoryRootPath string, libraryPath string) error {
	for _, carried := range []struct{ source, destination string }{
		{".dependency/internkim-plugin/skills", "skills"},
		{blueclaw.BlueclawSubmodulePath + "/migrations", "migrations"},
	} {
		if errorValue := copyTree(
			filepath.Join(repositoryRootPath, carried.source),
			filepath.Join(libraryPath, carried.destination)); errorValue != nil {
			return fmt.Errorf("the keg carries %s: %w", carried.destination, errorValue)
		}
	}
	layout := blueclaw.MacCompanyHostLayout("")
	for _, carried := range []struct{ source, destination string }{
		{"host/runtime.template.json", layout.RuntimeTemplatePath()},
		{documentConversionLockPath, layout.DocumentRequirementsPath()},
	} {
		destinationPath := filepath.Join(libraryPath, strings.TrimPrefix(carried.destination, layout.LibraryRoot))
		if errorValue := os.MkdirAll(filepath.Dir(destinationPath), 0o755); errorValue != nil {
			return errorValue
		}
		if errorValue := copyFile(filepath.Join(repositoryRootPath, carried.source), destinationPath, 0o644); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

// writeGzippedTar tars a directory and returns what the result hashes to. A
// prefix makes it a bottle, whose entries Homebrew expects under
// <name>/<version>; an empty prefix makes it the tarball the formula's url names.
func writeGzippedTar(archivePath string, rootPath string, prefix string) (string, error) {
	file, errorValue := os.Create(archivePath)
	if errorValue != nil {
		return "", errorValue
	}
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	errorValue = filepath.Walk(rootPath, func(path string, information fs.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relative, errorValue := filepath.Rel(rootPath, path)
		if errorValue != nil || relative == "." {
			return errorValue
		}
		return writeHomebrewTarEntry(archive, path, filepath.Join(prefix, relative), information)
	})
	if errorValue != nil {
		file.Close()
		return "", errorValue
	}
	for _, closer := range []io.Closer{archive, compressed, file} {
		if errorValue := closer.Close(); errorValue != nil {
			return "", errorValue
		}
	}
	return checksumOf(archivePath)
}

func writeHomebrewTarEntry(archive *tar.Writer, path string, name string, information fs.FileInfo) error {
	link := ""
	if information.Mode()&fs.ModeSymlink != 0 {
		read, errorValue := os.Readlink(path)
		if errorValue != nil {
			return errorValue
		}
		link = read
	}
	header, errorValue := tar.FileInfoHeader(information, link)
	if errorValue != nil {
		return errorValue
	}
	header.Name = name
	// Two builds of one tree produce one tarball, which is what lets a release
	// be cut twice and publish the same bytes.
	header.ModTime = homebrewArchiveTime
	header.AccessTime = homebrewArchiveTime
	header.ChangeTime = homebrewArchiveTime
	header.Uname = ""
	header.Gname = ""
	header.Uid = 0
	header.Gid = 0
	if errorValue := archive.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	if !information.Mode().IsRegular() {
		return nil
	}
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	_, errorValue = io.Copy(archive, file)
	return errorValue
}

func copyFile(sourcePath string, destinationPath string, mode fs.FileMode) error {
	source, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	defer source.Close()
	if errorValue := os.MkdirAll(filepath.Dir(destinationPath), 0o755); errorValue != nil {
		return errorValue
	}
	destination, errorValue := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if errorValue != nil {
		return errorValue
	}
	defer destination.Close()
	_, errorValue = io.Copy(destination, source)
	return errorValue
}

func copyTree(sourcePath string, destinationPath string) error {
	return filepath.Walk(sourcePath, func(path string, information fs.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relative, errorValue := filepath.Rel(sourcePath, path)
		if errorValue != nil {
			return errorValue
		}
		destination := filepath.Join(destinationPath, relative)
		if information.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		if information.Mode()&fs.ModeSymlink != 0 {
			return nil
		}
		return copyFile(path, destination, information.Mode().Perm())
	})
}
