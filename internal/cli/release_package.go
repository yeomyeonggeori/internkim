package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goreleaser/nfpm/v2"
	_ "github.com/goreleaser/nfpm/v2/arch"
	_ "github.com/goreleaser/nfpm/v2/deb"
	"github.com/goreleaser/nfpm/v2/files"
	_ "github.com/goreleaser/nfpm/v2/rpm"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const releaseChecksumsName = "SHA256SUMS"

var packageLayout = blueclaw.LinuxCompanyHostLayout()

// One payload, three package formats. The contents of the package are built
// once (packageContents), and each format is that same list of files with
// its own dependency dialect and its own maintainer-script prologue.
type linuxPackageFormat struct {
	// Name is nfpm's name for the packager and what --format takes.
	Name string
	// Manager is whose dependency dialect the package speaks.
	Manager blueclaw.PackageManager
	// HasPurge is whether the format can tell a removal from a purge.
	HasPurge bool
	// Compression is what nfpm compresses the payload with.
	Compression string
	// CompressPayload, when set, compresses the payload nfpm left uncompressed.
	CompressPayload func(packagePath string) error
	// Suffix ends the file the format is shipped as, and is what the format's
	// manager asks a local file to end with.
	Suffix string
}

var (
	debianPackageFormat = linuxPackageFormat{
		Name:     "deb",
		Manager:  blueclaw.PackageManagerApt,
		HasPurge: true,
		// dpkg has read xz since 1.15, older than every distribution the package is for.
		Compression:     "none",
		CompressPayload: compressDebianPayload,
		Suffix:          ".deb",
	}
	rpmPackageFormat = linuxPackageFormat{
		Name:        "rpm",
		Manager:     blueclaw.PackageManagerDnf,
		Compression: "zstd",
		Suffix:      ".rpm",
	}
	archlinuxPackageFormat = linuxPackageFormat{
		Name:    "archlinux",
		Manager: blueclaw.PackageManagerPacman,
		Suffix:  ".pkg.tar.zst",
	}
)

func linuxPackageFormats() []linuxPackageFormat {
	return []linuxPackageFormat{debianPackageFormat, rpmPackageFormat, archlinuxPackageFormat}
}

// releaseFormats is what one build writes into the release directory: the
// Linux packages asked for and, unless left out, the Homebrew bottle.
type releaseFormats struct {
	Linux    []linuxPackageFormat
	Homebrew bool
}

const homebrewFormatName = "homebrew"

func everyReleaseFormat() releaseFormats {
	return releaseFormats{Linux: linuxPackageFormats(), Homebrew: true}
}

func releaseFormatsNamed(requested string) (releaseFormats, error) {
	if requested == "" {
		return everyReleaseFormat(), nil
	}
	chosen := releaseFormats{}
	for _, name := range strings.Split(requested, ",") {
		name = strings.TrimSpace(name)
		if name == homebrewFormatName {
			chosen.Homebrew = true
			continue
		}
		format, isKnown := linuxPackageFormatNamed(name)
		if !isKnown {
			return releaseFormats{}, fmt.Errorf("no package is built as %q; the formats are deb, rpm, archlinux and %s", name, homebrewFormatName)
		}
		chosen.Linux = append(chosen.Linux, format)
	}
	return chosen, nil
}

func linuxPackageFormatNamed(name string) (linuxPackageFormat, bool) {
	for _, format := range linuxPackageFormats() {
		if format.Name == name {
			return format, true
		}
	}
	return linuxPackageFormat{}, false
}

// RunsOnlyWhen is the first line of a script that only one of dpkg's calls is
// for: dpkg runs postinst for configure, abort-upgrade and others, and only the
// first is an install. rpm and pacman call each script for exactly its purpose.
func (format linuxPackageFormat) RunsOnlyWhen(script string) string {
	if format.Name == "deb" && script == "postinst" {
		return "[ \"$1\" = configure ] || exit 0\n"
	}
	return ""
}

// RemovalTest is the shell test that says this run is a removal and not an
// upgrade. dpkg says so in $1, rpm says 0 instances remain, and pacman only
// calls its removal functions for a removal.
func (format linuxPackageFormat) RemovalTest(script string) string {
	switch format.Name {
	case "deb":
		if script == "prerm" {
			return `[ "$1" = remove ] || [ "$1" = deconfigure ]`
		}
		return `[ "$1" = remove ] || [ "$1" = purge ]`
	case "rpm":
		return `[ "${1:-}" = 0 ]`
	}
	return "true"
}

// assetName is the file one format and architecture is shipped as. The version
// lives in the package and in the release tag, so the name stays put and
// releases/latest/download/<name> always answers; web/static/install.sh spells
// the same name.
func (format linuxPackageFormat) assetName(architecture string) string {
	return blueclaw.CompanyPackageName + "-" + architecture + format.Suffix
}

