package box

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
)

func (daemon Daemon) forgetAnEarlierCompany(ctx context.Context, companyID, companyName string) error {
	if daemon.ForgetEveryCompany == nil {
		return nil
	}
	holdsAnother, errorValue := daemon.holdsAnotherCompany(companyID)
	if errorValue != nil {
		return fmt.Errorf("asking whose data this computer holds before it serves %s: %w", companyName, errorValue)
	}
	if !holdsAnother {
		return nil
	}
	log.Printf("this computer holds another company's data; removing all of it before it serves %s", companyName)
	if errorValue := daemon.ForgetEveryCompany(); errorValue != nil {
		return fmt.Errorf("removing the earlier company's data, so %s is not installed beside it: %w", companyName, errorValue)
	}
	if errorValue := daemon.lockTheEarlierCompanyOut(ctx); errorValue != nil {
		return errorValue
	}
	return daemon.forgetTheEarlierCompanyHere()
}

func (daemon Daemon) lockTheEarlierCompanyOut(ctx context.Context) error {
	if daemon.LockAdminPassword == nil {
		return nil
	}
	if errorValue := daemon.LockAdminPassword(ctx); errorValue != nil {
		return fmt.Errorf("locking the admin password the earlier company set: %w", errorValue)
	}
	return nil
}

func (daemon Daemon) holdsAnotherCompany(companyID string) (bool, error) {
	keyOwner, errorValue := companyKeyOwner(daemon.Places.StateDirectoryPath)
	if errorValue != nil {
		return false, errorValue
	}
	if keyOwner != "" && keyOwner != companyID {
		return true, nil
	}
	if daemon.HoldsAnotherCompany == nil {
		return false, nil
	}
	return daemon.HoldsAnotherCompany(companyID)
}

const TakeOverRetryFileName = "take-over-retry.json"

func (daemon Daemon) forgetTheEarlierCompanyHere() error {
	daemon.forgetTheQueueAddress()
	return errors.Join(
		ForgetInstalledCompany(daemon.Places.StateDirectoryPath),
		daemon.forgetServiceLease(),
		removeIfPresent(daemon.statePath(modelKeyAwaitingRestartFileName)),
		removeIfPresent(daemon.statePath(modelKeyFromCompanyFileName)),
		removeIfPresent(daemon.statePath(TakeOverRetryFileName)),
		daemon.forgetSeveralBoxes(),
		removeIfPresent(companyKeyPathIn(daemon.Places.StateDirectoryPath)),
	)
}

func (daemon Daemon) forgetSeveralBoxes() error {
	if daemon.Places.SeveralBoxesPath == "" {
		return nil
	}
	return removeIfPresent(daemon.Places.SeveralBoxesPath)
}

func companyKeyOwner(stateDirectoryPath string) (string, error) {
	document, errorValue := os.ReadFile(companyKeyPathIn(stateDirectoryPath))
	if errors.Is(errorValue, os.ErrNotExist) {
		return "", nil
	}
	if errorValue != nil {
		return "", errorValue
	}
	var stored storedCompanyKey
	if errorValue := json.Unmarshal(document, &stored); errorValue != nil {
		return "", fmt.Errorf("the company key at %s is not one this box wrote: %w", companyKeyPathIn(stateDirectoryPath), errorValue)
	}
	return stored.CompanyID, nil
}
