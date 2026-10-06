package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/goreleaser/nfpm/v2"
	"github.com/goreleaser/nfpm/v2/files"
	"github.com/klauspost/compress/zstd"
)

func writtenPackage(t *testing.T, format linuxPackageFormat, version string) []byte {
	t.Helper()
	information := linuxPackageInformation(format, packageTargets[0], version, files.Contents{}, nfpm.Scripts{})
	information.Deb.Compression = "gzip"
	format.CompressPayload = nil
	packagePath := filepath.Join(t.TempDir(), format.assetName(packageTargets[0].Architecture))
	if errorValue := writeLinuxPackage(format, information, packagePath); errorValue != nil {
		t.Fatal(errorValue)
	}
	contents, errorValue := os.ReadFile(packagePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return contents
}

func arMember(t *testing.T, archive []byte, namePrefix string) []byte {
	t.Helper()
	offset := len("!<arch>\n")
	for offset+60 <= len(archive) {
		header := archive[offset : offset+60]
		size, errorValue := strconv.Atoi(strings.TrimSpace(string(header[48:58])))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		body := archive[offset+60 : offset+60+size]
		if strings.HasPrefix(string(header[:16]), namePrefix) {
			return body
		}
		offset += 60 + size + size%2
	}
	t.Fatalf("no %s in the archive", namePrefix)
	return nil
}

func tarMember(t *testing.T, archive io.Reader, name string) string {
	t.Helper()
	reader := tar.NewReader(archive)
	for {
		header, errorValue := reader.Next()
		if errorValue != nil {
			t.Fatalf("no %s in the archive: %v", name, errorValue)
		}
		if strings.TrimPrefix(header.Name, "./") == name {
			contents, _ := io.ReadAll(reader)
			return string(contents)
		}
	}
}

func debControlFile(t *testing.T, contents []byte) string {
	t.Helper()
	decompressed, errorValue := gzip.NewReader(bytes.NewReader(arMember(t, contents, "control.tar")))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return tarMember(t, decompressed, "control")
}

func archPackageInformation(t *testing.T, contents []byte) string {
	t.Helper()
	decompressed, errorValue := zstd.NewReader(bytes.NewReader(contents))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer decompressed.Close()
	return tarMember(t, decompressed, ".PKGINFO")
}

func rpmEpoch(t *testing.T, contents []byte) (uint32, bool) {
	t.Helper()
	offset := 96
	for range 2 {
		indexCount := int(binary.BigEndian.Uint32(contents[offset+8:]))
		storeSize := int(binary.BigEndian.Uint32(contents[offset+12:]))
		entries := contents[offset+16 : offset+16+indexCount*16]
		storeStart := offset + 16 + indexCount*16
		if offset != 96 {
			for index := 0; index < indexCount; index++ {
				entry := entries[index*16:]
				const epochTag = 1003
				if binary.BigEndian.Uint32(entry) == epochTag {
					return binary.BigEndian.Uint32(contents[storeStart+int(binary.BigEndian.Uint32(entry[8:])):]), true
				}
			}
			return 0, false
		}
		offset = storeStart + storeSize
		offset += (8 - offset%8) % 8
	}
	return 0, false
}

func TestEveryFormatCarriesTheMilestoneEpochInItsOwnMetadata(t *testing.T) {
	const version = "0.0.1+37"
	if control := debControlFile(t, writtenPackage(t, debianPackageFormat, version)); !strings.Contains(control, "\nVersion: 1:0.0.1+37\n") {
		t.Errorf("the deb control file says %q", control)
	}
	if epoch, isRecorded := rpmEpoch(t, writtenPackage(t, rpmPackageFormat, version)); !isRecorded || epoch != 1 {
		t.Errorf("the rpm header records epoch %d (present: %v)", epoch, isRecorded)
	}
	if information := archPackageInformation(t, writtenPackage(t, archlinuxPackageFormat, version)); !strings.Contains(information, "\npkgver = 1:0.0.1+37-1\n") {
		t.Errorf("the pacman package says %q", information)
	}
}

func TestEveryFormatNamesTheEpochOnTheInformationItIsGiven(t *testing.T) {
	for _, format := range linuxPackageFormats() {
		if epoch := linuxPackageInformation(format, packageTargets[0], "0.0.1", files.Contents{}, nfpm.Scripts{}).Epoch; epoch != "1" {
			t.Errorf("the %s package is given epoch %q", format.Name, epoch)
		}
	}
}

func TestAPackageIsBuiltOnlyAsAMilestoneOrABuildOfOne(t *testing.T) {
	for _, accepted := range []string{"0.0.1", "v0.0.1", "0.0.1+37", "1:0.0.1"} {
		if _, errorValue := chosenPackageVersion(accepted, t.TempDir()); errorValue != nil {
			t.Errorf("%q was refused: %v", accepted, errorValue)
		}
	}
	for _, refused := range []string{"v2026.10.07.120000", "latest", "0.0"} {
		if _, errorValue := chosenPackageVersion(refused, t.TempDir()); errorValue == nil {
			t.Errorf("%q was built", refused)
		}
	}
}
