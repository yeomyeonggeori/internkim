package box

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/durablefile"
)

const boxMarkerFileName = "is-a-box"

func (daemon Daemon) connectedByFile() bool {
	if daemon.installedCompany() != "" || isABox(daemon.Places.StateDirectoryPath) {
		return false
	}
	_, errorValue := companyhost.ReadConnection(daemon.Places.ConnectionFilePath)
	return errorValue == nil
}

func (daemon Daemon) moveOffTheConnectionFile(ctx context.Context) error {
	for {
		errorValue := daemon.claimWithTheConnectionFile(ctx)
		if errorValue == nil {
			log.Printf("this computer spent its connection file and proves itself to its company with its own box key from now on")
			return nil
		}
		if errors.Is(errorValue, ErrCompanyHasOtherComputers) {
			return daemon.stayOnTheConnectionFile(ctx)
		}
		log.Printf("this computer was connected to its company with a connection file and could not move onto a box key of its own: %v; it keeps serving on the file's key and tries again in %s", errorValue, announceInterval)
		if errorValue := daemon.waitOnItsNetwork(ctx); errorValue != nil {
			return errorValue
		}
	}
}

func (daemon Daemon) stayOnTheConnectionFile(ctx context.Context) error {
	log.Printf("this computer stays on its connection file and does not ask again until internkim-box restarts: %v, and moving it onto a box key would disconnect them. To move it, run sudo internkim install with a connection file downloaded from company setup, which makes it the company's only host, or disconnect the other boxes in company setup and run sudo systemctl restart internkim-box", ErrCompanyHasOtherComputers)
	for {
		if errorValue := daemon.waitOnItsNetwork(ctx); errorValue != nil {
			return errorValue
		}
	}
}

func (daemon Daemon) waitOnItsNetwork(ctx context.Context) error {
	daemon.getOnlineWhenOffline(ctx)
	return daemon.sleep(ctx, announceInterval)
}

func (daemon Daemon) claimWithTheConnectionFile(ctx context.Context) error {
	connection, errorValue := companyhost.ReadConnection(daemon.Places.ConnectionFilePath)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := daemon.companiesHeld(); errorValue != nil {
		return fmt.Errorf("asking whose data this computer holds before it claims itself for %s: %w", connection.Company.Name, errorValue)
	}
	identity, errorValue := LoadOrCreateIdentity(identityPathIn(daemon.Places.StateDirectoryPath))
	if errorValue != nil {
		return errorValue
	}
	claimError := daemon.Client.Claim(ctx, identity, connection.AgentKey, false)
	session, isClaimed, errorValue := daemon.Client.Session(ctx, identity, daemon.PackageVersion)
	if errorValue != nil {
		return errors.Join(claimError, errorValue)
	}
	if !isServingCompany(session, isClaimed, connection.Company.ID) {
		return errors.Join(claimError, fmt.Errorf("the central plane does not answer this computer as the box that serves %s", connection.Company.Name))
	}
	return MarkInstalledCompany(daemon.Places.StateDirectoryPath, connection.Company.ID)
}

func isServingCompany(session Session, isClaimed bool, companyID string) bool {
	return isClaimed && session.Configuration.Company.ID == companyID && session.Role == RoleService
}

func rememberThisComputerIsABox(stateDirectoryPath string) error {
	if isABox(stateDirectoryPath) {
		return nil
	}
	return durablefile.Write(filepath.Join(stateDirectoryPath, boxMarkerFileName), nil, 0o600)
}

func isABox(stateDirectoryPath string) bool {
	_, errorValue := os.Stat(filepath.Join(stateDirectoryPath, boxMarkerFileName))
	return errorValue == nil
}
