package aptrepository

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	// Prefix is where the whole repository sits in the release bucket, and the
	// one path `workers/release-registry` serves it from.
	Prefix = "deb"

	// KeyringName is the public half of the signing key, beside the repository
	// so the install script can fetch it from the host it already trusts for
	// the packages.
	KeyringName = "internkim-archive-keyring.pgp"

	origin = "InternKim"
	label  = "InternKim"
)

// Architectures are the two the company host is built for. A suite declares
// both whether or not a package exists for each, because an architecture apt
// is told about and then cannot fetch an index for is a broken repository.
var Architectures = []string{"amd64", "arm64"}

// Suites are the published suites: `stable` is what an install follows, and
// `testing` carries release candidates.
var Suites = []string{"stable", "testing"}

const Component = "main"

// Package is one .deb about to be indexed.
type Package struct {
	FileName string
	Contents []byte
}

// Signer turns the text of a Release file into its two published signatures.
type Signer interface {
	ClearSign(document []byte) ([]byte, error)
	DetachSign(document []byte) ([]byte, error)
	PublicKeyring() ([]byte, error)
}

// Build renders every object of a one-suite apt repository, keyed by the
// object name it takes in the release bucket.
//
// The same map is written to disk for the install rig and uploaded to R2 by
// `internkim release apt`, so the repository a test serves is the repository a
// customer installs from.
func Build(suite string, packages []Package, signer Signer, now time.Time) (map[string][]byte, error) {
	if !isKnownSuite(suite) {
		return nil, fmt.Errorf("suite %q is not one of %s", suite, strings.Join(Suites, ", "))
	}
	if signer == nil {
		return nil, errors.New("an apt repository is signed; no signer was given")
	}
	objects := map[string][]byte{}
	indexed, errorValue := poolObjects(packages, objects)
	if errorValue != nil {
		return nil, errorValue
	}
	indices := indexObjects(suite, indexed, objects)
	release := renderRelease(suite, indices, now)
	suiteDirectory := path.Join(Prefix, "dists", suite)
	objects[path.Join(suiteDirectory, "Release")] = release

	inRelease, errorValue := signer.ClearSign(release)
	if errorValue != nil {
		return nil, fmt.Errorf("clear-sign the Release of suite %s: %w", suite, errorValue)
	}
	objects[path.Join(suiteDirectory, "InRelease")] = inRelease

	detached, errorValue := signer.DetachSign(release)
	if errorValue != nil {
		return nil, fmt.Errorf("detach-sign the Release of suite %s: %w", suite, errorValue)
	}
	objects[path.Join(suiteDirectory, "Release.gpg")] = detached

	keyring, errorValue := signer.PublicKeyring()
	if errorValue != nil {
		return nil, fmt.Errorf("export the public keyring: %w", errorValue)
	}
	objects[path.Join(Prefix, KeyringName)] = keyring
	return objects, nil
}

type indexedPackage struct {
	fields       ControlFields
	poolPath     string
	size         int
	md5Digest    string
	sha256Digest string
}

func poolObjects(packages []Package, objects map[string][]byte) ([]indexedPackage, error) {
	indexed := make([]indexedPackage, 0, len(packages))
	for _, packageFile := range packages {
		fields, errorValue := ReadControlFields(packageFile.Contents)
		if errorValue != nil {
			return nil, fmt.Errorf("read %s: %w", packageFile.FileName, errorValue)
		}
		poolPath := PoolPath(fields.Value("Package"), packageFile.FileName)
		objects[path.Join(Prefix, poolPath)] = packageFile.Contents
		indexed = append(indexed, indexedPackage{
			fields:       fields,
			poolPath:     poolPath,
			size:         len(packageFile.Contents),
			md5Digest:    digestOf(md5.New(), packageFile.Contents),
			sha256Digest: digestOf(sha256.New(), packageFile.Contents),
		})
	}
	sort.Slice(indexed, func(first int, second int) bool { return indexed[first].poolPath < indexed[second].poolPath })
	return indexed, nil
}

// PoolPath is where a .deb lives, under the first letter of its source name as
// every Debian archive lays it out.
func PoolPath(packageName string, fileName string) string {
	return path.Join("pool", Component, packageName[:1], packageName, fileName)
}

type indexFile struct {
	relativePath string
	contents     []byte
}

