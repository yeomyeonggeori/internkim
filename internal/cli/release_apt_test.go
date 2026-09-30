package cli

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"gitlab.com/eastriver/internkim/internal/aptrepository"
)

// The repository declares which architectures a suite publishes and the
// builder declares which it builds. An architecture in the Release that no
// package is ever built for is an index apt fetches and finds empty; one built
// but not declared is a package nothing can install.
func TestTheSuitePublishesExactlyTheArchitecturesTheBuilderBuilds(t *testing.T) {
	built := make([]string, 0, len(debianTargets))
	for _, target := range debianTargets {
		built = append(built, target.DebianArchitecture)
	}
	published := append([]string(nil), aptrepository.Architectures...)
	sort.Strings(built)
	sort.Strings(published)
	if len(built) != len(published) {
		t.Fatalf("the builder builds %v and the suite publishes %v", built, published)
	}
	for index := range built {
		if built[index] != published[index] {
			t.Fatalf("the builder builds %v and the suite publishes %v", built, published)
		}
	}
}

// `release deb` writes its packages where `release apt` reads them, and the
// path is written once.
func TestTheRepositoryIsBuiltFromWhereThePackagesAreWritten(t *testing.T) {
	directory := t.TempDir()
	if _, errorValue := readPackageDirectory(directory); errorValue == nil {
		t.Fatal("an empty package directory was accepted")
	}
	if errorValue := os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("not a package"), 0o644); errorValue != nil {
		t.Fatalf("write a decoy: %v", errorValue)
	}
	if _, errorValue := readPackageDirectory(directory); errorValue == nil {
		t.Fatal("a directory holding no .deb was accepted")
	}
	if debDefaultOutputDirectory == "" {
		t.Fatal("release apt reads a directory release deb does not name")
	}
}

func TestEachPublishedObjectCarriesTheTypeItIs(t *testing.T) {
	for objectKey, wanted := range map[string]string{
		"deb/pool/main/i/internkim/internkim_1.0.0_arm64.deb": "application/vnd.debian.binary-package",
		"deb/dists/testing/main/binary-arm64/Packages.gz":     "application/gzip",
		"deb/dists/testing/Release.gpg":                       "application/pgp-signature",
		"deb/internkim-archive-keyring.pgp":                   "application/pgp-keys",
		"deb/dists/testing/InRelease":                         "text/plain; charset=utf-8",
	} {
		if given := aptContentType(objectKey); given != wanted {
			t.Errorf("%s is published as %s, wanted %s", objectKey, given, wanted)
		}
	}
}
