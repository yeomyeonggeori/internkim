package box

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yeomyeonggeori/internkim/internal/durablefile"
)

const resetRequestFileName = "reset"

func AskForReset(stateDirectoryPath string) error {
	if errorValue := os.MkdirAll(stateDirectoryPath, 0o700); errorValue != nil {
		return errorValue
	}
	return durablefile.Write(resetRequestPathIn(stateDirectoryPath), nil, 0o600)
}

func WithdrawResetRequest(stateDirectoryPath string) error {
	return removeIfPresent(resetRequestPathIn(stateDirectoryPath))
}

func resetRequestPathIn(stateDirectoryPath string) string {
	return filepath.Join(stateDirectoryPath, resetRequestFileName)
}

type disconnection struct {
	ConnectionKey string `json:"connectionKey,omitempty"`
}

func (client Client) Disconnect(ctx context.Context, identity Identity, connectionKey string) error {
	body, errorValue := json.Marshal(disconnection{ConnectionKey: connectionKey})
	if errorValue != nil {
		return errorValue
	}
	response, errorValue := client.post(ctx, identity, "/api/box/disconnect", body)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return refusalOf(response, "disconnecting this computer from its company")
	}
	return nil
}

func (daemon Daemon) resetIfAsked(ctx context.Context, identity Identity) error {
	askedBy, isAsked := daemon.resetRequest()
	if !isAsked {
		return nil
	}
	log.Printf("%s asks for this computer to be reset; disconnecting it from its company", askedBy)
	connectionKey := daemon.keyOfTheConnectionFile()
	if errorValue := ForgetInstalledCompany(daemon.Places.StateDirectoryPath); errorValue != nil {
		return errorValue
	}
	if errorValue := daemon.disconnect(ctx, identity, connectionKey); errorValue != nil {
		return errorValue
	}
	if daemon.LockAdminPassword != nil {
		if errorValue := daemon.LockAdminPassword(ctx); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := daemon.forgetResetRequests(); errorValue != nil {
		return errorValue
	}
	log.Printf("this computer belongs to no company now")
	return nil
}

func (daemon Daemon) disconnect(ctx context.Context, identity Identity, connectionKey string) error {
	for {
		errorValue := daemon.Client.Disconnect(ctx, identity, connectionKey)
		if errorValue == nil {
			return nil
		}
		log.Printf("disconnecting this computer: %v; trying again in %s", errorValue, announceInterval)
		if errorValue := daemon.waitOnItsNetwork(ctx); errorValue != nil {
			return errorValue
		}
	}
}

func (daemon Daemon) isResetAsked() bool {
	_, isAsked := daemon.resetRequest()
	return isAsked
}

func (daemon Daemon) resetRequest() (string, bool) {
	for _, path := range daemon.resetRequestPaths() {
		if _, errorValue := os.Stat(path); errorValue == nil {
			return path, true
		}
	}
	return "", false
}

func (daemon Daemon) forgetResetRequests() error {
	for _, path := range daemon.resetRequestPaths() {
		if errorValue := removeIfPresent(path); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (daemon Daemon) resetRequestPaths() []string {
	paths := []string{resetRequestPathIn(daemon.Places.StateDirectoryPath)}
	if daemon.Places.BootResetRequestPath != "" {
		paths = append(paths, daemon.Places.BootResetRequestPath)
	}
	return paths
}
