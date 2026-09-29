package box

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gitlab.com/eastriver/internkim/internal/companyhost"
)

const (
	announceInterval      = 30 * time.Second
	sessionRenewalMargin  = 10 * time.Minute
	shortestSessionWait   = time.Minute
	companyMarkerFileName = "company"
	refreshTokenFileName  = "refresh-token"
)

var ErrConnectedByFile = errors.New("this computer was connected to its company with a connection file")

type Places struct {
	StateDirectoryPath        string
	ConnectionFilePath        string
	CredentialPaths           []string
	ModelKeyPath              string
	CompanyStateDirectoryPath func(companyID string) string
}

type Daemon struct {
	Client  Client
	Places  Places
	Install func(companyhost.Request) error
	Sleep   func(context.Context, time.Duration) error
	Now     func() time.Time
}

func (daemon Daemon) Run(ctx context.Context) error {
	if daemon.connectedByFile() {
		return ErrConnectedByFile
	}
	identity, errorValue := LoadOrCreateIdentity(filepath.Join(daemon.Places.StateDirectoryPath, "identity.json"))
	if errorValue != nil {
		return errorValue
	}
	log.Printf("this box is %s", identity.PublicKey())
	for {
		wait, errorValue := daemon.step(ctx, identity)
		if errorValue != nil {
			log.Printf("%v; trying again in %s", errorValue, announceInterval)
			wait = announceInterval
		}
		if errorValue := daemon.sleep(ctx, wait); errorValue != nil {
			return nil
		}
	}
}

func (daemon Daemon) step(ctx context.Context, identity Identity) (time.Duration, error) {
	if wait, isRenewed, errorValue := daemon.refresh(ctx, identity); isRenewed || errorValue != nil {
		return wait, errorValue
	}
	session, isClaimed, errorValue := daemon.Client.Session(ctx, identity)
	if errorValue != nil {
		return 0, errorValue
	}
	if !isClaimed {
		return announceInterval, daemon.announce(ctx, identity)
	}
	plane := session.Configuration.CentralPlane
	if daemon.installedCompany() == session.Configuration.Company.ID {
		return daemon.untilRenewal(session.Session), daemon.renew(ctx, plane, session.Session, identity)
	}
	sealedModelKey, errorValue := daemon.Client.SealedModelKey(ctx, plane, session.Session.AccessToken)
	if errorValue != nil {
		return 0, errorValue
	}
	if sealedModelKey == nil {
		log.Printf("%s connected this box and has not given it a model key yet", session.Configuration.Company.Name)
		return announceInterval, nil
	}
	modelKey, errorValue := identity.OpenModelKey(*sealedModelKey)
	if errorValue != nil {
		return 0, errorValue
	}
	if errorValue := daemon.installFor(session, modelKey); errorValue != nil {
		return 0, errorValue
	}
	return daemon.untilRenewal(session.Session), nil
}

func (daemon Daemon) announce(ctx context.Context, identity Identity) error {
	isClaimed, errorValue := daemon.Client.Announce(ctx, identity)
	if errorValue != nil {
		return errorValue
	}
	if isClaimed {
		log.Printf("a company claimed this box; asking for its session")
	}
	return nil
}

func (daemon Daemon) refresh(ctx context.Context, identity Identity) (time.Duration, bool, error) {
	plane, isInstalled := daemon.installedPlane()
	if !isInstalled {
		return 0, false, nil
	}
	stored, errorValue := os.ReadFile(daemon.refreshTokenPath())
	refreshToken := strings.TrimSpace(string(stored))
	if errorValue != nil || refreshToken == "" {
		return 0, false, nil
	}
	session, errorValue := daemon.Client.Refresh(ctx, plane, refreshToken)
	if errors.Is(errorValue, ErrRefreshRefused) {
		log.Printf("%v; asking the company for a new session", errorValue)
		return 0, false, nil
	}
	if errorValue != nil {
		return 0, false, errorValue
	}
	if errorValue := daemon.renew(ctx, plane, session, identity); errorValue != nil {
		return 0, false, errorValue
	}
	return daemon.untilRenewal(session), true, nil
}

func (daemon Daemon) installedPlane() (companyhost.CentralPlane, bool) {
	companyID := daemon.installedCompany()
	if companyID == "" {
		return companyhost.CentralPlane{}, false
	}
	connection, errorValue := companyhost.ReadConnection(companyhost.StoredConnectionPath(daemon.Places.CompanyStateDirectoryPath(companyID)))
	if errorValue != nil {
		return companyhost.CentralPlane{}, false
	}
	return connection.CentralPlane, true
}

func (daemon Daemon) keepSession(session HostSession) error {
	if errorValue := daemon.keepRefreshToken(session.RefreshToken); errorValue != nil {
		return errorValue
	}
	for _, path := range daemon.Places.CredentialPaths {
		if errorValue := replaceKeepingOwner(path, session.AccessToken+"\n"); errorValue != nil {
			return fmt.Errorf("keeping the renewed session at %s: %w", path, errorValue)
		}
	}
	return nil
}

func (daemon Daemon) keepRefreshToken(refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	if errorValue := writeFileAtomically(daemon.refreshTokenPath(), []byte(refreshToken+"\n"), 0o600); errorValue != nil {
		return fmt.Errorf("keeping the refresh token: %w", errorValue)
	}
	return nil
}

