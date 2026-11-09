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
	baseBackupEvery        = 24 * time.Hour
	upkeepCheckEvery       = time.Hour
	baseBackupsKept        = 2
	unreferencedChunkGrace = time.Hour
	removeBatchSize        = 1000
)

func KeepBaseBackups(ctx context.Context, replicationConnection string, destination offsite.Destination, keyring box.Keyring) {
	for {
		if errorValue := keepABaseBackupWhenDue(ctx, replicationConnection, destination, keyring, time.Now()); errorValue != nil && ctx.Err() == nil {
			log.Printf("keeping a base backup of the company database: %v; trying again in %s", errorValue, upkeepCheckEvery)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(upkeepCheckEvery):
		}
	}
}

func keepABaseBackupWhenDue(ctx context.Context, replicationConnection string, destination offsite.Destination, keyring box.Keyring, now time.Time) error {
	system, errorValue := identify(ctx, replicationConnection)
	if errorValue != nil {
		return errorValue
	}
	newest, errorValue := NewestBaseBackup(ctx, destination)
	isDue := errors.Is(errorValue, ErrNoBaseBackup) || newest.SystemID != system.SystemID || newest.Timeline != system.Timeline || now.Sub(newest.CreatedAt) >= baseBackupEvery
	if errorValue != nil && !errors.Is(errorValue, ErrNoBaseBackup) {
		return errorValue
	}
	if !isDue {
		return nil
	}
	backup, errorValue := TakeBaseBackup(ctx, replicationConnection, destination, keyring, now)
	if errorValue != nil {
		return errorValue
	}
	log.Printf("kept the base backup %s of the company database, from %s on timeline %d", backup.Name, backup.Start, backup.Timeline)
	return PruneBehindBaseBackups(ctx, destination, now)
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
	kept := []BaseBackup{}
	for _, name := range names[len(names)-baseBackupsKept:] {
		backup, isReadable := readBaseBackup(ctx, destination, name)
		if !isReadable {
			return fmt.Errorf("the kept base backup %s cannot be read, so nothing behind it is pruned until it is", name)
		}
		kept = append(kept, backup)
	}
	stale := []string{}
	for _, name := range names[:len(names)-baseBackupsKept] {
		stale = append(stale, baseBackupPath(destination, name))
	}
	if errorValue := removeInBatches(ctx, destination.Store, stale); errorValue != nil {
		return errorValue
	}
	if errorValue := prunePiecesBefore(ctx, destination, kept[0]); errorValue != nil {
		return errorValue
	}
	return sweepUnreferencedBaseChunks(ctx, destination, kept, now)
}

func prunePiecesBefore(ctx context.Context, destination offsite.Destination, oldestKept BaseBackup) error {
	start, errorValue := oldestKept.StartLSN()
	if errorValue != nil {
		return fmt.Errorf("the base backup %s names no start: %w", oldestKept.Name, errorValue)
	}
	kept := KeptPieces{Destination: destination, SystemID: oldestKept.SystemID}
	names, errorValue := kept.Names(ctx)
	if errorValue != nil {
		return errorValue
	}
	stale := []string{}
	for _, name := range names {
		if name.Timeline <= oldestKept.Timeline && name.End <= start {
			stale = append(stale, kept.pathOf(name.String()))
		}
	}
	return removeInBatches(ctx, destination.Store, stale)
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
