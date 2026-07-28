package admind

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const buzzIdentityVersionDirectoryName = "buzz-identity-version"

// versionedSubject is the derivation subject for a person's Buzz key at a given
// version. Version 1 is the bare email, so imported and first-generation
// identities are unchanged; a reset bumps the version, which changes the subject
// and therefore mints a fresh key (A') while the email stays the anchor.
func versionedSubject(email string, version int) string {
	normalized := normalizedVaultSubject(email)
	if version <= 1 {
		return normalized
	}
	return normalized + "|v" + strconv.Itoa(version)
}

func (service *Service) buzzIdentityVersionPath(personSubject string) string {
	return filepath.Join(service.Configuration.StateDirectory, buzzIdentityVersionDirectoryName, personSubject+".txt")
}

func (service *Service) buzzIdentityVersion(personSubject string) int {
	contents, errorValue := os.ReadFile(service.buzzIdentityVersionPath(personSubject))
	if errorValue != nil {
		return 1
	}
	version, errorValue := strconv.Atoi(strings.TrimSpace(string(contents)))
	if errorValue != nil || version < 1 {
		return 1
	}
	return version
}

func (service *Service) bumpBuzzIdentityVersion(personSubject string) (int, error) {
	next := service.buzzIdentityVersion(personSubject) + 1
	path := service.buzzIdentityVersionPath(personSubject)
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return 0, errorValue
	}
	if errorValue := writeFileAtomically(path, []byte(strconv.Itoa(next)), 0o600); errorValue != nil {
		return 0, errorValue
	}
	return next, nil
}
