package companyhost

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

type SafetyFinding struct {
	Archive       string
	HeldNoCompany bool
}

type safetyStep int

const (
	takeASafetyBackup safetyStep = iota
	relyOnTheEarlierArchive
	recordThatNoCompanyWasHeld
	keepNothingAnEarlierAttemptFound
)

func safetyStepFor(earlier SafetyFinding, holdsACompany bool) (safetyStep, error) {
	if earlier.HeldNoCompany {
		return keepNothingAnEarlierAttemptFound, nil
	}
	isKept, errorValue := archiveIsKept(earlier.Archive)
	if errorValue != nil {
		return takeASafetyBackup, errorValue
	}
	if isKept {
		return relyOnTheEarlierArchive, nil
	}
	if !holdsACompany {
		return recordThatNoCompanyWasHeld, nil
	}
	return takeASafetyBackup, nil
}

func archiveIsKept(archivePath string) (bool, error) {
	if archivePath == "" {
		return false, nil
	}
	_, errorValue := os.Stat(archivePath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return false, nil
	}
	if errorValue != nil {
		return false, fmt.Errorf("reading the safety backup an earlier attempt of this take-over kept at %s: %w", archivePath, errorValue)
	}
	return true, nil
}

func keepWhatThisComputerHolds(platform backupPlatform, request RestoreRequest, backupDirectoryPath string, machine Machine, progress io.Writer) (string, error) {
	step, errorValue := safetyStepFor(request.EarlierFinding, request.Replace)
	if errorValue != nil {
		return "", errorValue
	}
	switch step {
	case keepNothingAnEarlierAttemptFound:
		fmt.Fprintln(progress, "2/6 An earlier attempt of this take-over found no company on this computer to keep, so nothing is backed up…")
		return "", nil
	case relyOnTheEarlierArchive:
		fmt.Fprintf(progress, "2/6 An earlier attempt of this take-over kept what this computer held in %s, so it is not backed up again…\n", request.EarlierFinding.Archive)
		return request.EarlierFinding.Archive, nil
	case recordThatNoCompanyWasHeld:
		fmt.Fprintln(progress, "2/6 This computer holds no company, so there is nothing to keep before restoring…")
		return "", recordTheFinding(request, SafetyFinding{HeldNoCompany: true})
	}
	fmt.Fprintln(progress, "2/6 Backing up what this computer holds now, so the restore can be undone…")
	safety, errorValue := backupWhileLocked(platform, BackupRequest{DirectoryPath: backupDirectoryPath, Now: request.Now}, machine, io.Discard)
	if errorValue != nil {
		return "", fmt.Errorf("the backup of this computer's current state failed, so nothing was restored: %w", errorValue)
	}
	fmt.Fprintf(progress, "  %s\n", safety.ArchivePath)
	return safety.ArchivePath, recordTheFinding(request, SafetyFinding{Archive: safety.ArchivePath})
}

func recordTheFinding(request RestoreRequest, finding SafetyFinding) error {
	if request.FindingKept == nil {
		return nil
	}
	if errorValue := request.FindingKept(finding); errorValue != nil {
		return fmt.Errorf("what this computer held was not recorded for a later attempt of the take-over, so nothing was restored: %w", errorValue)
	}
	return nil
}