func linuxPackageInformation(format linuxPackageFormat, target packageTarget, version string, contents files.Contents, scripts nfpm.Scripts) *nfpm.Info {
	return &nfpm.Info{
		Name:          blueclaw.CompanyPackageName,
		Arch:          target.Architecture,
		Platform:      "linux",
		Version:       version,
		VersionSchema: "none",
		Section:       blueclaw.CompanyPackageSection,
		Priority:      "optional",
		Maintainer:    blueclaw.CompanyPackageMaintainer,
		Description:   packageDescription,
		Vendor:        blueclaw.CompanyPackageVendor,
		Homepage:      blueclaw.CompanyPackageHomepage,
		License:       packageLicense,
		MTime:         time.Now().UTC().Truncate(time.Second),
		Overridables: nfpm.Overridables{
			Depends:   blueclaw.HostPackageDependsFor(format.Manager),
			Contents:  contents,
			Scripts:   scripts,
			Deb:       nfpm.Deb{Compression: format.Compression},
			RPM:       nfpm.RPM{Compression: format.Compression},
			ArchLinux: nfpm.ArchLinux{Scripts: nfpm.ArchLinuxScripts{PostUpgrade: scripts.PostInstall}},
		},
	}
}

// nfpm's Arch writer records a file two ways that have to agree for
// `pacman -Qkk` to find the file unaltered. The tar header rounds a modification
// time to the nearest second where the .MTREE truncates it, so a file with a
// fractional mtime is a second apart in its own package; and int64(mode) puts
// Go's setuid flag (os.ModeSetuid) where no tool reads it, so the helper arrives
// without its mode. The deb and rpm writers handle both. For Arch the contents
// are expanded first, since tree expansion is what reads each file's own mtime,
// and then copied, because the contents are shared with the other formats.
func archContents(contents files.Contents) files.Contents {
	translated := make(files.Contents, len(contents))
	for index, content := range contents {
		copied := *content
		information := *content.FileInfo
		information.MTime = information.MTime.Truncate(time.Second)
		if information.Mode&os.ModeSetuid != 0 {
			information.Mode = information.Mode&^os.ModeSetuid | 0o4000
		}
		copied.FileInfo = &information
		translated[index] = &copied
	}
	return translated
}

func writeLinuxPackage(format linuxPackageFormat, information *nfpm.Info, packagePath string) error {
	packager, errorValue := nfpm.Get(format.Name)
	if errorValue != nil {
		return errorValue
	}
	information = nfpm.WithDefaults(information)
	if errorValue := nfpm.Validate(information); errorValue != nil {
		return errorValue
	}
	if format.Name == "archlinux" {
		if errorValue := nfpm.PrepareForPackager(information, format.Name); errorValue != nil {
			return errorValue
		}
		information.Contents = archContents(information.Contents)
		information.DisableGlobbing = true
	}
	file, errorValue := os.Create(packagePath)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	if errorValue := packager.Package(information, file); errorValue != nil {
		return errorValue
	}
	if errorValue := file.Close(); errorValue != nil || format.CompressPayload == nil {
		return errorValue
	}
	return format.CompressPayload(packagePath)
}

func writeMaintainerScripts(format linuxPackageFormat, stagingPath string) (nfpm.Scripts, error) {
	directory := filepath.Join(stagingPath, "scripts-"+format.Name)
	if errorValue := os.MkdirAll(directory, 0o755); errorValue != nil {
		return nfpm.Scripts{}, errorValue
	}
	paths := map[packageScript]string{}
	for _, script := range []packageScript{postInstallScript, preRemoveScript, postRemoveScript} {
		paths[script] = filepath.Join(directory, string(script))
		if errorValue := os.WriteFile(paths[script], []byte(maintainerScript(format, script)), 0o755); errorValue != nil {
			return nfpm.Scripts{}, errorValue
		}
	}
	return nfpm.Scripts{
		PostInstall: paths[postInstallScript],
		PreRemove:   paths[preRemoveScript],
		PostRemove:  paths[postRemoveScript],
	}, nil
}

// buildLinuxPackages stages the payload once and writes it as every format asked
// for, so the three packages of one architecture carry byte-identical files.
func buildLinuxPackages(repositoryRootPath string, target packageTarget, version string, outputDirectory string, formats []linuxPackageFormat, output io.Writer) ([]string, error) {
	stagingPath, errorValue := os.MkdirTemp("", "internkim-package-*")
	if errorValue != nil {
		return nil, errorValue
	}
	defer os.RemoveAll(stagingPath)
	contents, errorValue := packageContents(repositoryRootPath, target, version, stagingPath, output)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := os.MkdirAll(outputDirectory, 0o755); errorValue != nil {
		return nil, errorValue
	}
	built := []string{}
	for _, format := range formats {
		scripts, errorValue := writeMaintainerScripts(format, stagingPath)
		if errorValue != nil {
			return nil, errorValue
		}
		information := linuxPackageInformation(format, target, version, contents, scripts)
		packagePath := filepath.Join(outputDirectory, format.assetName(target.Architecture))
		if errorValue := writeLinuxPackage(format, information, packagePath); errorValue != nil {
			return nil, fmt.Errorf("write the %s package: %w", format.Name, errorValue)
		}
		built = append(built, packagePath)
	}
	return built, nil
}

