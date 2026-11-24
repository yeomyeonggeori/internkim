package walstream

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

const (
	baseBackupEvery                    = 24 * time.Hour
	upkeepCheckEvery                   = time.Hour
	baseBackupsKept                    = 2
	baseBackupKeptForATakeOverUnderway = time.Hour
	unreferencedChunkGrace             = time.Hour
	removeBatchSize                    = 1000
)

func KeepBaseBackups(ctx context.Context, replicationConnection string, destination offsite.Destination, keyring box.Keyring, release string, pieces *PiecesSinceBaseBackup) {
	lastFailedAt := time.Time{}
	for {
		if !aFailedAttemptIsRecent(lastFailedAt, time.Now()) {
			lastFailedAt = time.Time{}
			if errorValue := keepABaseBackupWhenDue(ctx, replicationConnection, destination, keyring, release, pieces, time.Now()); errorValue != nil && ctx.Err() == nil {
				log.Printf("keeping a base backup of the company database: %v; trying again in %s", errorValue, upkeepCheckEvery)
				lastFailedAt = time.Now()
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(upkeepCheckEvery):
		case <-pieces.reached:
		}
	}
}

func keepABaseBackupWhenDue(ctx context.Context, replicationConnection string, destination offsite.Destination, keyring box.Keyring, release string, pieces *PiecesSinceBaseBackup, now time.Time) error {
	system, errorValue := identify(ctx, replicationConnection)
	if errorValue != nil {
		return errorValue
	}
	newest, errorValue := NewestBaseBackup(ctx, destination)
	if errorValue != nil && !errors.Is(errorValue, ErrNoBaseBackup) {
		return errorValue
	}
	facts := baseBackupFacts{
		hasNewest:       errorValue == nil,
		newestTakenAt:   newest.CreatedAt,
		isOtherDatabase: newest.SystemID != system.SystemID || newest.Timeline != system.Timeline,
		namesItsMajor:   newest.PostgreSQLMajor != 0,
	}
	if facts.hasNewest && !facts.isOtherDatabase {
		piecesSince, errorValue := piecesSinceTheNewest(ctx, destination, newest, pieces)
		if errorValue != nil {
			return errorValue
		}
		facts.piecesSince = piecesSince
	}
	if !baseBackupIsDue(facts, now) {
		return nil
	}
	backup, errorValue := TakeBaseBackup(ctx, replicationConnection, destination, keyring, release, now)
	if errorValue != nil {
		return errorValue
	}
	pieces.start(0)
	log.Printf("kept the base backup %s of the company database, from %s on timeline %d, after %d pieces of WAL", backup.Name, backup.Start, backup.Timeline, facts.piecesSince)
	return PruneBehindBaseBackups(ctx, destination, now)
}

func piecesSinceTheNewest(ctx context.Context, destination offsite.Destination, newest BaseBackup, pieces *PiecesSinceBaseBackup) (int, error) {
	if count, isInitialized := pieces.counted(); isInitialized {
		return count, nil
	}
	start, errorValue := newest.StartLSN()
	if errorValue != nil {
		return 0, fmt.Errorf("the base backup %s names no start: %w", newest.Name, errorValue)
	}
	names, errorValue := KeptPieces{Destination: destination, SystemID: newest.SystemID}.Names(ctx)
	if errorValue != nil {
		return 0, errorValue
	}
	count := piecesFrom(names, start)
	pieces.start(count)
	return count, nil
}

func identify(ctx context.Context, replicationConnection string) (pglogrepl.IdentifySystemResult, error) {
	connection, errorValue := pgconn.Connect(ctx, replicationConnection)
	if errorValue != nil {
		return pglogrepl.IdentifySystemResult{}, fmt.Errorf("connecting to the company database for replication: %w", errorValue)
	}
	defer connection.Close(context.Background())
	system, errorValue := pglogrepl.IdentifySystem(ctx, connection)
	if errorValue != nil {
		return pglogrepl.IdentifySystemResult{}, fmt.Errorf("identifying the company database: %w", errorValue)
	}
	return system, nil
}

func PruneBehindBaseBackups(ctx context.Context, destination offsite.Destination, now time.Time) error {
	names, errorValue := baseBackupNames(ctx, destination)
	if errorValue != nil {
		return errorValue
	}
	if len(names) <= baseBackupsKept {
		return nil
	}
	backups := []BaseBackup{}
	for _, name := range names {
		backup, isReadable := readBaseBackup(ctx, destination, name)
		if !isReadable {
			return fmt.Errorf("the base backup %s cannot be read, so nothing is pruned until it is", name)
		}
		backups = append(backups, backup)
	}
	kept, stale := baseBackupsToKeep(backups, now)
	stalePaths := make([]string, 0, len(stale))
	for _, backup := range stale {
		stalePaths = append(stalePaths, baseBackupPath(destination, backup.Name))
	}
	if errorValue := removeInBatches(ctx, destination.Store, stalePaths); errorValue != nil {
		return errorValue
	}
	if errorValue := prunePiecesBefore(ctx, destination, kept[0]); errorValue != nil {
		return errorValue
	}
	return sweepUnreferencedBaseChunks(ctx, destination, kept, now)
}

func baseBackupsToKeep(backups []BaseBackup, now time.Time) ([]BaseBackup, []BaseBackup) {
	kept, stale := []BaseBackup{}, []BaseBackup{}
	for index, backup := range backups {
		isAmongTheNewest := index >= len(backups)-baseBackupsKept
		mayBeInUse := now.Sub(backup.CreatedAt) < baseBackupKeptForATakeOverUnderway
		if isAmongTheNewest || mayBeInUse {
			kept = append(kept, backup)
			continue
		}
		stale = append(stale, backup)
	}
	return kept, stale
}

func prunePiecesBefore(ctx context.Context, destination offsite.Destination, oldestKept BaseBackup) error {
	kept := KeptPieces{Destination: destination, SystemID: oldestKept.SystemID}
	names, errorValue := kept.Names(ctx)
	if errorValue != nil {
		return errorValue
	}
	stale, errorValue := stalePieces(names, oldestKept)
	if errorValue != nil {
		return errorValue
	}
	paths := make([]string, 0, len(stale))
	for _, name := range stale {
		paths = append(paths, kept.pathOf(name.String()))
	}
	return removeInBatches(ctx, destination.Store, paths)
}

func stalePieces(names []PieceName, oldestKept BaseBackup) ([]PieceName, error) {
	first, errorValue := oldestKept.FirstWALNeeded()
	if errorValue != nil {
		return nil, errorValue
	}
	stale := []PieceName{}
	for _, name := range names {
		if name.Timeline <= oldestKept.Timeline && name.End <= first {
			stale = append(stale, name)
		}
	}
	return stale, nil
}

func sweepUnreferencedBaseChunks(ctx context.Context, destination offsite.Destination, kept []BaseBackup, now time.Time) error {
	referenced := map[string]bool{}
	for _, backup := range kept {
		for _, identifiers := range backup.Chunks {
			for _, identifier := range identifiers {
				referenced[identifier] = true
			}
		}
	}
	folder := destination.CompanyID + "/" + baseBackupChunkFolder + "/"
	listed, errorValue := destination.Store.List(ctx, folder)
	if errorValue != nil {
		return errorValue
	}
	stale := []string{}
	for _, object := range listed {
		if !referenced[object.Name] && object.CreatedAt.Before(now.Add(-unreferencedChunkGrace)) {
			stale = append(stale, folder+object.Name)
		}
	}
	return removeInBatches(ctx, destination.Store, stale)
}

func removeInBatches(ctx context.Context, store offsite.Store, paths []string) error {
	for start := 0; start < len(paths); start += removeBatchSize {
		if errorValue := store.Remove(ctx, paths[start:min(start+removeBatchSize, len(paths))]); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
