package box

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

func (daemon Daemon) forgetAnEarlierCompany(ctx context.Context, companyID, companyName string) error {
	if daemon.ForgetEveryCompany == nil || daemon.takeOverIsUnderway() {
		return nil
	}
	held, errorValue := daemon.companiesHeld()
	if errorValue != nil {
		return fmt.Errorf("asking whose data this computer holds before it serves %s: %w", companyName, errorValue)
	}
	if daemon.installedCompany() == companyID || held.Current == companyID {
		return daemon.forgetOtherCompanyDirectories(held, companyID, companyName)
	}
	others, errorValue := daemon.companiesOtherThan(held, companyID)
	if errorValue != nil {
		return fmt.Errorf("asking whose data this computer holds before it serves %s: %w", companyName, errorValue)
	}
	if len(others) == 0 {
		return nil
	}
	log.Printf("this computer holds the data of companies %s; removing all of it before it serves %s", strings.Join(others, ", "), companyName)
	if errorValue := daemon.ForgetEveryCompany(); errorValue != nil {
		return fmt.Errorf("removing the data of companies %s, so %s is not installed beside it: %w", strings.Join(others, ", "), companyName, errorValue)
	}
	if errorValue := daemon.lockTheEarlierCompanyOut(ctx); errorValue != nil {
		return errorValue
	}
	return daemon.forgetTheEarlierCompanyHere()
}

func (daemon Daemon) takeOverIsUnderway() bool {
	return daemon.TakeOverIsUnderway != nil && daemon.TakeOverIsUnderway()
}

func (daemon Daemon) companiesHeld() (companyhost.CompaniesHeld, error) {
	if daemon.CompaniesHeld == nil {
		return companyhost.CompaniesHeld{}, nil
	}
	return daemon.CompaniesHeld()
}

func (daemon Daemon) companiesOtherThan(held companyhost.CompaniesHeld, companyID string) ([]string, error) {
	keyOwner, errorValue := companyKeyOwner(daemon.Places.StateDirectoryPath)
	if errorValue != nil {
		return nil, errorValue
	}
	held.Directories = append(slices.Clone(held.Directories), keyOwner, daemon.installedCompany())
	return held.OtherThan(companyID), nil
}

func (daemon Daemon) forgetOtherCompanyDirectories(held companyhost.CompaniesHeld, companyID, companyName string) error {
	if daemon.ForgetCompanyDirectory == nil {
		return nil
	}
	for _, otherID := range held.OtherThan(companyID) {
		if otherID == held.Current {
			continue
		}
		log.Printf("removing the directory of company %s, which this computer serving %s does not use", otherID, companyName)
		if errorValue := daemon.ForgetCompanyDirectory(otherID); errorValue != nil {
			return fmt.Errorf("removing the directory of company %s before serving %s: %w", otherID, companyName, errorValue)
		}
	}
	return nil
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
