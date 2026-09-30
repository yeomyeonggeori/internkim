package pacmanrepository

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	Prefix = "arch"

	// KeyName is the public half of the signing key, beside the architectures
	// of a channel so the install script can name it from the repository URL.
	KeyName = "internkim-pacman-signing.asc"

	// RepositoryName is the section name in pacman.conf, and so the stem of
	// the database pacman fetches.
	RepositoryName = "internkim"
)

// Architectures are the two the company host is built for, spelled as
// pacman's $arch spells them.
var Architectures = []string{"x86_64", "aarch64"}

type indexedPackage struct {
	packageFile packagerepository.Package
	information packageInformation
	signature   []byte
}

// Build renders every object of a one-channel pacman repository, keyed by the
// object name it takes in the release bucket: each package with its detached
// signature, and per architecture the .db and .files databases repo-add would
// write, each with its own signature. A database that is not signed is refused
// by `SigLevel = Required`, so it is never published unsigned.
func Build(channel string, packages []packagerepository.Package, signer packagerepository.Signer, now time.Time) (map[string][]byte, error) {
	if errorValue := packagerepository.CheckChannel(channel); errorValue != nil {
		return nil, errorValue
	}
	if signer == nil {
		return nil, fmt.Errorf("a pacman repository is signed; no signer was given")
	}
	objects := map[string][]byte{}
	indexed := map[string][]indexedPackage{}
	for _, packageFile := range packages {
		information, errorValue := readPackage(packageFile.Contents)
		if errorValue != nil {
			return nil, fmt.Errorf("read %s: %w", packageFile.FileName, errorValue)
		}
		signature, errorValue := signer.DetachSignBinary(packageFile.Contents)
		if errorValue != nil {
			return nil, fmt.Errorf("sign %s: %w", packageFile.FileName, errorValue)
		}
		architecture := information.value("arch")
		if errorValue := packagerepository.CheckArchitecture("pacman", packageFile.FileName, architecture, Architectures); errorValue != nil {
			return nil, errorValue
		}
		directory := architectureDirectory(channel, architecture)
		objects[path.Join(directory, packageFile.FileName)] = packageFile.Contents
		objects[path.Join(directory, packageFile.FileName+".sig")] = signature
		indexed[architecture] = append(indexed[architecture], indexedPackage{packageFile: packageFile, information: information, signature: signature})
	}
	for _, architecture := range Architectures {
		if errorValue := addDatabases(objects, channel, architecture, indexed[architecture], signer, now); errorValue != nil {
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

func addDatabases(objects map[string][]byte, channel string, architecture string, entries []indexedPackage, signer packagerepository.Signer, now time.Time) error {
	sort.Slice(entries, func(first int, second int) bool {
		return entries[first].packageFile.FileName < entries[second].packageFile.FileName
	})
	for index := 1; index < len(entries); index++ {
		if entries[index].information.value("pkgname") == entries[index-1].information.value("pkgname") {
			return fmt.Errorf("%s and %s are both %s for %s; a pacman database holds one version of a package",
				entries[index-1].packageFile.FileName, entries[index].packageFile.FileName, entries[index].information.value("pkgname"), architecture)
		}
	}
	directory := architectureDirectory(channel, architecture)
	for extension, withFiles := range map[string]bool{"db": false, "files": true} {
		database, errorValue := renderDatabase(entries, withFiles)
		if errorValue != nil {
			return errorValue
		}
		name := RepositoryName + "." + extension
		signature, errorValue := signer.DetachSignBinary(database)
		if errorValue != nil {
			return fmt.Errorf("sign %s of %s/%s: %w", name, channel, architecture, errorValue)
		}
		objects[path.Join(directory, name)] = database
		objects[path.Join(directory, name+".sig")] = signature
	}
	return nil
}

func renderDatabase(entries []indexedPackage, withFiles bool) ([]byte, error) {
	var compressed bytes.Buffer
	compressor, _ := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	archive := tar.NewWriter(compressor)
	for _, entry := range entries {
		directory := entry.information.value("pkgname") + "-" + entry.information.value("pkgver")
		documents := []struct{ name, body string }{{"desc", renderDescription(entry)}}
		if withFiles {
			documents = append(documents, struct{ name, body string }{"files", renderFileList(entry)})
		}
		if errorValue := archive.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: directory + "/", Mode: 0o755}); errorValue != nil {
			return nil, errorValue
		}
		for _, document := range documents {
			header := &tar.Header{Name: directory + "/" + document.name, Mode: 0o644, Size: int64(len(document.body))}
			if errorValue := archive.WriteHeader(header); errorValue != nil {
				return nil, errorValue
			}
			if _, errorValue := archive.Write([]byte(document.body)); errorValue != nil {
				return nil, errorValue
			}
		}
	}
	if errorValue := archive.Close(); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := compressor.Close(); errorValue != nil {
		return nil, errorValue
	}
	return compressed.Bytes(), nil
}

func renderDescription(entry indexedPackage) string {
	information := entry.information
	contents := entry.packageFile.Contents
	var description strings.Builder
	section := func(name string, values ...string) {
		kept := make([]string, 0, len(values))
		for _, value := range values {
			if value != "" {
				kept = append(kept, value)
			}
		}
		if len(kept) == 0 {
			return
		}
		fmt.Fprintf(&description, "%%%s%%\n%s\n\n", name, strings.Join(kept, "\n"))
	}
	section("FILENAME", entry.packageFile.FileName)
	section("NAME", information.value("pkgname"))
	section("BASE", firstNonEmpty(information.value("pkgbase"), information.value("pkgname")))
	section("VERSION", information.value("pkgver"))
	section("DESC", information.value("pkgdesc"))
	section("GROUPS", information.fields["group"]...)
	section("CSIZE", fmt.Sprint(len(contents)))
	section("ISIZE", fmt.Sprint(information.size()))
	section("MD5SUM", digest(md5.New(), contents))
	section("SHA256SUM", digest(sha256.New(), contents))
	section("PGPSIG", base64.StdEncoding.EncodeToString(entry.signature))
	section("URL", information.value("url"))
	section("LICENSE", information.fields["license"]...)
	section("ARCH", information.value("arch"))
	section("BUILDDATE", information.value("builddate"))
	section("PACKAGER", information.value("packager"))
	section("REPLACES", information.fields["replaces"]...)
	section("CONFLICTS", information.fields["conflict"]...)
	section("PROVIDES", information.fields["provides"]...)
	section("DEPENDS", information.fields["depend"]...)
	section("OPTDEPENDS", information.fields["optdepend"]...)
	return description.String()
}

func renderFileList(entry indexedPackage) string {
	var list strings.Builder
	list.WriteString("%FILES%\n")
	for _, file := range entry.information.files {
		list.WriteString(file + "\n")
	}
	list.WriteString("\n")
	if backups := entry.information.fields["backup"]; len(backups) > 0 {
		list.WriteString("%BACKUP%\n")
		for _, backup := range backups {
			fmt.Fprintf(&list, "%s\t\n", backup)
		}
		list.WriteString("\n")
	}
	return list.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func digest(hasher interface {
	Write([]byte) (int, error)
	Sum([]byte) []byte
}, payload []byte) string {
	hasher.Write(payload)
	return hex.EncodeToString(hasher.Sum(nil))
}
