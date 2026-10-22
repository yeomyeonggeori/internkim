package box

import (
	"os"
	"path/filepath"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/durablefile"
)

const boxMarkerFileName = "is-a-box"

func (daemon Daemon) connectedByFile() bool {
	if daemon.installedCompany() != "" || isABox(daemon.Places.StateDirectoryPath) {
		return false
	}
	_, errorValue := companyhost.ReadConnection(daemon.Places.ConnectionFilePath)
	return errorValue == nil
}

func rememberThisComputerIsABox(stateDirectoryPath string) error {
	if isABox(stateDirectoryPath) {
		return nil
	}
	return durablefile.Write(filepath.Join(stateDirectoryPath, boxMarkerFileName), nil, 0o600)
}

func isABox(stateDirectoryPath string) bool {
	_, errorValue := os.Stat(filepath.Join(stateDirectoryPath, boxMarkerFileName))
	return errorValue == nil
}