func (daemon Daemon) refreshTokenPath() string {
	return filepath.Join(daemon.Places.StateDirectoryPath, refreshTokenFileName)
}

func (daemon Daemon) renew(ctx context.Context, plane companyhost.CentralPlane, session HostSession, identity Identity) error {
	if errorValue := daemon.keepSession(session); errorValue != nil {
		return errorValue
	}
	if errorValue := daemon.followModelKey(ctx, plane, session.AccessToken, identity); errorValue != nil {
		log.Printf("the session was renewed and the model key could not be checked: %v", errorValue)
	}
	return nil
}

func (daemon Daemon) followModelKey(ctx context.Context, plane companyhost.CentralPlane, accessToken string, identity Identity) error {
	sealedModelKey, errorValue := daemon.Client.SealedModelKey(ctx, plane, accessToken)
	if errorValue != nil || sealedModelKey == nil {
		return errorValue
	}
	return daemon.applyModelKey(*sealedModelKey, identity)
}

func (daemon Daemon) applyModelKey(sealedModelKey SealedModelKey, identity Identity) error {
	modelKey, errorValue := identity.OpenModelKey(sealedModelKey)
	if errorValue != nil {
		return errorValue
	}
	applied, errorValue := os.ReadFile(daemon.Places.ModelKeyPath)
	if errorValue == nil && string(applied) == modelKey+"\n" {
		return nil
	}
	return replaceKeepingOwner(daemon.Places.ModelKeyPath, modelKey+"\n")
}

func (daemon Daemon) InstallWithConnectionFile(ctx context.Context, connectionKey string, modelKey string) error {
	identity, errorValue := LoadOrCreateIdentity(filepath.Join(daemon.Places.StateDirectoryPath, "identity.json"))
	if errorValue != nil {
		return errorValue
	}
	if errorValue := daemon.Client.Claim(ctx, identity, connectionKey); errorValue != nil {
		return errorValue
	}
	session, isClaimed, errorValue := daemon.Client.Session(ctx, identity)
	if errorValue != nil {
		return errorValue
	}
	if !isClaimed {
		return fmt.Errorf("the central plane accepted the connection file and then gave this computer no company")
	}
	return daemon.installFor(session, modelKey)
}

func (daemon Daemon) installFor(session Session, modelKey string) error {
	connection := connectionOf(session)
	log.Printf("installing the server for %s", connection.Company.Name)
	if errorValue := daemon.Install(companyhost.Request{
		Connection:         &connection,
		StateDirectoryPath: daemon.Places.CompanyStateDirectoryPath(connection.Company.ID),
		ModelKey:           modelKey,
		PromptForModelKey:  func() (string, error) { return modelKey, nil },
	}); errorValue != nil {
		return fmt.Errorf("installing the server for %s: %w", connection.Company.Name, errorValue)
	}
	if errorValue := daemon.keepRefreshToken(session.Session.RefreshToken); errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(daemon.companyMarkerPath(), []byte(connection.Company.ID+"\n"), 0o600)
}

func connectionOf(session Session) companyhost.Connection {
	return companyhost.Connection{
		SchemaVersion: session.Configuration.SchemaVersion,
		AppURL:        session.Configuration.AppURL,
		Company:       session.Configuration.Company,
		CentralPlane:  session.Configuration.CentralPlane,
		GatewayURL:    session.Configuration.GatewayURL,
		AgentKey:      session.Session.AccessToken,
	}
}

func (daemon Daemon) untilRenewal(session HostSession) time.Duration {
	wait := time.Unix(session.ExpiresAt, 0).Sub(daemon.now()) - sessionRenewalMargin
	return max(wait, shortestSessionWait)
}

func (daemon Daemon) connectedByFile() bool {
	if daemon.installedCompany() != "" {
		return false
	}
	_, errorValue := os.Stat(daemon.Places.ConnectionFilePath)
	return errorValue == nil
}

func (daemon Daemon) installedCompany() string {
	document, errorValue := os.ReadFile(daemon.companyMarkerPath())
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func (daemon Daemon) companyMarkerPath() string {
	return filepath.Join(daemon.Places.StateDirectoryPath, companyMarkerFileName)
}

func (daemon Daemon) now() time.Time {
	if daemon.Now == nil {
		return time.Now()
	}
	return daemon.Now()
}

func (daemon Daemon) sleep(ctx context.Context, wait time.Duration) error {
	if daemon.Sleep != nil {
		return daemon.Sleep(ctx, wait)
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func replaceKeepingOwner(path, contents string) error {
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		return errorValue
	}
	temporary := path + ".renewing"
	if errorValue := os.WriteFile(temporary, []byte(contents), information.Mode().Perm()); errorValue != nil {
		return errorValue
	}
	if errorValue := os.Chmod(temporary, information.Mode().Perm()); errorValue != nil {
		os.Remove(temporary)
		return errorValue
	}
	if owner, isUnix := information.Sys().(*syscall.Stat_t); isUnix {
		if errorValue := os.Chown(temporary, int(owner.Uid), int(owner.Gid)); errorValue != nil {
			os.Remove(temporary)
			return errorValue
		}
	}
	return os.Rename(temporary, path)
}
