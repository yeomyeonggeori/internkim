package rpmrepository

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goreleaser/nfpm/v2"
	"github.com/goreleaser/nfpm/v2/files"
	_ "github.com/goreleaser/nfpm/v2/rpm"

	"gitlab.com/eastriver/internkim/internal/packagerepository"
	"gitlab.com/eastriver/internkim/internal/packagerepository/repositorytest"
)

var publishedAt = time.Date(2026, time.October, 1, 9, 30, 0, 0, time.UTC)

func buildTestPackage(t *testing.T, architecture string) packagerepository.Package {
	t.Helper()
	source := filepath.Join(t.TempDir(), "internkim")
	if errorValue := os.WriteFile(source, []byte("#!/bin/sh\n"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	information := nfpm.WithDefaults(&nfpm.Info{
		Name:        "internkim",
		Arch:        architecture,
		Platform:    "linux",
		Version:     "1.2.3",
		Release:     "1",
		Maintainer:  "InternKim <packages@example.com>",
		Description: "the company host",
		Homepage:    "https://intern.kim",
		License:     "Apache-2.0",
		Overridables: nfpm.Overridables{
			Depends: []string{"systemd", "postgresql-server >= 15"},
			Contents: files.Contents{
				{Source: source, Destination: "/usr/bin/internkim", FileInfo: &files.ContentFileInfo{Mode: 0o755}},
				{Source: source, Destination: "/etc/internkim/host.env", Type: files.TypeConfig, FileInfo: &files.ContentFileInfo{Mode: 0o644}},
			},
		},
	})
	packager, errorValue := nfpm.Get("rpm")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var built bytes.Buffer
	if errorValue := packager.Package(information, &built); errorValue != nil {
		t.Fatalf("build a test rpm: %v", errorValue)
	}
	return packagerepository.Package{FileName: packager.ConventionalFileName(information), Contents: built.Bytes()}
}

func newSigner(t *testing.T) *packagerepository.GPGSigner {
	t.Helper()
	signer, errorValue := packagerepository.NewGPGSigner(repositorytest.KeyPath(t))
	if errorValue != nil {
		t.Fatalf("open the signer: %v", errorValue)
	}
	t.Cleanup(signer.Close)
	return signer
}

func buildRepository(t *testing.T) (map[string][]byte, packagerepository.Package) {
	t.Helper()
	unsigned := buildTestPackage(t, "arm64")
	objects, errorValue := Build(packagerepository.DefaultChannel, []packagerepository.Package{unsigned}, newSigner(t), publishedAt)
	if errorValue != nil {
		t.Fatalf("build the repository: %v", errorValue)
	}
	return objects, unsigned
}

func gunzip(t *testing.T, compressed []byte) []byte {
	t.Helper()
	reader, errorValue := gzip.NewReader(bytes.NewReader(compressed))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return document
}

func TestAPackageNfpmBuiltIsUnsignedAndLeavesSignedAfterwards(t *testing.T) {
	objects, unsigned := buildRepository(t)
	if IsSigned(unsigned.Contents) {
		t.Fatal("the test package was signed before the repository signed it")
	}
	published := objects["rpm/stable/aarch64/Packages/"+unsigned.FileName]
	if !IsSigned(published) {
		t.Fatal("the package the repository publishes carries no header and body signature")
	}
	unsignedLayout, _, _ := splitPackage(unsigned.Contents)
	signedLayout, _, _ := splitPackage(published)
	unsignedMetadata, _ := readMetadata(unsigned.Contents)
	signedMetadata, _ := readMetadata(published)
	if !bytes.Equal(unsignedLayout.header, signedLayout.header) || !bytes.Equal(unsignedLayout.payload, signedLayout.payload) {
		t.Fatal("signing changed the header or the payload, which is what the signature covers")
	}
	if unsignedMetadata.name != signedMetadata.name || unsignedMetadata.version != signedMetadata.version {
		t.Fatal("signing changed what the package says it is")
	}
}

func TestBothSignaturesInThePackageVerifyAgainstThePublishedKey(t *testing.T) {
	objects, unsigned := buildRepository(t)
	layout, _, _ := splitPackage(objects["rpm/stable/aarch64/Packages/"+unsigned.FileName])
	publicKey := objects["rpm/stable/"+KeyName]

	repositorytest.Verify(t, publicKey, layout.signature.entries[signatureTagHeaderSignature].data, layout.header)
	repositorytest.Verify(t, publicKey, layout.signature.entries[signatureTagBodySignature].data, append(append([]byte{}, layout.header...), layout.payload...))
}

func TestTheSignatureHeaderKeepsTheDigestsRpmChecksBeforeTheSignature(t *testing.T) {
	objects, unsigned := buildRepository(t)
	published := objects["rpm/stable/aarch64/Packages/"+unsigned.FileName]
	layout, _, _ := splitPackage(published)
	for _, tag := range []int{273, signatureTagSize, signatureTagPayloadSize} {
		if _, present := layout.signature.entries[tag]; !present {
			t.Errorf("signature tag %d was dropped when the package was re-signed", tag)
		}
	}
	if layout.headerStartOffset(published)%signatureAlignment != 0 {
		t.Error("the header does not start on an 8-byte boundary")
	}
}

func (layout packageLayout) headerStartOffset(contents []byte) int {
	return len(contents) - len(layout.payload) - len(layout.header)
}

type repositoryMetadata struct {
	Data []struct {
		Type     string `xml:"type,attr"`
		Checksum string `xml:"checksum"`
		Open     string `xml:"open-checksum"`
		Location struct {
			Href string `xml:"href,attr"`
		} `xml:"location"`
		Size int `xml:"size"`
	} `xml:"data"`
}

func TestEveryMetadataFileIsListedInTheSignedRepomdWithItsDigest(t *testing.T) {
	objects, _ := buildRepository(t)
	var parsed repositoryMetadata
	if errorValue := xml.Unmarshal(objects["rpm/stable/aarch64/repodata/repomd.xml"], &parsed); errorValue != nil {
		t.Fatalf("repomd.xml is not XML: %v", errorValue)
	}
	types := []string{}
	for _, data := range parsed.Data {
		types = append(types, data.Type)
		file, present := objects["rpm/stable/aarch64/"+data.Location.Href]
		if !present {
			t.Fatalf("repomd.xml lists %s and it is not published", data.Location.Href)
		}
		if sha256Hex(file) != data.Checksum || len(file) != data.Size {
			t.Errorf("%s does not match the digest and size repomd.xml gives it", data.Location.Href)
		}
		if sha256Hex(gunzip(t, file)) != data.Open {
			t.Errorf("%s does not decompress to the open-checksum repomd.xml gives it", data.Location.Href)
		}
	}
	if strings.Join(types, ",") != "primary,filelists,other" {
		t.Errorf("repomd.xml lists %v", types)
	}
}

func TestRepomdIsSignedByTheKeyPublishedBesideIt(t *testing.T) {
	objects, _ := buildRepository(t)
	for _, architecture := range Architectures {
		directory := "rpm/stable/" + architecture + "/repodata/"
		repositorytest.Verify(t, objects["rpm/stable/"+KeyName], objects[directory+"repomd.xml.asc"], objects[directory+"repomd.xml"])
	}
}

type primaryMetadata struct {
	Packages int `xml:"packages,attr"`
	Package  []struct {
		Name     string `xml:"name"`
		Arch     string `xml:"arch"`
		Checksum string `xml:"checksum"`
		Location struct {
			Href string `xml:"href,attr"`
		} `xml:"location"`
		Format struct {
			Range struct {
				Start int `xml:"start,attr"`
				End   int `xml:"end,attr"`
			} `xml:"header-range"`
			Requires struct {
				Entries []struct {
					Name  string `xml:"name,attr"`
					Flags string `xml:"flags,attr"`
					Ver   string `xml:"ver,attr"`
				} `xml:"entry"`
			} `xml:"requires"`
			Files []string `xml:"file"`
		} `xml:"format"`
	} `xml:"package"`
}

func TestPrimaryNamesThePublishedFileByItsDigestAndItsHeaderByRange(t *testing.T) {
	objects, unsigned := buildRepository(t)
	var primary primaryMetadata
	if errorValue := xml.Unmarshal(gunzip(t, objects["rpm/stable/aarch64/repodata/primary.xml.gz"]), &primary); errorValue != nil {
		t.Fatalf("primary.xml is not XML: %v", errorValue)
	}
	if primary.Packages != 1 || len(primary.Package) != 1 {
		t.Fatalf("primary holds %d packages", len(primary.Package))
	}
	entry := primary.Package[0]
	published := objects["rpm/stable/aarch64/"+entry.Location.Href]
	if entry.Location.Href != "Packages/"+unsigned.FileName || sha256Hex(published) != entry.Checksum {
		t.Errorf("primary points at %s with digest %s", entry.Location.Href, entry.Checksum)
	}
	if entry.Name != "internkim" || entry.Arch != "aarch64" {
		t.Errorf("primary names %s.%s", entry.Name, entry.Arch)
	}
	if !bytes.HasPrefix(published[entry.Format.Range.Start:entry.Format.Range.End], headerMagic[:4]) {
		t.Error("header-range does not start at the package header")
	}
	requirements := map[string]string{}
	for _, required := range entry.Format.Requires.Entries {
		requirements[required.Name] = required.Flags + " " + required.Ver
	}
	if requirements["postgresql-server"] != "GE 15" {
		t.Errorf("the versioned requirement was written as %q", requirements["postgresql-server"])
	}
	for name := range requirements {
		if strings.HasPrefix(name, "rpmlib(") {
			t.Errorf("primary lists %s, which no repository provides", name)
		}
	}
	listed := strings.Join(entry.Format.Files, " ")
	if !strings.Contains(listed, "/usr/bin/internkim") || !strings.Contains(listed, "/etc/internkim/host.env") {
		t.Errorf("primary lists files %v", entry.Format.Files)
	}
}

func TestAnArchitectureWithNoPackageAnswersWithAnEmptyRepository(t *testing.T) {
	objects, _ := buildRepository(t)
	var primary primaryMetadata
	if errorValue := xml.Unmarshal(gunzip(t, objects["rpm/stable/x86_64/repodata/primary.xml.gz"]), &primary); errorValue != nil {
		t.Fatal(errorValue)
	}
	if primary.Packages != 0 {
		t.Errorf("x86_64 has %d packages", primary.Packages)
	}
	if _, present := objects["rpm/stable/x86_64/repodata/repomd.xml.asc"]; !present {
		t.Error("the empty architecture's repomd.xml is unsigned")
	}
}

func TestAPackageForAnArchitectureNotPublishedIsRefused(t *testing.T) {
	_, errorValue := Build(packagerepository.DefaultChannel, []packagerepository.Package{buildTestPackage(t, "riscv64")}, newSigner(t), publishedAt)
	if errorValue == nil {
		t.Fatal("a package for riscv64 was published to a repository that has no such architecture")
	}
}

func TestAnUnsignedRepositoryAndAnUnknownChannelAreRefused(t *testing.T) {
	if _, errorValue := Build(packagerepository.DefaultChannel, nil, nil, publishedAt); errorValue == nil {
		t.Error("a repository was built with no signer")
	}
	if _, errorValue := Build("unstable", nil, newSigner(t), publishedAt); errorValue == nil {
		t.Error("an unpublished channel was accepted")
	}
}

func TestEachChannelKeepsItsOwnTree(t *testing.T) {
	objects, errorValue := Build(packagerepository.TestingChannel, []packagerepository.Package{buildTestPackage(t, "amd64")}, newSigner(t), publishedAt)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for objectKey := range objects {
		if !strings.HasPrefix(objectKey, "rpm/testing/") {
			t.Errorf("%s is outside the testing tree", objectKey)
		}
	}
}
