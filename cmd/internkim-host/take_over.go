package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const takeOverUnitName = "internkim-take-over"

func takeOverFromTheNewestBackup(ctx context.Context, session box.Session, keyring box.Keyring) error {
	switch takeOverUnitState() {
	case "active", "activating":
		return box.ErrTakeOverUnderway
	case "failed":
		return fmt.Errorf("the last take-over failed; read `journalctl -u %s`, then run `systemctl reset-failed %s` to let this box try again", takeOverUnitName, takeOverUnitName)
	}
	destination := offsite.Destination{
		Store: offsite.Store{
			ProjectURL:     session.Configuration.CentralPlane.ProjectURL,
			PublishableKey: session.Configuration.CentralPlane.PublishableKey,
			AccessToken:    func() (string, error) { return session.Session.AccessToken, nil },
		},
		CompanyID:  session.Configuration.Company.ID,
		SealingKey: session.SealingKey,
	}
	manifest, errorValue := offsite.Newest(ctx, destination)
	if errors.Is(errorValue, offsite.ErrNoBackup) {
		return box.ErrNothingToTakeOver
	}
	if errorValue != nil {
		return errorValue
	}
	archivePath := filepath.Join(companyhost.DefaultBackupDirectoryPath(), manifest.ArchiveName)
	if errorValue := os.MkdirAll(filepath.Dir(archivePath), 0o700); errorValue != nil {
		return errorValue
	}
	if errorValue := offsite.Fetch(ctx, destination, manifest, keyring, archivePath); errorValue != nil {
		return errorValue
	}
	return startTheTakeOver(archivePath, manifest.CreatedAt)
}

func takeOverUnitState() string {
	output, _ := exec.Command("systemctl", "show", "--property=ActiveState", "--value", takeOverUnitName+".service").Output()
	return strings.TrimSpace(string(output))
}

func startTheTakeOver(archivePath string, createdAt time.Time) error {
	hostCommandPath, errorValue := os.Executable()
	if errorValue != nil {
		return errorValue
	}
	output, errorValue := exec.Command("systemd-run",
		"--unit="+takeOverUnitName,
		"--description=internkim take-over from the backup made "+createdAt.UTC().Format(time.RFC3339),
		hostCommandPath, "restore", "--taking-over", archivePath,
	).CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("systemd-run refused the take-over: %v: %s", errorValue, strings.TrimSpace(string(output)))
	}
	return box.ErrTakeOverUnderway
}

func standDown() error {
	return companyhost.StandDown(thisComputer{}, os.Stdout)
}

func markThisBoxInstalled(companyID string) error {
	return box.MarkInstalledCompany(blueclaw.CompanyHostBoxStatePath, companyID)
}