func runReleasePackages(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	formats, errorValue := releaseFormatsNamed(commandArgumentValue(arguments, "--format", ""))
	if errorValue != nil {
		return errorValue
	}
	targets, errorValue := packageTargetsNamed(commandArgumentValue(arguments, "--architecture", ""))
	if errorValue != nil {
		return errorValue
	}
	version := firstNonEmptyString(commandArgumentValue(arguments, "--version", ""), packageVersionFromRepository(repositoryRootPath))
	outputDirectory := firstNonEmptyString(commandArgumentValue(arguments, "--out", ""), filepath.Join(repositoryRootPath, defaultPackageDirectory))
	return buildHostRelease(repositoryRootPath, targets, version, outputDirectory, formats, os.Stdout)
}

// buildHostRelease writes the directory a release is uploaded from: one package
// per format and architecture, the Homebrew bottle with its tarball and formula,
// and the SHA256SUMS that lists them. The bottle goes first because it is the
// part that needs this machine to be an Apple-silicon Mac.
func buildHostRelease(repositoryRootPath string, targets []packageTarget, version string, outputDirectory string, formats releaseFormats, output io.Writer) error {
	if formats.Homebrew {
		if errorValue := buildHomebrewRelease(repositoryRootPath, version, outputDirectory, output); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := buildLinuxReleasePackages(repositoryRootPath, targets, version, outputDirectory, formats.Linux, output); errorValue != nil {
		return errorValue
	}
	return writeReleaseChecksums(outputDirectory, version)
}

func buildLinuxReleasePackages(repositoryRootPath string, targets []packageTarget, version string, outputDirectory string, formats []linuxPackageFormat, output io.Writer) error {
	if len(formats) == 0 {
		return nil
	}
	for _, target := range targets {
		built, errorValue := buildLinuxPackages(repositoryRootPath, target, version, outputDirectory, formats, output)
		if errorValue != nil {
			return errorValue
		}
		for _, packagePath := range built {
			fmt.Fprintf(output, "built %s\n", packagePath)
		}
	}
	return nil
}

// writeReleaseChecksums lists every release asset the directory holds, so a
// directory built one format or architecture at a time still has one list
// naming all of them.
func writeReleaseChecksums(directory string, version string) error {
	names, errorValue := releaseAssetsIn(directory, version)
	if errorValue != nil {
		return errorValue
	}
	var checksums strings.Builder
	for _, name := range names {
		checksum, _, errorValue := releaseFileSHA256AndSize(filepath.Join(directory, name))
		if errorValue != nil {
			return errorValue
		}
		fmt.Fprintf(&checksums, "%s  %s\n", checksum, name)
	}
	return os.WriteFile(filepath.Join(directory, releaseChecksumsName), []byte(checksums.String()), 0o644)
}

// releaseAssetsIn is every asset of this version the directory holds. The
// bottle is found rather than named, because its tag is read from the keg.
func releaseAssetsIn(directory string, version string) ([]string, error) {
	present := []string{}
	for _, name := range releaseAssetNamesWithoutBottle() {
		_, errorValue := os.Stat(filepath.Join(directory, name))
		if errors.Is(errorValue, os.ErrNotExist) {
			continue
		}
		if errorValue != nil {
			return nil, errorValue
		}
		present = append(present, name)
	}
	bottles, errorValue := homebrewBottlesIn(directory, version)
	if errorValue != nil {
		return nil, errorValue
	}
	return append(present, bottles...), nil
}

func homebrewBottlesIn(directory string, version string) ([]string, error) {
	paths, errorValue := filepath.Glob(filepath.Join(directory, blueclaw.HomebrewBottleFileName(version, "*")))
	if errorValue != nil {
		return nil, errorValue
	}
	names := []string{}
	for _, path := range paths {
		names = append(names, filepath.Base(path))
	}
	return names, nil
}

func releaseAssetNamesWithoutBottle() []string {
	return append(linuxPackageAssetNames(), blueclaw.HomebrewSourceTarballName(), blueclaw.HomebrewFormulaAssetName())
}

func linuxPackageAssetNames() []string {
	names := []string{}
	for _, target := range packageTargets {
		for _, format := range linuxPackageFormats() {
			names = append(names, format.assetName(target.Architecture))
		}
	}
	return names
}
