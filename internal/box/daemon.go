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
	session, isClaimed, errorValue := daemon.Client.Session(ctx, identity)
	if errorValue != nil {
		return 0, errorValue
	}
	if !isClaimed {
		return announceInterval, daemon.announce(ctx, identity)
	}
	if session.SealedModelKey == nil {
		log.Printf("%s connected this box and has not given it a model key yet", session.Configuration.Company.Name)
		return announceInterval, nil
	}
	modelKey, errorValue := identity.OpenModelKey(*session.SealedModelKey)
	if errorValue != nil {
		return 0, errorValue
	}
	if errorValue := daemon.apply(session, modelKey); errorValue != nil {
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

func (daemon Daemon) apply(session Session, modelKey string) error {
	companyID := session.Configuration.Company.ID
	if daemon.installedCompany() != companyID {
		return daemon.installFor(session, modelKey)
	}
	for _, path := range daemon.Places.CredentialPaths {
		if errorValue := replaceKeepingOwner(path, session.Session.AccessToken+"\n"); errorValue != nil {
			return fmt.Errorf("keeping the renewed session at %s: %w", path, errorValue)
		}
	}
	return replaceKeepingOwner(daemon.Places.ModelKeyPath, modelKey+"\n")
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
