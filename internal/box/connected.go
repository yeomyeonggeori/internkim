package box

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const identityFileName = "identity.json"

var ErrNoBoxKey = errors.New("this computer holds no box key")

type Connected struct {
	Identity  Identity
	CompanyID string
}

func LoadConnected(stateDirectoryPath string) (Connected, error) {
	identity, errorValue := loadIdentity(identityPathIn(stateDirectoryPath))
	if errors.Is(errorValue, os.ErrNotExist) {
		return Connected{}, fmt.Errorf("%w at %s", ErrNoBoxKey, stateDirectoryPath)
	}
	if errorValue != nil {
		return Connected{}, errorValue
	}
	companyID := installedCompanyIn(stateDirectoryPath)
	if companyID == "" {
		return Connected{}, fmt.Errorf("the box at %s has installed no company yet", stateDirectoryPath)
	}
	return Connected{Identity: identity, CompanyID: companyID}, nil
}

func identityPathIn(stateDirectoryPath string) string {
	return filepath.Join(stateDirectoryPath, identityFileName)
}

func installedCompanyIn(stateDirectoryPath string) string {
	document, errorValue := os.ReadFile(filepath.Join(stateDirectoryPath, companyMarkerFileName))
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}
