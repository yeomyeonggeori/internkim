package hostbackup

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
)

type Archive struct {
	Path     string
	Manifest Manifest
}

func Open(path string) (Archive, error) {
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return Archive{}, errorValue
	}
	defer file.Close()
	manifest, errorValue := findManifest(tar.NewReader(file))
	if errorValue != nil {
		return Archive{}, fmt.Errorf("%s is not a backup this computer can restore: %w", path, errorValue)
	}
	return Archive{Path: path, Manifest: manifest}, nil
}

func findManifest(reader *tar.Reader) (Manifest, error) {
	var found *Manifest
	for {
		header, errorValue := reader.Next()
		if errors.Is(errorValue, io.EOF) {
			break
		}
		if errorValue != nil {
			return Manifest{}, fmt.Errorf("it is not a whole tar archive (%w)", errorValue)
		}
		if header.Name != ManifestName {
			continue
		}
		if found != nil {
			return Manifest{}, fmt.Errorf("it carries two %s members", ManifestName)
		}
		manifest, errorValue := readManifest(reader)
		if errorValue != nil {
			return Manifest{}, errorValue
		}
		found = &manifest
	}
	if found == nil {
		return Manifest{}, fmt.Errorf("it carries no %s", ManifestName)
	}
	return *found, nil
}

func (archive Archive) Verify() error {
	file, errorValue := os.Open(archive.Path)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	reader := tar.NewReader(file)
	seen := map[string]bool{}
	for {
		header, errorValue := reader.Next()
		if errors.Is(errorValue, io.EOF) {
			break
		}
		if errorValue != nil {
			return fmt.Errorf("%s ends part way through (%w)", archive.Path, errorValue)
		}
		if header.Name == ManifestName {
			continue
		}
		if errorValue := archive.verifyMember(header, reader, seen); errorValue != nil {
			return errorValue
		}
	}
	for _, member := range archive.Manifest.Members {
		if !seen[member.Name] {
			return fmt.Errorf("the manifest lists %s and the archive does not carry it", member.Name)
		}
	}
	return nil
}

func (archive Archive) verifyMember(header *tar.Header, reader io.Reader, seen map[string]bool) error {
	member, isListed := archive.Manifest.Member(header.Name)
	if !isListed {
		return fmt.Errorf("the archive carries %s, which its manifest does not list", header.Name)
	}
	if seen[header.Name] {
		return fmt.Errorf("the archive carries %s twice", header.Name)
	}
	seen[header.Name] = true
	if header.Size != member.Size {
		return fmt.Errorf("%s is %d bytes and the manifest says %d", member.Name, header.Size, member.Size)
	}
	hasher := sha256.New()
	if _, errorValue := io.Copy(hasher, reader); errorValue != nil {
		return fmt.Errorf("%s could not be read to its end (%w)", member.Name, errorValue)
	}
	if hex.EncodeToString(hasher.Sum(nil)) != member.SHA256 {
		return fmt.Errorf("%s does not match the checksum its manifest records, so it changed after the backup was written", member.Name)
	}
	return nil
}

func (archive Archive) ReadMember(name string, consume func(io.Reader) error) error {
	member, isListed := archive.Manifest.Member(name)
	if !isListed {
		return fmt.Errorf("the manifest lists no %s", name)
	}
	file, errorValue := os.Open(archive.Path)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	reader := tar.NewReader(file)
	for {
		header, errorValue := reader.Next()
		if errorValue != nil {
			return fmt.Errorf("finding %s in %s: %w", name, archive.Path, errorValue)
		}
		if header.Name == name {
			return consume(&checkedReader{reader: reader, hasher: sha256.New(), member: member})
		}
	}
}

type checkedReader struct {
	reader io.Reader
	hasher hash.Hash
	member Member
}

func (checked *checkedReader) Read(buffer []byte) (int, error) {
	count, errorValue := checked.reader.Read(buffer)
	checked.hasher.Write(buffer[:count])
	if !errors.Is(errorValue, io.EOF) {
		return count, errorValue
	}
	if hex.EncodeToString(checked.hasher.Sum(nil)) != checked.member.SHA256 {
		return count, fmt.Errorf("%s changed since it was verified", checked.member.Name)
	}
	return count, io.EOF
}
