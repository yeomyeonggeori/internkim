package aptrepository

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/packagerepository"
)

var publishedAt = time.Date(2026, time.September, 22, 9, 30, 0, 0, time.UTC)

type recordingSigner struct {
	clearSigned []byte
	detached    []byte
}

func (signer *recordingSigner) ClearSign(document []byte) ([]byte, error) {
	signer.clearSigned = document
	return append([]byte("-----BEGIN PGP SIGNED MESSAGE-----\n\n"), document...), nil
}

func (signer *recordingSigner) DetachSign(document []byte) ([]byte, error) {
	signer.detached = document
	return []byte("-----BEGIN PGP SIGNATURE-----\n"), nil
}

func (signer *recordingSigner) DetachSignBinary(document []byte) ([]byte, error) {
	return []byte("binary-signature"), nil
}

func (signer *recordingSigner) PublicKeyArmoured() ([]byte, error) {
	return []byte("armoured-key"), nil
}

func (signer *recordingSigner) PublicKeyring() ([]byte, error) {
	return []byte("public-keyring"), nil
}

// buildTestPackage writes the smallest thing that is still a .deb: an ar
// archive whose control member carries the paragraph apt indexes.
func buildTestPackage(t *testing.T, fields string) []byte {
	t.Helper()
	var controlArchive bytes.Buffer
	compressor := gzip.NewWriter(&controlArchive)
	archive := tar.NewWriter(compressor)
	if errorValue := archive.WriteHeader(&tar.Header{Name: "./control", Mode: 0o644, Size: int64(len(fields))}); errorValue != nil {
		t.Fatalf("write the control header: %v", errorValue)
	}
	if _, errorValue := archive.Write([]byte(fields)); errorValue != nil {
		t.Fatalf("write the control member: %v", errorValue)
	}
	archive.Close()
	compressor.Close()

	var packageFile bytes.Buffer
	packageFile.WriteString(arFileMagic)
	for _, member := range []struct {
		name    string
		payload []byte
	}{
		{name: "debian-binary", payload: []byte("2.0\n")},
		{name: "control.tar.gz", payload: controlArchive.Bytes()},
		{name: "data.tar.gz", payload: []byte("data")},
	} {
		fmt.Fprintf(&packageFile, "%-16s%-12d%-6d%-6d%-8s%-10d`\n", member.name, 0, 0, 0, "100644", len(member.payload))
		packageFile.Write(member.payload)
		if len(member.payload)%2 == 1 {
			packageFile.WriteString("\n")
		}
	}
	return packageFile.Bytes()
}

func hostPackage(t *testing.T, version string, architecture string) packagerepository.Package {
	t.Helper()
	fields := fmt.Sprintf(
		"Package: internkim\nVersion: %s\nArchitecture: %s\nMaintainer: InternKim <support@example.com>\n"+
			"Installed-Size: 1024\nDepends: postgresql (>= 14), redis-server\nSection: admin\nPriority: optional\n"+
			"Description: the company host\n it runs the agent.\n",
		version, architecture)
	return packagerepository.Package{
		FileName: fmt.Sprintf("internkim_%s_%s.deb", version, architecture),
		Contents: buildTestPackage(t, fields),
	}
}

func TestPackagesIndexDescribesTheFileAptWillFetch(t *testing.T) {
	packageFile := hostPackage(t, "1.0.0", "arm64")
	objects, errorValue := Build(packagerepository.DefaultChannel, []packagerepository.Package{packageFile}, &recordingSigner{}, publishedAt)
	if errorValue != nil {
		t.Fatalf("build the repository: %v", errorValue)
	}

	index := string(objects["deb/dists/"+packagerepository.DefaultChannel+"/main/binary-arm64/Packages"])
	expectedPoolPath := "pool/main/i/internkim/internkim_1.0.0_arm64.deb"
	digest := sha256.Sum256(packageFile.Contents)
	for _, wanted := range []string{
		"Package: internkim",
		"Version: 1.0.0",
		"Filename: " + expectedPoolPath,
		fmt.Sprintf("Size: %d", len(packageFile.Contents)),
		"SHA256: " + hex.EncodeToString(digest[:]),
		"Depends: postgresql (>= 14), redis-server",
	} {
		if !strings.Contains(index, wanted) {
			t.Errorf("the Packages index does not carry %q:\n%s", wanted, index)
		}
	}
	if _, present := objects["deb/"+expectedPoolPath]; !present {
		t.Errorf("the pool does not hold %s; the index names a file that is not published", expectedPoolPath)
	}
}

// A stanza whose Description came before another field would swallow it: the
// folded continuation lines belong to whatever field precedes them.
func TestFoldedDescriptionIsTheLastFieldOfAStanza(t *testing.T) {
	objects, errorValue := Build(packagerepository.DefaultChannel, []packagerepository.Package{hostPackage(t, "1.0.0", "arm64")}, &recordingSigner{}, publishedAt)
	if errorValue != nil {
		t.Fatalf("build the repository: %v", errorValue)
	}
	index := string(objects["deb/dists/"+packagerepository.DefaultChannel+"/main/binary-arm64/Packages"])
	lines := strings.Split(strings.TrimRight(index, "\n"), "\n")
	if lines[len(lines)-1] != " it runs the agent." {
		t.Fatalf("the stanza ends with %q, so a field follows the folded Description:\n%s", lines[len(lines)-1], index)
	}
}

