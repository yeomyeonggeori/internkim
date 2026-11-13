package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"github.com/yeomyeonggeori/internkim/internal/walstream"
)

const (
	releaseVerb             = "release"
	forgetKeptWALAgainAfter = time.Minute
)

var errNotAServingBox = errors.New("this computer is not a box holding its company's key, so it keeps no WAL for the company's other boxes")

func companyDatabaseConnection() string {
	return fmt.Sprintf("host=%s port=%s user=%s dbname=postgres", blueclaw.CompanyHostDatabaseSocketDirectory, blueclaw.CompanyHostDatabasePort, blueclaw.CompanyHostDatabaseUser)
}

func companyReplicationConnection() string {
	return companyDatabaseConnection() + " replication=true application_name=" + walstream.StandbyName
}

func runWalFollow(arguments []string) error {
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		return errorValue
	}
	if len(arguments) == 1 && arguments[0] == releaseVerb {
		return walstream.ReleaseTheStandby(context.Background(), companyDatabaseConnection())
	}
	if len(arguments) != 0 {
		return fmt.Errorf("wal-follow takes no arguments, or %q to let the database commit without the company's other boxes", releaseVerb)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	kept, errorValue := whatThisBoxKeeps()
	if errorValue != nil {
		return errorValue
	}
	watched, cancel := whileThisBoxKeepsTheSame(ctx, kept)
	defer cancel()
	if !kept.isJournaled {
		return keepNoWAL(watched, kept)
	}
	errorValue = walstream.Follower{
		ReplicationConnection: companyReplicationConnection(),
		KeptFor: func(systemID string) walstream.Keeper {
			return walstream.KeptPieces{Destination: kept.destination, SystemID: systemID}
		},
		OnStreaming: func(ctx context.Context) error {
			if errorValue := walstream.RequireTheStandby(ctx, companyDatabaseConnection()); errorValue != nil {
				return errorValue
			}
			go walstream.KeepBaseBackups(ctx, companyReplicationConnection(), kept.destination, kept.keyring)
			return nil
		},
	}.Run(watched)
	if ctx.Err() == nil && watched.Err() != nil {
		return walstream.ReleaseTheStandby(ctx, companyDatabaseConnection())
	}
	return errorValue
}

func keepNoWAL(ctx context.Context, kept keptOffTheBox) error {
	if errorValue := walstream.ReleaseTheStandby(ctx, companyDatabaseConnection()); errorValue != nil {
		return errorValue
	}
	if kept.holdsTheCompanyKey {
		untilTheKeptWALIsForgotten(ctx, kept.destination)
	}
	<-ctx.Done()
	return nil
}

func untilTheKeptWALIsForgotten(ctx context.Context, destination offsite.Destination) {
	for {
		errorValue := walstream.ForgetKeptWAL(ctx, companyReplicationConnection(), destination)
		if errorValue == nil || ctx.Err() != nil {
			return
		}
		log.Printf("the WAL kept for the company's other boxes is not forgotten yet: %v; trying again in %s", errorValue, forgetKeptWALAgainAfter)
		select {
		case <-ctx.Done():
			return
		case <-time.After(forgetKeptWALAgainAfter):
		}
	}
}

func rebuildTheDatabaseFromKeptWALAs(place offsite.ThisBox) func(companyID, dataDirectoryPath string) error {
	return func(companyID, dataDirectoryPath string) error {
		destination, keyring, isConnected, errorValue := place.DestinationOf(companyID)
		if errorValue != nil {
			return errorValue
		}
		if !isConnected {
			return errNotAServingBox
		}
		cache := offsite.ChunkCache{DirectoryPath: filepath.Join(blueclaw.CompanyHostBackupsPath, "base-chunks")}
		rebuilt, errorValue := walstream.RebuildDataDirectory(context.Background(), destination, keyring, cache, dataDirectoryPath)
		if errors.Is(errorValue, walstream.ErrNoBaseBackup) {
			return companyhost.ErrNoKeptWAL
		}
		if errorValue != nil {
			return errorValue
		}
		fmt.Printf("  the database was rebuilt from %s and %d pieces of kept WAL, to %s\n", rebuilt.BaseBackup.Name, rebuilt.Pieces, rebuilt.ReplaysTo)
		return nil
	}
}
