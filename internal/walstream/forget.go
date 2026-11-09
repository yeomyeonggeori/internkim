package walstream

import (
	"context"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

func ForgetKeptWAL(ctx context.Context, replicationConnection string, destination offsite.Destination) error {
	system, errorValue := identify(ctx, replicationConnection)
	if errorValue != nil {
		return errorValue
	}
	kept := KeptPieces{Destination: destination, SystemID: system.SystemID}
	stale := []string{}
	for _, folder := range []string{kept.folder(), destination.CompanyID + "/" + historyFolder + "/" + system.SystemID + "/"} {
		listed, errorValue := destination.Store.List(ctx, folder)
		if errorValue != nil {
			return errorValue
		}
		for _, object := range listed {
			stale = append(stale, folder+object.Name)
		}
	}
	names, errorValue := baseBackupNames(ctx, destination)
	if errorValue != nil {
		return errorValue
	}
	remaining := []BaseBackup{}
	for _, name := range names {
		backup, isReadable := readBaseBackup(ctx, destination, name)
		switch {
		case isReadable && backup.SystemID == system.SystemID:
			stale = append(stale, baseBackupPath(destination, name))
		case isReadable:
			remaining = append(remaining, backup)
		}
	}
	if errorValue := removeInBatches(ctx, destination.Store, stale); errorValue != nil {
		return errorValue
	}
	return sweepUnreferencedBaseChunks(ctx, destination, remaining, time.Now())
}
