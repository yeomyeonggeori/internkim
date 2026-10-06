package cli

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blakesmith/ar"
	"github.com/goreleaser/nfpm/v2"
	"github.com/goreleaser/nfpm/v2/files"
)

func TestTheDebCarriesItsPayloadAsXzThatHoldsEveryFile(t *testing.T) {
	if _, errorValue := exec.LookPath("xz"); errorValue != nil {
		t.Skip("xz is not installed here")
	}
	directory := t.TempDir()
	programPath := filepath.Join(directory, "program")
	if errorValue := os.WriteFile(programPath, []byte("#!/bin/sh\necho program\n"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	packagePath := filepath.Join(directory, "sample.deb")
	information := &nfpm.Info{
		Name: "sample", Version: "1.0.0", Arch: "arm64", Maintainer: "sample <sample@example.com>",
		Overridables: nfpm.Overridables{
			Contents: files.Contents{{Source: programPath, Destination: "/usr/bin/program"}},
			Deb:      nfpm.Deb{Compression: debianPackageFormat.Compression},
		},
	}
	if errorValue := writeLinuxPackage(debianPackageFormat, information, packagePath); errorValue != nil {
		t.Fatal(errorValue)
	}
	members := readArchiveMembers(t, packagePath)
	names := []string{}
	for _, member := range members {
		names = append(names, member.name)
	}
	if len(names) != 3 || names[0] != "debian-binary" || names[2] != compressedDebianPayloadName {
		t.Fatalf("the deb's members are %v; dpkg reads debian-binary, then control, then %s", names, compressedDebianPayloadName)
	}
	if !tarHoldsFile(t, decompressXz(t, members[2].body), "./usr/bin/program") {
		t.Fatal("the compressed payload does not hold /usr/bin/program")
	}
}

type archiveMember struct {
	name string
	body []byte
}

func readArchiveMembers(t *testing.T, path string) []archiveMember {
	t.Helper()
	file, errorValue := os.Open(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer file.Close()
	reader := ar.NewReader(file)
	members := []archiveMember{}
	for {
		header, errorValue := reader.Next()
		if errors.Is(errorValue, io.EOF) {
			return members
		}
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		body, errorValue := io.ReadAll(reader)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		members = append(members, archiveMember{name: header.Name, body: body})
	}
}

func decompressXz(t *testing.T, compressed []byte) []byte {
	t.Helper()
	command := exec.Command("xz", "--decompress", "--stdout")
	command.Stdin = bytes.NewReader(compressed)
	output, errorValue := command.Output()
	if errorValue != nil {
		t.Fatalf("the payload is not xz: %v", errorValue)
	}
	return output
}

func tarHoldsFile(t *testing.T, archive []byte, name string) bool {
	t.Helper()
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, errorValue := reader.Next()
		if errors.Is(errorValue, io.EOF) {
			return false
		}
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if header.Name == name {
			return true
		}
	}
}