func TestEveryDeclaredArchitectureGetsAnIndexEvenWithNoPackage(t *testing.T) {
	objects, errorValue := Build(packagerepository.DefaultChannel, []packagerepository.Package{hostPackage(t, "1.0.0", "arm64")}, &recordingSigner{}, publishedAt)
	if errorValue != nil {
		t.Fatalf("build the repository: %v", errorValue)
	}
	for _, architecture := range Architectures {
		if _, present := objects[fmt.Sprintf("deb/dists/%s/main/binary-%s/Packages", packagerepository.DefaultChannel, architecture)]; !present {
			t.Errorf("no index for %s; apt refuses a repository whose Release names an architecture it cannot fetch", architecture)
		}
	}
	if length := len(objects["deb/dists/"+packagerepository.DefaultChannel+"/main/binary-amd64/Packages"]); length != 0 {
		t.Errorf("the amd64 index is %d bytes, wanted an empty one", length)
	}
}

func TestAPackageForAnUnpublishedArchitectureIsRefused(t *testing.T) {
	if _, errorValue := Build(packagerepository.DefaultChannel, []packagerepository.Package{hostPackage(t, "1.0.0", "riscv64")}, &recordingSigner{}, publishedAt); errorValue == nil {
		t.Fatal("a riscv64 package was accepted, and would sit in the pool with no index naming it")
	}
}

// The Release is what the signature covers; every index is trusted only
// through the digest listed here, so a digest that does not match the bytes
// published beside it is the repository failing open.
func TestTheReleaseDigestsMatchThePublishedIndices(t *testing.T) {
	objects, errorValue := Build(packagerepository.DefaultChannel, []packagerepository.Package{hostPackage(t, "1.0.0", "arm64")}, &recordingSigner{}, publishedAt)
	if errorValue != nil {
		t.Fatalf("build the repository: %v", errorValue)
	}
	release := string(objects["deb/dists/"+packagerepository.DefaultChannel+"/Release"])
	listed := 0
	for _, line := range strings.Split(release, "\n") {
		if !strings.HasPrefix(line, " ") {
			continue
		}
		columns := strings.Fields(line)
		if len(columns) != 3 || len(columns[0]) != 64 {
			continue
		}
		published, present := objects["deb/dists/"+packagerepository.DefaultChannel+"/"+columns[2]]
		if !present {
			t.Fatalf("the Release lists %s, which is not published", columns[2])
		}
		digest := sha256.Sum256(published)
		if hex.EncodeToString(digest[:]) != columns[0] {
			t.Errorf("the Release digest of %s does not match its bytes", columns[2])
		}
		listed++
	}
	if listed != len(Architectures)*2 {
		t.Fatalf("the Release lists %d SHA256 indices, wanted %d", listed, len(Architectures)*2)
	}
}

// apt asks for by-hash objects only when the Release advertises them, and the
// worker serving this repository publishes none, so the field is stated rather
// than left to a default that could move.
func TestTheReleaseTurnsByHashOff(t *testing.T) {
	objects, errorValue := Build(packagerepository.DefaultChannel, []packagerepository.Package{hostPackage(t, "1.0.0", "arm64")}, &recordingSigner{}, publishedAt)
	if errorValue != nil {
		t.Fatalf("build the repository: %v", errorValue)
	}
	release := string(objects["deb/dists/"+packagerepository.DefaultChannel+"/Release"])
	if !strings.Contains(release, "Acquire-By-Hash: no\n") {
		t.Fatalf("the Release does not turn by-hash off:\n%s", release)
	}
	if strings.Contains(release, "Packages.diff") {
		t.Fatal("the Release names a pdiff index, which nothing publishes")
	}
}

func TestBothSignaturesCoverTheReleaseThatWasPublished(t *testing.T) {
	signer := &recordingSigner{}
	objects, errorValue := Build(packagerepository.DefaultChannel, []packagerepository.Package{hostPackage(t, "1.0.0", "arm64")}, signer, publishedAt)
	if errorValue != nil {
		t.Fatalf("build the repository: %v", errorValue)
	}
	release := objects["deb/dists/"+packagerepository.DefaultChannel+"/Release"]
	if !bytes.Equal(signer.clearSigned, release) {
		t.Error("InRelease was signed over something other than the published Release")
	}
	if !bytes.Equal(signer.detached, release) {
		t.Error("Release.gpg was signed over something other than the published Release")
	}
	if _, present := objects["deb/"+KeyringName]; !present {
		t.Error("the public keyring is not published beside the repository")
	}
}

func TestATestingSuiteIsPublishedBesideStable(t *testing.T) {
	testingSuite := packagerepository.TestingChannel
	objects, errorValue := Build(testingSuite, []packagerepository.Package{hostPackage(t, "1.1.0~rc1", "arm64")}, &recordingSigner{}, publishedAt)
	if errorValue != nil {
		t.Fatalf("build the testing suite: %v", errorValue)
	}
	if !strings.Contains(string(objects["deb/dists/"+testingSuite+"/Release"]), "Suite: "+testingSuite+"\n") {
		t.Error("the testing suite's Release does not name it")
	}
	if _, errorValue := Build("unstable", nil, &recordingSigner{}, publishedAt); errorValue == nil {
		t.Error("an unpublished suite name was accepted")
	}
}

func TestASuiteNamedForADistributionIsRefused(t *testing.T) {
	for _, named := range []string{"trixie-stable", "noble-stable", "trixie-testing"} {
		if _, errorValue := Build(named, nil, &recordingSigner{}, publishedAt); errorValue == nil {
			t.Errorf("suite %q was accepted though the package names no distribution", named)
		}
	}
}

func TestAnUnsignedRepositoryIsRefusedAtTheSource(t *testing.T) {
	if _, errorValue := Build(packagerepository.DefaultChannel, []packagerepository.Package{hostPackage(t, "1.0.0", "arm64")}, nil, publishedAt); errorValue == nil {
		t.Fatal("a repository was built with no signer; apt would refuse what this would publish")
	}
}
