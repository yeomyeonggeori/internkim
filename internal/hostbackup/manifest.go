package hostbackup

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

const (
	FormatName    = "internkim-host-backup"
	FormatVersion = 1

	ManifestName     = "manifest.json"
	FilesMemberName  = "files.tar"
	RosterMemberName = "roster.json"

	databaseMemberDirectory = "databases"
	databaseMemberSuffix    = ".dump"
	largestManifestBytes    = 1 << 20
)

type Manifest struct {
	Format          string    `json:"format"`
	FormatVersion   int       `json:"formatVersion"`
	CreatedAt       time.Time `json:"createdAt"`
	PackageVersion  string    `json:"packageVersion"`
	CompanyID       string    `json:"companyID"`
	PostgreSQLMajor int       `json:"postgresqlMajor"`
	FileRoots       []string  `json:"fileRoots"`
	Members         []Member  `json:"members"`
}

type Member struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func DatabaseMemberName(database string) string {
	return path.Join(databaseMemberDirectory, database+databaseMemberSuffix)
}

func DatabaseNamedBy(memberName string) (string, bool) {
	directory, fileName := path.Split(memberName)
	if directory != databaseMemberDirectory+"/" || !strings.HasSuffix(fileName, databaseMemberSuffix) {
		return "", false
	}
	database := strings.TrimSuffix(fileName, databaseMemberSuffix)
	return database, database != ""
}

func (manifest Manifest) Member(name string) (Member, bool) {
	for _, member := range manifest.Members {
		if member.Name == name {
			return member, true
		}
	}
	return Member{}, false
}

func (manifest Manifest) TotalSize() int64 {
	total := int64(0)
	for _, member := range manifest.Members {
		total += member.Size
	}
	return total
}

func readManifest(reader io.Reader) (Manifest, error) {
	document, errorValue := io.ReadAll(io.LimitReader(reader, largestManifestBytes+1))
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	if len(document) > largestManifestBytes {
		return Manifest{}, fmt.Errorf("its %s is larger than %d bytes, which no backup writes", ManifestName, largestManifestBytes)
	}
	var manifest Manifest
	if errorValue := json.Unmarshal(document, &manifest); errorValue != nil {
		return Manifest{}, fmt.Errorf("its %s is not JSON: %w", ManifestName, errorValue)
	}
	return manifest, validateManifest(manifest)
}

func validateManifest(manifest Manifest) error {
	if manifest.Format != FormatName {
		return fmt.Errorf("its %s names the format %q, not %q", ManifestName, manifest.Format, FormatName)
	}
	if manifest.FormatVersion > FormatVersion {
		return fmt.Errorf(
			"it was written in format %d and this internkim reads format %d and older. Upgrade internkim on this computer, then restore it",
			manifest.FormatVersion, FormatVersion)
	}
	if manifest.FormatVersion < 1 {
		return fmt.Errorf("its %s names format %d, which no internkim wrote", ManifestName, manifest.FormatVersion)
	}
	seen := map[string]bool{}
	for _, member := range manifest.Members {
		if errorValue := validateMemberName(member.Name); errorValue != nil {
			return errorValue
		}
		if seen[member.Name] {
			return fmt.Errorf("its %s lists %s twice", ManifestName, member.Name)
		}
		seen[member.Name] = true
	}
	return nil
}

func validateMemberName(name string) error {
	if name == ManifestName {
		return fmt.Errorf("its %s lists itself as a member", ManifestName)
	}
	if name == "" || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") {
		return fmt.Errorf("it names a member %q, which is not a plain relative path", name)
	}
	return nil
}
