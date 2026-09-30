package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goreleaser/nfpm/v2"
	_ "github.com/goreleaser/nfpm/v2/arch"
	"github.com/goreleaser/nfpm/v2/files"
	_ "github.com/goreleaser/nfpm/v2/rpm"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// One payload, three package formats. The contents of the package are built
// once (debPackageContents), and each format is that same list of files with
// its own dependency dialect and its own maintainer-script prologue.
type linuxPackageFormat struct {
	// Name is nfpm's name for the packager and what --format takes.
	Name string
	// Manager is whose dependency dialect the package speaks.
	Manager blueclaw.PackageManager
	// UnitHelpers are the shell functions the maintainer scripts call to act on units.
	UnitHelpers string
	// HasPurge is whether the format can tell a removal from a purge.
	HasPurge bool
	// Compression is what the payload is compressed with.
	Compression string
}

var (
	debianPackageFormat = linuxPackageFormat{
		Name:        "deb",
		Manager:     blueclaw.PackageManagerApt,
		UnitHelpers: debianUnitHelpers,
		HasPurge:    true,
		Compression: debPayloadCompression,
	}
	rpmPackageFormat = linuxPackageFormat{
		Name:        "rpm",
		Manager:     blueclaw.PackageManagerDnf,
		UnitHelpers: systemctlUnitHelpers,
		Compression: "zstd",
	}
	archlinuxPackageFormat = linuxPackageFormat{
		Name:        "archlinux",
		Manager:     blueclaw.PackageManagerPacman,
		UnitHelpers: systemctlUnitHelpers,
	}
)

func linuxPackageFormats() []linuxPackageFormat {
	return []linuxPackageFormat{debianPackageFormat, rpmPackageFormat, archlinuxPackageFormat}
}

func linuxPackageFormatsNamed(requested string) ([]linuxPackageFormat, error) {
	if requested == "" {
		return linuxPackageFormats(), nil
	}
	chosen := []linuxPackageFormat{}
	for _, name := range strings.Split(requested, ",") {
		name = strings.TrimSpace(name)
		found := false
		for _, format := range linuxPackageFormats() {
			if format.Name == name {
				chosen = append(chosen, format)
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("no package is built as %q; the formats are deb, rpm and archlinux", name)
		}
	}
	return chosen, nil
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

func (format linuxPackageFormat) packageFileName(information *nfpm.Info) (string, error) {
	packager, errorValue := nfpm.Get(format.Name)
	if errorValue != nil {
		return "", errorValue
	}
	return packager.ConventionalFileName(nfpm.WithDefaults(information)), nil
}

func linuxPackageInformation(format linuxPackageFormat, target debianTarget, version string, contents files.Contents, scripts nfpm.Scripts) *nfpm.Info {
	return &nfpm.Info{
		Name:          blueclaw.CompanyPackageName,
		Arch:          target.DebianArchitecture,
		Platform:      "linux",
		Version:       version,
		VersionSchema: "none",
		Section:       blueclaw.CompanyPackageSection,
		Priority:      "optional",
		Maintainer:    blueclaw.CompanyPackageMaintainer,
		Description:   debPackageDescription,
		Vendor:        blueclaw.CompanyPackageVendor,
		Homepage:      blueclaw.CompanyPackageHomepage,
		License:       debReleaseLicense,
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
	return packager.Package(information, file)
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
func buildLinuxPackages(repositoryRootPath string, target debianTarget, version string, outputDirectory string, formats []linuxPackageFormat, output io.Writer) ([]string, error) {
	stagingPath, errorValue := os.MkdirTemp("", "internkim-package-*")
	if errorValue != nil {
		return nil, errorValue
	}
	defer os.RemoveAll(stagingPath)
	contents, errorValue := debPackageContents(repositoryRootPath, target, version, stagingPath, output)
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
		fileName, errorValue := format.packageFileName(information)
		if errorValue != nil {
			return nil, errorValue
		}
		packagePath := filepath.Join(outputDirectory, fileName)
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
	formats, errorValue := linuxPackageFormatsNamed(commandArgumentValue(arguments, "--format", ""))
	if errorValue != nil {
		return errorValue
	}
	targets, errorValue := debTargetsNamed(commandArgumentValue(arguments, "--architecture", ""))
	if errorValue != nil {
		return errorValue
	}
	version := firstNonEmptyString(commandArgumentValue(arguments, "--version", ""), debVersionFromRepository(repositoryRootPath))
	outputDirectory := firstNonEmptyString(commandArgumentValue(arguments, "--out", ""), filepath.Join(repositoryRootPath, debDefaultOutputDirectory))
	for _, target := range targets {
		built, errorValue := buildLinuxPackages(repositoryRootPath, target, version, outputDirectory, formats, os.Stdout)
		if errorValue != nil {
			return errorValue
		}
		for _, packagePath := range built {
			fmt.Fprintf(os.Stdout, "built %s\n", packagePath)
		}
	}
	return nil
}
