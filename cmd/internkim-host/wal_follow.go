package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"github.com/yeomyeonggeori/internkim/internal/walstream"
)

const releaseVerb = "release"

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
	destination, keyring, isConnected, errorValue := thisBox().Destination()
	if errorValue != nil {
		return errorValue
	}
	if !isConnected {
		return errNotAServingBox
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	hasSeveralBoxes, errorValue := untilTheBoxCountIsKnown(ctx)
	if errorValue != nil {
		return errorValue
	}
	if !hasSeveralBoxes {
		if errorValue := releaseAndForgetTheKeptWAL(ctx, destination); errorValue != nil {
			return errorValue
		}
		watched, cancel := whileSeveralBoxesIs(ctx, false)
		defer cancel()
		<-watched.Done()
		return nil
	}
	watched, cancel := whileSeveralBoxesIs(ctx, true)
	defer cancel()
	errorValue = walstream.Follower{
		ReplicationConnection: companyReplicationConnection(),
		KeptFor: func(systemID string) walstream.Keeper {
			return walstream.KeptPieces{Destination: destination, SystemID: systemID}
		},
		OnStreaming: func(ctx context.Context) error {
			if errorValue := walstream.RequireTheStandby(ctx, companyDatabaseConnection()); errorValue != nil {
				return errorValue
			}
			go walstream.KeepBaseBackups(ctx, companyReplicationConnection(), destination, keyring)
			return nil
		},
	}.Run(watched)
	if ctx.Err() == nil && watched.Err() != nil {
		return releaseAndForgetTheKeptWAL(ctx, destination)
	}
	return errorValue
}

func releaseAndForgetTheKeptWAL(ctx context.Context, destination offsite.Destination) error {
	if errorValue := walstream.ReleaseTheStandby(ctx, companyDatabaseConnection()); errorValue != nil {
		return errorValue
	}
	return walstream.ForgetKeptWAL(ctx, companyReplicationConnection(), destination)
}

func rebuildTheDatabaseFromKeptWAL(dataDirectoryPath string) error {
	destination, keyring, isConnected, errorValue := thisBox().Destination()
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
