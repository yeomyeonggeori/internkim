package cli

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/aptrepository"
)

func runReleaseAPT(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	suite := firstNonEmptyString(commandArgumentValue(arguments, "--suite", ""), aptrepository.DefaultSuite)
	packageDirectory := firstNonEmptyString(
		commandArgumentValue(arguments, "--package-directory", ""),
		filepath.Join(repositoryRootPath, debDefaultOutputDirectory),
	)
	packages, errorValue := readPackageDirectory(packageDirectory)
	if errorValue != nil {
		return errorValue
	}
	signingKeyPath, removeSigningKey, errorValue := aptrepository.MaterialiseSigningKey()
	if errorValue != nil {
		return errorValue
	}
	defer removeSigningKey()
	signer, errorValue := aptrepository.NewGPGSigner(signingKeyPath)
	if errorValue != nil {
		return errorValue
	}
	defer signer.Close()

	objects, errorValue := aptrepository.Build(suite, packages, signer, time.Now())
	if errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(os.Stdout, "signed suite %s with %s\n", suite, signer.Fingerprint())

	if outputDirectory := commandArgumentValue(arguments, "--output", ""); outputDirectory != "" {
		return writeRepositoryTree(outputDirectory, objects, os.Stdout)
	}
	publisher, errorValue := releasePublisherFromEnvironment(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	return publishRepositoryObjects(publisher, objects, os.Stdout)
}

func readPackageDirectory(directoryPath string) ([]aptrepository.Package, error) {
	entries, errorValue := os.ReadDir(directoryPath)
	if errorValue != nil {
		return nil, fmt.Errorf("read the package directory: %w", errorValue)
	}
	var packages []aptrepository.Package
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".deb" {
			continue
		}
		contents, errorValue := os.ReadFile(filepath.Join(directoryPath, entry.Name()))
		if errorValue != nil {
			return nil, errorValue
		}
		packages = append(packages, aptrepository.Package{FileName: entry.Name(), Contents: contents})
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("%s holds no .deb; run `internkim release deb` first", directoryPath)
	}
	return packages, nil
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

func publishRepositoryObjects(publisher releaseObjectPublisher, objects map[string][]byte, output io.Writer) error {
	for _, objectKey := range sortedObjectKeys(objects) {
		if errorValue := publisher.PutObject(objectKey, objects[objectKey], aptContentType(objectKey)); errorValue != nil {
			return errorValue
		}
		fmt.Fprintf(output, "published %s (%d bytes)\n", objectKey, len(objects[objectKey]))
	}
	fmt.Fprintf(output, "%s -> %s\n", aptrepository.Prefix, publisher.PublicURL(aptrepository.Prefix+"/"))
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

func aptContentType(objectKey string) string {
	switch {
	case strings.HasSuffix(objectKey, ".deb"):
		return "application/vnd.debian.binary-package"
	case strings.HasSuffix(objectKey, ".gz"):
		return "application/gzip"
	case strings.HasSuffix(objectKey, ".pgp"):
		return "application/pgp-keys"
	case path.Base(objectKey) == "Release.gpg":
		return "application/pgp-signature"
	}
	return "text/plain; charset=utf-8"
}