func indexObjects(suite string, indexed []indexedPackage, objects map[string][]byte) []indexFile {
	var indices []indexFile
	for _, architecture := range Architectures {
		stanzas := renderPackagesIndex(indexed, architecture)
		binaryDirectory := path.Join(Component, "binary-"+architecture)
		for name, contents := range map[string][]byte{
			"Packages":    stanzas,
			"Packages.gz": compress(stanzas),
		} {
			relativePath := path.Join(binaryDirectory, name)
			indices = append(indices, indexFile{relativePath: relativePath, contents: contents})
			objects[path.Join(Prefix, "dists", suite, relativePath)] = contents
		}
	}
	sort.Slice(indices, func(first int, second int) bool {
		return indices[first].relativePath < indices[second].relativePath
	})
	return indices
}

// renderPackagesIndex writes the stanza apt reads for each package of one
// architecture. `all` is indexed under every architecture, which is what makes
// an architecture-independent package installable at all.
func renderPackagesIndex(indexed []indexedPackage, architecture string) []byte {
	var index bytes.Buffer
	for _, entry := range indexed {
		declared := entry.fields.Value("Architecture")
		if declared != architecture && declared != "all" {
			continue
		}
		if index.Len() > 0 {
			index.WriteString("\n")
		}
		index.Write(renderPackageStanza(entry))
	}
	return index.Bytes()
}

// descriptionIsLast keeps Description at the end of a stanza, because its
// folded continuation lines would otherwise swallow the field after it.
func renderPackageStanza(entry indexedPackage) []byte {
	var stanza bytes.Buffer
	for _, name := range entry.fields.Names() {
		if name == "Description" {
			continue
		}
		fmt.Fprintf(&stanza, "%s: %s\n", name, entry.fields.Value(name))
	}
	fmt.Fprintf(&stanza, "Filename: %s\n", entry.poolPath)
	fmt.Fprintf(&stanza, "Size: %d\n", entry.size)
	fmt.Fprintf(&stanza, "MD5sum: %s\n", entry.md5Digest)
	fmt.Fprintf(&stanza, "SHA256: %s\n", entry.sha256Digest)
	if description := entry.fields.Value("Description"); description != "" {
		fmt.Fprintf(&stanza, "Description: %s\n", description)
	}
	return stanza.Bytes()
}

// renderRelease writes the document the signature covers, and through it every
// index: apt verifies the signature on this file and then each index against
// the digest listed here.
//
// `Acquire-By-Hash: no` is stated rather than left to apt's default because
// the worker serving this repository has no by-hash objects to offer; saying
// so is what keeps apt from asking for one.
func renderRelease(suite string, indices []indexFile, now time.Time) []byte {
	var release bytes.Buffer
	fmt.Fprintf(&release, "Origin: %s\n", origin)
	fmt.Fprintf(&release, "Label: %s\n", label)
	fmt.Fprintf(&release, "Suite: %s\n", suite)
	fmt.Fprintf(&release, "Codename: %s\n", suite)
	fmt.Fprintf(&release, "Architectures: %s\n", strings.Join(Architectures, " "))
	fmt.Fprintf(&release, "Components: %s\n", Component)
	fmt.Fprintf(&release, "Date: %s\n", now.UTC().Format("Mon, 02 Jan 2006 15:04:05 UTC"))
	fmt.Fprintf(&release, "Acquire-By-Hash: no\n")
	fmt.Fprintf(&release, "Description: InternKim company host packages (%s)\n", suite)
	for _, algorithm := range []struct {
		name    string
		builder func() hash.Hash
	}{
		{name: "MD5Sum", builder: md5.New},
		{name: "SHA256", builder: sha256.New},
	} {
		fmt.Fprintf(&release, "%s:\n", algorithm.name)
		for _, index := range indices {
			fmt.Fprintf(&release, " %s %d %s\n", digestOf(algorithm.builder(), index.contents), len(index.contents), index.relativePath)
		}
	}
	return release.Bytes()
}

func compress(payload []byte) []byte {
	var compressed bytes.Buffer
	writer, _ := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	writer.Write(payload)
	writer.Close()
	return compressed.Bytes()
}

func digestOf(digest hash.Hash, payload []byte) string {
	digest.Write(payload)
	return hex.EncodeToString(digest.Sum(nil))
}

func isKnownSuite(suite string) bool {
	for _, known := range Suites {
		if known == suite {
			return true
		}
	}
	return false
}
