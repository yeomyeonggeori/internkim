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
	"github.com/yeomyeonggeori/internkim/internal/walstream"
)

const takeOverUnitName = "internkim-take-over"

func companyDestinationOf(session box.Session) offsite.Destination {
	return offsite.Destination{
		Store: offsite.Store{
			ProjectURL:     session.Configuration.CentralPlane.ProjectURL,
			PublishableKey: session.Configuration.CentralPlane.PublishableKey,
			AccessToken:    func() (string, error) { return session.Session.AccessToken, nil },
			Held:           thisBoxShare(),
		},
		CompanyID:  session.Configuration.Company.ID,
		SealingKey: session.SealingKey,
	}
}

type takeOverSource struct {
	Description string
	ArchivePath string
}

func takeOverFromTheNewestBackup(ctx context.Context, session box.Session, keyring box.Keyring) error {
	switch takeOverUnitState() {
	case "active", "activating":
		return box.ErrTakeOverUnderway
	case "failed":
		if errorValue := takeOverRetries().clearTheFailedUnitWhenItIsTime(); errorValue != nil {
			return errorValue
		}
	default:
		if errorValue := takeOverRetries().forget(); errorValue != nil {
			return errorValue
		}
	}
	source, errorValue := newestSourceToTakeOverFrom(ctx, companyDestinationOf(session), keyring)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := keepTheTakeOverSession(takeOverSessionPath(), session.Session.AccessToken); errorValue != nil {
		return fmt.Errorf("keeping this box's session for the take-over: %w", errorValue)
	}
	return startTheTakeOver(source)
}

func newestSourceToTakeOverFrom(ctx context.Context, destination offsite.Destination, keyring box.Keyring) (takeOverSource, error) {
	baseBackup, errorValue := walstream.NewestBaseBackup(ctx, destination)
	if errorValue == nil {
		return takeOverSource{Description: "the company's log, from the base backup made " + baseBackup.CreatedAt.UTC().Format(time.RFC3339)}, nil
	}
	if !errors.Is(errorValue, walstream.ErrNoBaseBackup) {
		return takeOverSource{}, errorValue
	}
	return newestSnapshotToTakeOverFrom(ctx, destination, keyring)
}

func newestSnapshotToTakeOverFrom(ctx context.Context, destination offsite.Destination, keyring box.Keyring) (takeOverSource, error) {
	manifest, errorValue := offsite.Newest(ctx, destination)
	if errors.Is(errorValue, offsite.ErrNoBackup) {
		return takeOverSource{}, box.ErrNothingToTakeOver
	}
	if errorValue != nil {
		return takeOverSource{}, errorValue
	}
	archivePath := filepath.Join(companyhost.DefaultBackupDirectoryPath(), strings.TrimSuffix(manifest.Name, ".json")+".tar")
	if errorValue := os.MkdirAll(filepath.Dir(archivePath), 0o700); errorValue != nil {
		return takeOverSource{}, errorValue
	}
	cache := offsite.ChunkCache{DirectoryPath: filepath.Join(blueclaw.CompanyHostBackupsPath, "snapshot-chunks")}
	defer os.RemoveAll(cache.DirectoryPath)
	if errorValue := cache.Materialize(ctx, destination, manifest, keyring, archivePath); errorValue != nil {
		return takeOverSource{}, errorValue
	}
	return takeOverSource{Description: "the snapshot made " + manifest.CreatedAt.UTC().Format(time.RFC3339), ArchivePath: archivePath}, nil
}

func takeOverIsUnderway() bool {
	switch takeOverUnitState() {
	case "active", "activating":
		return true
	}
	return false
}

func takeOverUnitState() string {
	output, _ := exec.Command("systemctl", "show", "--property=ActiveState", "--value", takeOverUnitName+".service").Output()
	return strings.TrimSpace(string(output))
}

func startTheTakeOver(source takeOverSource) error {
	hostCommandPath, errorValue := os.Executable()
	if errorValue != nil {
		return errorValue
	}
	output, errorValue := exec.Command("systemd-run", takeOverUnitArguments(hostCommandPath, source)...).CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("systemd-run refused the take-over: %v: %s", errorValue, strings.TrimSpace(string(output)))
	}
	return box.ErrTakeOverUnderway
}

func takeOverUnitArguments(hostCommandPath string, source takeOverSource) []string {
	arguments := []string{
		"--unit=" + takeOverUnitName,
		"--description=internkim take-over from " + source.Description,
		"--property=OnFailure=" + blueclaw.BoxServiceName + ".service",
		hostCommandPath, "restore", "--taking-over",
	}
	if source.ArchivePath == "" {
		return arguments
	}
	return append(arguments, source.ArchivePath)
}

func standDown() error {
	if errorValue := stopTheTakeOver(); errorValue != nil {
		return errorValue
	}
	return companyhost.StandDown(thisComputer{}, os.Stdout)
}

func stopTheServices() error {
	if errorValue := stopTheTakeOver(); errorValue != nil {
		return errorValue
	}
	return companyhost.StopTheServices(thisComputer{}, os.Stdout)
}

func stopTheTakeOver() error {
	if !takeOverIsUnderway() {
		return nil
	}
	output, errorValue := exec.Command("systemctl", "stop", takeOverUnitName+".service").CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("systemctl stop %s: %v: %s", takeOverUnitName, errorValue, strings.TrimSpace(string(output)))
	}
	return nil
}

func standUp() error {
	return companyhost.StandUp(thisComputer{}, os.Stdout)
}

func markThisBoxInstalled(companyID string) error {
	return box.MarkInstalledCompany(blueclaw.CompanyHostBoxStatePath, companyID)
}
