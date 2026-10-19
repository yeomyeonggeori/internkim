package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const takeOverUnitName = "internkim-take-over"

func companyDestinationOf(session box.Session) offsite.Destination {
	return offsite.Destination{
		Store: offsite.Store{
			ProjectURL:     session.Configuration.CentralPlane.ProjectURL,
			PublishableKey: session.Configuration.CentralPlane.PublishableKey,
			AccessToken:    func() (string, error) { return session.Session.AccessToken, nil },
		},
		CompanyID:  session.Configuration.Company.ID,
		SealingKey: session.SealingKey,
	}
}

func standbyChunks() offsite.ChunkCache {
	return offsite.ChunkCache{DirectoryPath: blueclaw.CompanyHostStandbyChunksPath}
}

func takeOverFromTheNewestBackup(ctx context.Context, session box.Session, keyring box.Keyring) error {
	switch takeOverUnitState() {
	case "active", "activating":
		return box.ErrTakeOverUnderway
	case "failed":
		return fmt.Errorf("the last take-over failed; read `journalctl -u %s`, then run `systemctl reset-failed %s` to let this box try again", takeOverUnitName, takeOverUnitName)
	}
	destination := companyDestinationOf(session)
	manifest, errorValue := offsite.Newest(ctx, destination)
	if errors.Is(errorValue, offsite.ErrNoBackup) {
		return box.ErrNothingToTakeOver
	}
	if errorValue != nil {
		return errorValue
	}
	archivePath := filepath.Join(companyhost.DefaultBackupDirectoryPath(), strings.TrimSuffix(manifest.Name, ".json")+".tar")
	if errorValue := os.MkdirAll(filepath.Dir(archivePath), 0o700); errorValue != nil {
		return errorValue
	}
	if errorValue := standbyChunks().Materialize(ctx, destination, manifest, keyring, archivePath); errorValue != nil {
		return errorValue
	}
	return startTheTakeOver(archivePath, manifest.CreatedAt)
}

var keepingUp sync.Mutex

func startKeepingUpWithTheServiceBox(ctx context.Context, session box.Session) {
	if !keepingUp.TryLock() {
		return
	}
	go func() {
		defer keepingUp.Unlock()
		manifest, fetched, errorValue := standbyChunks().KeepUp(ctx, companyDestinationOf(session))
		switch {
		case errors.Is(errorValue, offsite.ErrNoBackup):
		case errorValue != nil:
			log.Printf("keeping up with the service box's newest snapshot: %v; trying again at the next check", errorValue)
		case fetched > 0:
			log.Printf("this box holds %s now, %d chunks newer than what it held", manifest.Name, fetched)
		}
	}()
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

func standUp() error {
	return companyhost.StandUp(thisComputer{}, os.Stdout)
}

func markThisBoxInstalled(companyID string) error {
	return box.MarkInstalledCompany(blueclaw.CompanyHostBoxStatePath, companyID)
}
