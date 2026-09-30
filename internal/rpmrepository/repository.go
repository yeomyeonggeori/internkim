package rpmrepository

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/packagerepository"
)

const (
	// Prefix is where the whole repository sits in the release bucket, and the
	// one path `workers/release-registry` serves it from.
	Prefix = "rpm"

	// KeyName is the public half of the signing key, beside the architectures
	// of a channel so the install script can name it from the repository URL
	// it already holds.
	KeyName = "internkim-rpm-signing.asc"

	PackagesDirectory = "Packages"
)

// Architectures are the two the company host is built for, spelled as rpm and
// dnf's $basearch spell them. Both are published whether or not a package
// exists for each, so an architecture dnf asks for answers with an empty
// repository and not with a missing one.
var Architectures = []string{"x86_64", "aarch64"}

// Build renders every object of a one-channel rpm repository, keyed by the
// object name it takes in the release bucket: each package re-signed with the
// archive key, each architecture's repodata, and the detached signature over
// its repomd.xml that `repo_gpgcheck` reads.
func Build(channel string, packages []packagerepository.Package, signer packagerepository.Signer, now time.Time) (map[string][]byte, error) {
	if errorValue := packagerepository.CheckChannel(channel); errorValue != nil {
		return nil, errorValue
	}
	if signer == nil {
		return nil, fmt.Errorf("an rpm repository is signed; no signer was given")
	}
	objects := map[string][]byte{}
	indexed := map[string][]indexedPackage{}
	for _, packageFile := range packages {
		signed, errorValue := Sign(packageFile.Contents, signer)
		if errorValue != nil {
			return nil, fmt.Errorf("sign %s: %w", packageFile.FileName, errorValue)
		}
		metadata, errorValue := readMetadata(signed)
		if errorValue != nil {
			return nil, fmt.Errorf("read %s: %w", packageFile.FileName, errorValue)
		}
		served := architecturesServedBy(metadata.architecture)
		if len(served) == 0 {
			return nil, fmt.Errorf("%s is built for %s, and the rpm repository publishes %s", packageFile.FileName, metadata.architecture, strings.Join(Architectures, " and "))
		}
		for _, architecture := range served {
			objects[path.Join(architectureDirectory(channel, architecture), PackagesDirectory, packageFile.FileName)] = signed
			indexed[architecture] = append(indexed[architecture], indexedPackage{
				metadata:  metadata,
				fileName:  packageFile.FileName,
				checksum:  sha256Hex(signed),
				fileSize:  len(signed),
				timestamp: now.Unix(),
			})
		}
	}
	for _, architecture := range Architectures {
		if errorValue := addRepositoryMetadata(objects, channel, architecture, indexed[architecture], signer, now); errorValue != nil {
			return nil, errorValue
		}
	}
	key, errorValue := signer.PublicKeyArmoured()
	if errorValue != nil {
		return nil, fmt.Errorf("export the public key: %w", errorValue)
	}
	objects[path.Join(Prefix, channel, KeyName)] = key
	return objects, nil
}

func architectureDirectory(channel string, architecture string) string {
	return path.Join(Prefix, channel, architecture)
}

func architecturesServedBy(packageArchitecture string) []string {
	if packageArchitecture == "noarch" {
		return Architectures
	}
	for _, architecture := range Architectures {
		if architecture == packageArchitecture {
			return []string{architecture}
		}
	}
	return nil
}

func addRepositoryMetadata(objects map[string][]byte, channel string, architecture string, entries []indexedPackage, signer packagerepository.Signer, now time.Time) error {
	sort.Slice(entries, func(first int, second int) bool { return entries[first].fileName < entries[second].fileName })
	files := []metadataFile{
		compressMetadata("primary", renderPrimary(entries)),
		compressMetadata("filelists", renderFileLists(entries)),
		compressMetadata("other", renderOther(entries)),
	}
	directory := architectureDirectory(channel, architecture)
	for _, file := range files {
		objects[path.Join(directory, file.location())] = file.compressed
	}
	repositoryMetadata := renderRepositoryMetadata(files, now.Unix())
	objects[path.Join(directory, "repodata", "repomd.xml")] = repositoryMetadata
	signature, errorValue := signer.DetachSign(repositoryMetadata)
	if errorValue != nil {
		return fmt.Errorf("sign the repomd.xml of %s/%s: %w", channel, architecture, errorValue)
	}
	objects[path.Join(directory, "repodata", "repomd.xml.asc")] = signature
	return nil
}
