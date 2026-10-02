package companyhost

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func Refresh(machine Machine, progress io.Writer) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	return refreshOn(platform, machine, progress)
}

func refreshOn(platform companyHostPlatform, machine Machine, progress io.Writer) error {
	directoryPath, errorValue := filepath.EvalSymlinks(blueclaw.CompanyHostCurrentPath)
	if errorValue != nil {
		return fmt.Errorf("this computer has no company to bring back: %w", errorValue)
	}
	connection, errorValue := ReadConnection(filepath.Join(directoryPath, connectionFileName))
	if errorValue != nil {
		return errorValue
	}
	secrets, errorValue := keepCompanySecrets(filepath.Join(directoryPath, secretDirectoryName))
	if errorValue != nil {
		return errorValue
	}

	fmt.Fprintf(progress, "1/4 Rewriting %s's settings from this release…\n", connection.Company.Name)
	if errorValue := rewriteCompanySettings(platform, machine, directoryPath, connection, secrets); errorValue != nil {
		return errorValue
	}

	fmt.Fprintln(progress, "2/4 Preparing PostgreSQL…")
	if errorValue := prepareDatabases(platform, machine, companyHostSettings{DatabasePassword: secrets.DatabasePassword}, progress); errorValue != nil {
		return errorValue
	}
	if errorValue := rehomeTheMessengerCommunity(platform, machine, connection, progress); errorValue != nil {
		return errorValue
	}

	fmt.Fprintln(progress, "3/4 Restarting the services…")
	if errorValue := platform.SuperviseTheBundle(machine, progress); errorValue != nil {
		return errorValue
	}

	fmt.Fprintln(progress, "4/4 Waiting for the server to answer…")
	return waitUntilTheServerAnswers(platform, machine, progress)
}

func rewriteCompanySettings(platform companyHostPlatform, machine Machine, directoryPath string, connection Connection, secrets companySecrets) error {
	files, errorValue := companySettingsFiles(platform.Layout(), directoryPath, connection, secrets)
	if errorValue != nil {
		return errorValue
	}
	for _, file := range files {
		if errorValue := writeCompanyFile(machine, file); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
