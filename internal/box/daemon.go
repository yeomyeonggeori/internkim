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

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

const (
	announceInterval      = 30 * time.Second
	sessionRenewalMargin  = 10 * time.Minute
	shortestSessionWait   = time.Minute
	companyMarkerFileName = "company"
)

var ErrConnectedByFile = errors.New("this computer was connected to its company with a connection file")

type Places struct {
	StateDirectoryPath        string
	PairingPageListenAddress  string
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
	identity, errorValue := LoadOrCreateIdentity(identityPathIn(daemon.Places.StateDirectoryPath))
	if errorValue != nil {
		return errorValue
	}
	log.Printf("this box is %s", identity.PublicKey())
	var page *pairingPage
	if daemon.installedCompany() == "" {
		page = daemon.openPairingPage(identity)
	}
	defer func() { page.close() }()
	for {
		wait, isClaimed, errorValue := daemon.step(ctx, identity, page.localPage())
		if errorValue == nil || isClaimed {
			page = daemon.pairingPageFor(isClaimed, page, identity)
		}
		if errorValue != nil {
			log.Printf("%v; trying again in %s", errorValue, announceInterval)
			wait = announceInterval
		}
		if errorValue := daemon.sleep(ctx, wait); errorValue != nil {
			return nil
		}
	}
}

func (daemon Daemon) pairingPageFor(isClaimed bool, page *pairingPage, identity Identity) *pairingPage {
	if isClaimed {
		page.close()
		return nil
	}
	if page == nil {
		return daemon.openPairingPage(identity)
	}
	return page
}

func (daemon Daemon) step(ctx context.Context, identity Identity, page LocalPage) (time.Duration, bool, error) {
	session, isClaimed, errorValue := daemon.Client.Session(ctx, identity)
	if errorValue != nil {
		return 0, false, errorValue
	}
	if !isClaimed {
		return announceInterval, false, daemon.announce(ctx, identity, page)
	}
	if errorValue := forgetPairingCode(daemon.Places.StateDirectoryPath); errorValue != nil {
		return 0, true, errorValue
	}
	wait, errorValue := daemon.serveClaimed(session, identity)
	return wait, true, errorValue
}

func (daemon Daemon) serveClaimed(session Session, identity Identity) (time.Duration, error) {
	if daemon.installedCompany() == session.Configuration.Company.ID {
		return daemon.untilRenewal(session.Session), daemon.renew(session, identity)
	}
	if session.SealedModelKey == nil {
		log.Printf("%s connected this box and has not given it a model key yet", session.Configuration.Company.Name)
		return announceInterval, nil
	}
	modelKey, errorValue := identity.OpenModelKey(*session.SealedModelKey, session.Configuration.Company.ID)
	if errorValue != nil {
		return 0, errorValue
	}
	if errorValue := daemon.installFor(session, modelKey); errorValue != nil {
		return 0, errorValue
	}
	return daemon.untilRenewal(session.Session), nil
}

func (daemon Daemon) announce(ctx context.Context, identity Identity, page LocalPage) error {
	_, hasLiveCode, errorValue := ShownPairingCode(daemon.Places.StateDirectoryPath, daemon.now())
	if errorValue != nil {
		return errorValue
	}
	announcement, errorValue := daemon.Client.Announce(ctx, identity, AnnouncementRequest{
		WantsPairingCode: !hasLiveCode,
		LocalPage:        page,
	})
	if errorValue != nil {
		return errorValue
	}
	if announcement.IsClaimed {
		log.Printf("a company claimed this box; asking for its session")
		return forgetPairingCode(daemon.Places.StateDirectoryPath)
	}
	if announcement.PairingCode == nil {
		return nil
	}
	log.Printf("connect this box at %s/settings/setup with the code %s before %s",
		strings.TrimRight(daemon.Client.AppURL, "/"), announcement.PairingCode.Code,
		announcement.PairingCode.ExpiresAt.Local().Format("15:04"))
	return showPairingCode(daemon.Places.StateDirectoryPath, *announcement.PairingCode)
}

func (daemon Daemon) renew(session Session, identity Identity) error {
	for _, path := range daemon.Places.CredentialPaths {
		if errorValue := replaceKeepingOwner(path, session.Session.AccessToken+"\n"); errorValue != nil {
			return fmt.Errorf("keeping the renewed session at %s: %w", path, errorValue)
		}
	}
	if session.SealedModelKey == nil {
		return nil
	}
	modelKey, errorValue := identity.OpenModelKey(*session.SealedModelKey, session.Configuration.Company.ID)
	if errorValue != nil {
		return errorValue
	}
	return replaceKeepingOwner(daemon.Places.ModelKeyPath, modelKey+"\n")
}

func (daemon Daemon) InstallWithConnectionFile(ctx context.Context, connectionKey string, modelKey string) error {
	identity, errorValue := LoadOrCreateIdentity(identityPathIn(daemon.Places.StateDirectoryPath))
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
	return installedCompanyIn(daemon.Places.StateDirectoryPath)
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
