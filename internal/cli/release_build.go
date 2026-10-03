package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"
)

func releaseBinaryRevision(repositoryRootPath string) string {
	revision := strings.TrimSpace(runCmd("git", "-C", repositoryRootPath, "rev-parse", "--short", "HEAD"))
	if revision == "" {
		return "unknown"
	}
	return revision
}

// The admin gateway's health answer is the only one that carries a revision, so it is
// the only way to see whether an upgrade moved the running process rather than the
// file. A build that leaves these at their defaults answers `unknown` and that check
// can never be made.
func admindStampFlags(buildID string, revision string) string {
	return strings.Join([]string{
		"-X", "github.com/yeomyeonggeori/internkim/internal/admind.BuildID=" + buildID,
		"-X", "github.com/yeomyeonggeori/internkim/internal/admind.GitRevision=" + revision,
		"-X", "github.com/yeomyeonggeori/internkim/internal/companyhost.PackageVersion=" + buildID,
	}, " ")
}

func gitRevision(repositoryRootPath string) string {
	return firstNonEmptyString(
		strings.TrimSpace(runCmd("git", "-C", repositoryRootPath, "rev-parse", "HEAD")),
		"unknown",
	)
}

func shortRevision(revision string) string {
	revision = strings.TrimSpace(revision)
	if len(revision) <= 12 {
		return revision
	}
	return revision[:12]
}

func releaseFileSHA256AndSize(path string) (string, int64, error) {
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return "", 0, errorValue
	}
	defer file.Close()
	hash := sha256.New()
	size, errorValue := io.Copy(hash, file)
	if errorValue != nil {
		return "", 0, errorValue
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}
