package cli

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/aptrepository"
	"gitlab.com/eastriver/internkim/internal/packagerepository"
	"gitlab.com/eastriver/internkim/internal/pacmanrepository"
	"gitlab.com/eastriver/internkim/internal/rpmrepository"
)

type repositoryFormat struct {
	name          string
	packageSuffix string
	prefix        string
	build         func(channel string, packages []packagerepository.Package, signer packagerepository.Signer, now time.Time) (map[string][]byte, error)
}

var repositoryFormats = []repositoryFormat{
	{name: debianPackageFormat.Name, packageSuffix: ".deb", prefix: aptrepository.Prefix, build: aptrepository.Build},
	{name: rpmPackageFormat.Name, packageSuffix: ".rpm", prefix: rpmrepository.Prefix, build: rpmrepository.Build},
	{name: archlinuxPackageFormat.Name, packageSuffix: ".pkg.tar.zst", prefix: pacmanrepository.Prefix, build: pacmanrepository.Build},
}

func repositoryFormatsNamed(requested string) ([]repositoryFormat, error) {
	if requested == "" {
		return repositoryFormats, nil
	}
	chosen := []repositoryFormat{}
	for _, name := range strings.Split(requested, ",") {
		index := slices.IndexFunc(repositoryFormats, func(format repositoryFormat) bool { return format.name == strings.TrimSpace(name) })
		if index < 0 {
			return nil, fmt.Errorf("no repository is published for %q; the formats are deb, rpm and archlinux", name)
		}
		chosen = append(chosen, repositoryFormats[index])
	}
	return chosen, nil
}

func runReleaseRepositories(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	channel := firstNonEmptyString(commandArgumentValue(arguments, "--channel", ""), packagerepository.DefaultChannel)
	if errorValue := packagerepository.CheckChannel(channel); errorValue != nil {
		return errorValue
	}
	formats, errorValue := repositoryFormatsNamed(commandArgumentValue(arguments, "--format", ""))
	if errorValue != nil {
		return errorValue
	}
	packageDirectory := firstNonEmptyString(
		commandArgumentValue(arguments, "--package-directory", ""),
		filepath.Join(repositoryRootPath, defaultPackageDirectory),
	)
	packagesByFormat, errorValue := readPackageDirectory(packageDirectory, formats)
	if errorValue != nil {
		return errorValue
	}
	signingKeyPath, removeSigningKey, errorValue := packagerepository.MaterialiseSigningKey()
	if errorValue != nil {
		return errorValue
	}
	defer removeSigningKey()
	signer, errorValue := packagerepository.NewGPGSigner(signingKeyPath)
	if errorValue != nil {
		return errorValue
	}
	defer signer.Close()

	objects, errorValue := buildRepositoryObjects(channel, formats, packagesByFormat, signer, time.Now())
	if errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(os.Stdout, "signed channel %s with %s\n", channel, signer.Fingerprint())

	if outputDirectory := commandArgumentValue(arguments, "--output", ""); outputDirectory != "" {
		return writeRepositoryTree(outputDirectory, objects, os.Stdout)
	}
	publisher, errorValue := releasePublisherFromEnvironment(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	return publishRepositoryObjects(publisher, formats, objects, os.Stdout)
}

func buildRepositoryObjects(channel string, formats []repositoryFormat, packagesByFormat map[string][]packagerepository.Package, signer packagerepository.Signer, now time.Time) (map[string][]byte, error) {
	objects := map[string][]byte{}
	for _, format := range formats {
		built, errorValue := format.build(channel, packagesByFormat[format.name], signer, now)
		if errorValue != nil {
			return nil, fmt.Errorf("%s repository: %w", format.name, errorValue)
		}
		for objectKey, contents := range built {
			objects[objectKey] = contents
		}
	}
	return objects, nil
}

func readPackageDirectory(directoryPath string, formats []repositoryFormat) (map[string][]packagerepository.Package, error) {
	entries, errorValue := os.ReadDir(directoryPath)
	if errorValue != nil {
		return nil, fmt.Errorf("read the package directory: %w", errorValue)
	}
	packagesByFormat := map[string][]packagerepository.Package{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for _, format := range formats {
			if !strings.HasSuffix(entry.Name(), format.packageSuffix) {
				continue
			}
			contents, errorValue := os.ReadFile(filepath.Join(directoryPath, entry.Name()))
			if errorValue != nil {
				return nil, errorValue
			}
			packagesByFormat[format.name] = append(packagesByFormat[format.name], packagerepository.Package{FileName: entry.Name(), Contents: contents})
		}
	}
	for _, format := range formats {
		if len(packagesByFormat[format.name]) == 0 {
			return nil, fmt.Errorf("%s holds no %s file; run `internkim release packages` first, or name the formats to publish with --format", directoryPath, format.packageSuffix)
		}
	}
	return packagesByFormat, nil
}

func writeRepositoryTree(outputDirectory string, objects map[string][]byte, output io.Writer) error {
	for _, objectKey := range sortedObjectKeys(objects) {
		destination := filepath.Join(outputDirectory, filepath.FromSlash(objectKey))
		if errorValue := os.MkdirAll(filepath.Dir(destination), 0o755); errorValue != nil {
			return errorValue
		}
		if errorValue := os.WriteFile(destination, objects[objectKey], 0o644); errorValue != nil {
			return errorValue
		}
	}
	fmt.Fprintf(output, "wrote %d objects to %s\n", len(objects), outputDirectory)
	return nil
}

func publishRepositoryObjects(publisher releaseObjectPublisher, formats []repositoryFormat, objects map[string][]byte, output io.Writer) error {
	for _, objectKey := range sortedObjectKeys(objects) {
		if errorValue := publisher.PutObject(objectKey, objects[objectKey], repositoryContentType(objectKey)); errorValue != nil {
			return errorValue
		}
		fmt.Fprintf(output, "published %s (%d bytes)\n", objectKey, len(objects[objectKey]))
	}
	for _, format := range formats {
		fmt.Fprintf(output, "%s -> %s\n", format.prefix, publisher.PublicURL(format.prefix+"/"))
	}
	return nil
}

func sortedObjectKeys(objects map[string][]byte) []string {
	objectKeys := make([]string, 0, len(objects))
	for objectKey := range objects {
		objectKeys = append(objectKeys, objectKey)
	}
	sort.Strings(objectKeys)
	return objectKeys
}

func repositoryContentType(objectKey string) string {
	switch {
	case strings.HasSuffix(objectKey, ".deb"):
		return "application/vnd.debian.binary-package"
	case strings.HasSuffix(objectKey, ".rpm"):
		return "application/x-rpm"
	case strings.HasSuffix(objectKey, ".pkg.tar.zst"):
		return "application/zstd"
	case strings.HasSuffix(objectKey, ".gz"), strings.HasSuffix(objectKey, ".db"), strings.HasSuffix(objectKey, ".files"):
		return "application/gzip"
	case strings.HasSuffix(objectKey, ".pgp"), path.Base(objectKey) == rpmrepository.KeyName, path.Base(objectKey) == pacmanrepository.KeyName:
		return "application/pgp-keys"
	case strings.HasSuffix(objectKey, ".sig"), strings.HasSuffix(objectKey, ".asc"), path.Base(objectKey) == "Release.gpg":
		return "application/pgp-signature"
	case strings.HasSuffix(objectKey, ".xml"):
		return "application/xml"
	}
	return "text/plain; charset=utf-8"
}
