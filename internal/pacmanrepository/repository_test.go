package pacmanrepository

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goreleaser/nfpm/v2"
	_ "github.com/goreleaser/nfpm/v2/arch"
	"github.com/goreleaser/nfpm/v2/files"

	"gitlab.com/eastriver/internkim/internal/packagerepository"
	"gitlab.com/eastriver/internkim/internal/packagerepository/repositorytest"
)

var publishedAt = time.Date(2026, time.October, 1, 9, 30, 0, 0, time.UTC)

func buildTestPackage(t *testing.T, architecture string, version string) packagerepository.Package {
	t.Helper()
	source := filepath.Join(t.TempDir(), "internkim")
	if errorValue := os.WriteFile(source, []byte("#!/bin/sh\n"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	information := &nfpm.Info{
		Name:        "internkim",
		Arch:        architecture,
		Platform:    "linux",
		Version:     version,
		Maintainer:  "InternKim <packages@example.com>",
		Description: "the company host",
		Homepage:    "https://intern.kim",
		License:     "Apache-2.0",
		Overridables: nfpm.Overridables{
			Depends: []string{"systemd", "postgresql"},
			Contents: files.Contents{
				{Source: source, Destination: "/usr/bin/internkim", FileInfo: &files.ContentFileInfo{Mode: 0o755}},
			},
		},
	}
	packager, errorValue := nfpm.Get("archlinux")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	information = nfpm.WithDefaults(information)
	if errorValue := nfpm.PrepareForPackager(information, "archlinux"); errorValue != nil {
		t.Fatal(errorValue)
	}
	var built bytes.Buffer
	if errorValue := packager.Package(information, &built); errorValue != nil {
		t.Fatalf("build a test package: %v", errorValue)
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
	built := buildTestPackage(t, "arm64", "1.2.3")
	objects, errorValue := Build(packagerepository.DefaultChannel, []packagerepository.Package{built}, newSigner(t), publishedAt)
	if errorValue != nil {
		t.Fatalf("build the repository: %v", errorValue)
	}
	return objects, built
}

func databaseEntries(t *testing.T, database []byte) map[string]string {
	t.Helper()
	decompressed, errorValue := gzip.NewReader(bytes.NewReader(database))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	reader := tar.NewReader(decompressed)
	entries := map[string]string{}
	for {
		header, errorValue := reader.Next()
		if errorValue == io.EOF {
			return entries
		}
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		body, _ := io.ReadAll(reader)
		entries[header.Name] = string(body)
	}
}

func TestEveryPackageAndDatabaseIsPublishedWithASignatureThatVerifies(t *testing.T) {
	objects, built := buildRepository(t)
	publicKey := objects["arch/stable/"+KeyName]
	for _, name := range []string{built.FileName, "internkim.db", "internkim.files"} {
		signature, present := objects["arch/stable/aarch64/"+name+".sig"]
		if !present {
			t.Fatalf("%s is published with no .sig", name)
		}
		repositorytest.Verify(t, publicKey, signature, objects["arch/stable/aarch64/"+name])
	}
}

func TestTheDatabaseDescribesThePackageTheWayRepoAddDoes(t *testing.T) {
	objects, built := buildRepository(t)
	entries := databaseEntries(t, objects["arch/stable/aarch64/internkim.db"])
	var description string
	for name, body := range entries {
		if strings.HasSuffix(name, "/desc") {
			description = body
		}
	}
	digest := sha256.Sum256(built.Contents)
	signature := base64.StdEncoding.EncodeToString(objects["arch/stable/aarch64/"+built.FileName+".sig"])
	for _, wanted := range []string{
		"%FILENAME%\n" + built.FileName + "\n",
		"%NAME%\ninternkim\n",
		"%SHA256SUM%\n" + hex.EncodeToString(digest[:]) + "\n",
		"%PGPSIG%\n" + signature + "\n",
		"%ARCH%\naarch64\n",
		"%DEPENDS%\n",
	} {
		if !strings.Contains(description, wanted) {
			t.Errorf("the description lacks %q:\n%s", wanted, description)
		}
	}
}

func TestTheFilesDatabaseListsThePackagesFiles(t *testing.T) {
	objects, _ := buildRepository(t)
	found := false
	for name, body := range databaseEntries(t, objects["arch/stable/aarch64/internkim.files"]) {
		if strings.HasSuffix(name, "/files") && strings.Contains(body, "usr/bin/internkim\n") {
			found = true
		}
	}
	if !found {
		t.Error("the files database does not list usr/bin/internkim")
	}
}

func TestAnArchitectureWithNoPackageGetsAnEmptySignedDatabase(t *testing.T) {
	objects, _ := buildRepository(t)
	if entries := databaseEntries(t, objects["arch/stable/x86_64/internkim.db"]); len(entries) != 0 {
		t.Errorf("x86_64 lists %v", entries)
	}
	if _, present := objects["arch/stable/x86_64/internkim.db.sig"]; !present {
		t.Error("the empty database is unsigned")
	}
}

func TestTwoVersionsOfOnePackageAreRefused(t *testing.T) {
	packages := []packagerepository.Package{buildTestPackage(t, "arm64", "1.0.0"), buildTestPackage(t, "arm64", "1.1.0")}
	if _, errorValue := Build(packagerepository.DefaultChannel, packages, newSigner(t), publishedAt); errorValue == nil {
		t.Fatal("a database holding two versions of one package was built")
	}
}

func TestAnUnsignedRepositoryAnUnknownChannelAndAnUnpublishedArchitectureAreRefused(t *testing.T) {
	if _, errorValue := Build(packagerepository.DefaultChannel, nil, nil, publishedAt); errorValue == nil {
		t.Error("a repository was built with no signer")
	}
	if _, errorValue := Build("unstable", nil, newSigner(t), publishedAt); errorValue == nil {
		t.Error("an unpublished channel was accepted")
	}
	riscv := []packagerepository.Package{buildTestPackage(t, "riscv64", "1.0.0")}
	if _, errorValue := Build(packagerepository.DefaultChannel, riscv, newSigner(t), publishedAt); errorValue == nil {
		t.Error("a riscv64 package was published")
	}
}
