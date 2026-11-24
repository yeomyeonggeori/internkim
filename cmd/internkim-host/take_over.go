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
	"github.com/yeomyeonggeori/internkim/internal/filestream"
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
		},
		CompanyID:  session.Configuration.Company.ID,
		SealingKey: session.SealingKey,
	}
}

func standbyChunks() offsite.ChunkCache {
	return offsite.ChunkCache{DirectoryPath: blueclaw.CompanyHostStandbyChunksPath}
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
	if errorValue := standbyChunks().Materialize(ctx, destination, manifest, keyring, archivePath); errorValue != nil {
		return takeOverSource{}, errorValue
	}
	return takeOverSource{Description: "the snapshot made " + manifest.CreatedAt.UTC().Format(time.RFC3339), ArchivePath: archivePath}, nil
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

func handOverTheCompany(ctx context.Context, session box.Session) (box.LogPosition, error) {
	if errorValue := stopTheTakeOver(); errorValue != nil {
		return box.LogPosition{}, errorValue
	}
	keepWhatWasWritten := func() error { return keepTheLastFileChanges(ctx) }
	if errorValue := companyhost.StopForTheMove(thisComputer{}, os.Stdout, keepWhatWasWritten); errorValue != nil {
		return box.LogPosition{}, errorValue
	}
	return whereTheCompanysLogsEnd(ctx, companyDestinationOf(session))
}

func keepTheLastFileChanges(ctx context.Context) error {
	if !isActive(blueclaw.CompanyHostFileFollowServiceName) {
		return nil
	}
	asking, cancel := context.WithTimeout(ctx, lastFileChangesWait)
	defer cancel()
	return filestream.AskToKeep(asking, blueclaw.CompanyHostFileJournalSocketPath)
}

const lastFileChangesWait = time.Minute

func isActive(unitName string) bool {
	return exec.Command("systemctl", "is-active", "--quiet", unitName+".service").Run() == nil
}

func whereTheCompanysLogsEnd(ctx context.Context, destination offsite.Destination) (box.LogPosition, error) {
	wal, errorValue := walstream.KeptEnd(ctx, destination)
	if errorValue != nil {
		return box.LogPosition{}, fmt.Errorf("reading where the company's WAL ends in the store: %w", errorValue)
	}
	stop := box.LogPosition{WALTimeline: wal.Timeline, WALPosition: wal.LSN.String()}
	entry, hasEntries, errorValue := filestream.Journal{Destination: destination}.Newest(ctx)
	if errorValue != nil {
		return box.LogPosition{}, fmt.Errorf("reading where the company's file journal ends in the store: %w", errorValue)
	}
	if hasEntries {
		stop.FileEntry = &entry
	}
	return stop, nil
}

func standUp() error {
	return companyhost.StandUp(thisComputer{}, os.Stdout)
}

func markThisBoxInstalled(companyID string) error {
	return box.MarkInstalledCompany(blueclaw.CompanyHostBoxStatePath, companyID)
}
