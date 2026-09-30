package cli

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/aptrepository"
	"gitlab.com/eastriver/internkim/internal/pacmanrepository"
	"gitlab.com/eastriver/internkim/internal/rpmrepository"
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

func TestEveryFormatIsBuiltForTheSameChannelsAndTheBuilderArchitectures(t *testing.T) {
	built := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}
	for _, target := range debianTargets {
		if !slices.Contains(rpmrepository.Architectures, built[target.DebianArchitecture]) || !slices.Contains(pacmanrepository.Architectures, built[target.DebianArchitecture]) {
			t.Errorf("the builder builds %s and the rpm or pacman repository does not publish it", target.DebianArchitecture)
		}
	}
	if len(rpmrepository.Architectures) != len(debianTargets) || len(pacmanrepository.Architectures) != len(debianTargets) {
		t.Error("a repository publishes an architecture the builder never builds")
	}
}

func TestEveryRepositoryFormatIsOneThePackagesStepBuilds(t *testing.T) {
	for _, format := range repositoryFormats {
		if !slices.ContainsFunc(linuxPackageFormats(), func(built linuxPackageFormat) bool { return built.Name == format.name }) {
			t.Errorf("%s has a repository and release packages does not build it", format.name)
		}
	}
	if len(repositoryFormats) != len(linuxPackageFormats()) {
		t.Error("release packages builds a format no repository publishes")
	}
}

func TestTheReleaseRegistryServesEveryRepositoryPrefix(t *testing.T) {
	registry, errorValue := os.ReadFile(filepath.Join("..", "..", "workers", "release-registry", "src", "registry.ts"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, format := range repositoryFormats {
		if !strings.Contains(string(registry), "'"+format.prefix+"/'") {
			t.Errorf("the release registry does not serve %s/", format.prefix)
		}
	}
}

func TestTheInstallScriptNamesTheKeysTheRepositoriesPublish(t *testing.T) {
	script, errorValue := os.ReadFile(filepath.Join("..", "..", "web", "static", "install.sh"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, wanted := range []string{
		`rpm_repository_key_name="` + rpmrepository.KeyName + `"`,
		`pacman_repository_key_name="` + pacmanrepository.KeyName + `"`,
		`$repository_base_url/` + rpmrepository.Prefix + `/$repository_channel`,
		`$repository_base_url/` + pacmanrepository.Prefix + `/$repository_channel`,
		`$repository_base_url/` + aptrepository.Prefix,
	} {
		if !strings.Contains(string(script), wanted) {
			t.Errorf("install.sh does not contain %s", wanted)
		}
	}
}

func TestADirectoryMissingAFormatIsRefusedUnlessTheFormatIsNamedOut(t *testing.T) {
	directory := t.TempDir()
	if _, errorValue := readPackageDirectory(directory, repositoryFormats); errorValue == nil {
		t.Fatal("an empty package directory was accepted")
	}
	if errorValue := os.WriteFile(filepath.Join(directory, "internkim_1.0.0_arm64.deb"), []byte("deb"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := readPackageDirectory(directory, repositoryFormats); errorValue == nil {
		t.Fatal("a directory with no rpm and no pacman package was accepted for all three")
	}
	formats, errorValue := repositoryFormatsNamed("deb")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	packages, errorValue := readPackageDirectory(directory, formats)
	if errorValue != nil || len(packages["deb"]) != 1 {
		t.Fatalf("the deb alone was not read: %v", errorValue)
	}
	if _, errorValue := repositoryFormatsNamed("tarball"); errorValue == nil {
		t.Fatal("an unknown format was accepted")
	}
}

func TestEachPublishedObjectCarriesTheTypeItIs(t *testing.T) {
	for objectKey, wanted := range map[string]string{
		"deb/pool/main/i/internkim/internkim_1.0.0_arm64.deb":       "application/vnd.debian.binary-package",
		"deb/dists/testing/main/binary-arm64/Packages.gz":           "application/gzip",
		"deb/dists/testing/Release.gpg":                             "application/pgp-signature",
		"deb/internkim-archive-keyring.pgp":                         "application/pgp-keys",
		"deb/dists/testing/InRelease":                               "text/plain; charset=utf-8",
		"rpm/stable/aarch64/Packages/internkim-1.0.0-1.aarch64.rpm": "application/x-rpm",
		"rpm/stable/aarch64/repodata/repomd.xml":                    "application/xml",
		"rpm/stable/aarch64/repodata/repomd.xml.asc":                "application/pgp-signature",
		"rpm/stable/aarch64/repodata/primary.xml.gz":                "application/gzip",
		"rpm/stable/internkim-rpm-signing.asc":                      "application/pgp-keys",
		"arch/stable/aarch64/internkim-1.0.0-1-aarch64.pkg.tar.zst": "application/zstd",
		"arch/stable/aarch64/internkim.db":                          "application/gzip",
		"arch/stable/aarch64/internkim.db.sig":                      "application/pgp-signature",
		"arch/stable/internkim-pacman-signing.asc":                  "application/pgp-keys",
	} {
		if given := repositoryContentType(objectKey); given != wanted {
			t.Errorf("%s is published as %s, wanted %s", objectKey, given, wanted)
		}
	}
}
