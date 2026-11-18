package companyhost

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

func keepASafetyBackup(platform backupPlatform, request RestoreRequest, backupDirectoryPath string, machine Machine, progress io.Writer) (string, error) {
	relied, errorValue := safetyBackupToRelyOn(request.KeptSafetyBackup)
	if errorValue != nil {
		return "", errorValue
	}
	if relied != "" {
		fmt.Fprintf(progress, "2/6 An earlier attempt of this take-over kept what this computer held in %s, so it is not backed up again…\n", relied)
		return relied, nil
	}
	fmt.Fprintln(progress, "2/6 Backing up what this computer holds now, so the restore can be undone…")
	safety, errorValue := backupWhileLocked(platform, BackupRequest{DirectoryPath: backupDirectoryPath, Now: request.Now}, machine, io.Discard)
	if errorValue != nil {
		return "", fmt.Errorf("the backup of this computer's current state failed, so nothing was restored: %w", errorValue)
	}
	fmt.Fprintf(progress, "  %s\n", safety.ArchivePath)
	if request.SafetyBackupTaken == nil {
		return safety.ArchivePath, nil
	}
	if errorValue := request.SafetyBackupTaken(safety.ArchivePath); errorValue != nil {
		return "", fmt.Errorf("the safety backup %s was taken, and could not be recorded for a later attempt: %w", safety.ArchivePath, errorValue)
	}
	return safety.ArchivePath, nil
}

func safetyBackupToRelyOn(keptPath string) (string, error) {
	if keptPath == "" {
		return "", nil
	}
	_, errorValue := os.Stat(keptPath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return "", nil
	}
	if errorValue != nil {
		return "", fmt.Errorf("reading the safety backup an earlier attempt of this take-over kept at %s: %w", keptPath, errorValue)
	}
	return keptPath, nil
}
