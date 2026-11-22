package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/durablefile"
	"github.com/yeomyeonggeori/internkim/internal/filestream"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"github.com/yeomyeonggeori/internkim/internal/walstream"
)

const takeOverArchiveFileName = "take-over-archive"

func takeOverArchivePath() string {
	return filepath.Join(blueclaw.CompanyHostBoxStatePath, takeOverArchiveFileName)
}

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
	Snapshot    *offsite.Manifest
	ArchivePath string
}

func takeOverFromTheNewestBackup(ctx context.Context, session box.Session, keyring box.Keyring) error {
	if errorValue := waitForTheLastTakeOver(); errorValue != nil {
		return errorValue
	}
	destination := companyDestinationOf(session)
	source, errorValue := newestSourceToTakeOverFrom(ctx, destination)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := companyhost.TakesOverACompany(); errorValue != nil {
		return fmt.Errorf("this box stands by and does not take the company over: %w", errorValue)
	}
	if source, errorValue = source.materialized(ctx, destination, keyring); errorValue != nil {
		return errorValue
	}
	if errorValue := keepTheTakeOverSession(takeOverSessionPath(), session.Session.AccessToken); errorValue != nil {
		return fmt.Errorf("keeping this box's session for the take-over: %w", errorValue)
	}
	return startTheTakeOver(source)
}

func waitForTheLastTakeOver() error {
	state, errorValue := takeOverState()
	if errorValue != nil {
		return errorValue
	}
	switch state {
	case companyhost.ServiceRunning:
		return box.ErrTakeOverUnderway
	case companyhost.ServiceFailed:
		return takeOverRetries().allowARetryWhenItIsTime()
	}
	return takeOverRetries().forget()
}

func newestSourceToTakeOverFrom(ctx context.Context, destination offsite.Destination) (takeOverSource, error) {
	baseBackup, errorValue := walstream.NewestBaseBackup(ctx, destination)
	if errorValue == nil {
		return takeOverSource{Description: "the company's log, from the base backup made " + baseBackup.CreatedAt.UTC().Format(time.RFC3339)}, nil
	}
	if !errors.Is(errorValue, walstream.ErrNoBaseBackup) {
		return takeOverSource{}, errorValue
	}
	manifest, errorValue := offsite.Newest(ctx, destination)
	if errors.Is(errorValue, offsite.ErrNoBackup) {
		return takeOverSource{}, box.ErrNothingToTakeOver
	}
	if errorValue != nil {
		return takeOverSource{}, errorValue
	}
	return takeOverSource{Description: "the snapshot made " + manifest.CreatedAt.UTC().Format(time.RFC3339), Snapshot: &manifest}, nil
}

func (source takeOverSource) materialized(ctx context.Context, destination offsite.Destination, keyring box.Keyring) (takeOverSource, error) {
	if source.Snapshot == nil {
		return source, nil
	}
	archivePath := filepath.Join(companyhost.DefaultBackupDirectoryPath(), strings.TrimSuffix(source.Snapshot.Name, ".json")+".tar")
	if errorValue := os.MkdirAll(filepath.Dir(archivePath), 0o700); errorValue != nil {
		return takeOverSource{}, errorValue
	}
	if errorValue := standbyChunks().Materialize(ctx, destination, *source.Snapshot, keyring, archivePath); errorValue != nil {
		return takeOverSource{}, errorValue
	}
	source.ArchivePath = archivePath
	return source, nil
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
	state, errorValue := takeOverState()
	if errorValue != nil {
		log.Printf("reading whether a take-over is underway: %v", errorValue)
		return false
	}
	return state == companyhost.ServiceRunning
}

func takeOverState() (companyhost.ServiceState, error) {
	return companyhost.StateOfTheService(thisComputer{}, blueclaw.CompanyHostTakeOverServiceName)
}

func startTheTakeOver(source takeOverSource) error {
	if errorValue := nameTheArchiveToTakeOverFrom(takeOverArchivePath(), source.ArchivePath); errorValue != nil {
		return fmt.Errorf("naming the snapshot the take-over restores: %w", errorValue)
	}
	log.Printf("taking the company over from %s", source.Description)
	if errorValue := companyhost.StartTheService(thisComputer{}, blueclaw.CompanyHostTakeOverServiceName, os.Stdout); errorValue != nil {
		return fmt.Errorf("the take-over could not be started: %w", errorValue)
	}
	return box.ErrTakeOverUnderway
}

func nameTheArchiveToTakeOverFrom(path string, archivePath string) error {
	if archivePath != "" {
		return durablefile.Write(path, []byte(archivePath+"\n"), 0o600)
	}
	errorValue := os.Remove(path)
	if errors.Is(errorValue, os.ErrNotExist) {
		return nil
	}
	return errorValue
}

func archiveToTakeOverFrom(path string) (string, error) {
	document, errorValue := os.ReadFile(path)
	if errors.Is(errorValue, os.ErrNotExist) {
		return "", nil
	}
	if errorValue != nil {
		return "", errorValue
	}
	return strings.TrimSpace(string(document)), nil
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
	return companyhost.StopTheService(thisComputer{}, blueclaw.CompanyHostTakeOverServiceName, os.Stdout)
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

func isActive(serviceName string) bool {
	state, errorValue := companyhost.StateOfTheService(thisComputer{}, serviceName)
	return errorValue == nil && state == companyhost.ServiceRunning
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
