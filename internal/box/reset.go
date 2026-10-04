package box

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
)

func (client Client) Release(ctx context.Context, identity Identity) error {
	response, errorValue := client.post(ctx, identity, "/api/box/release", nil)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return refusalOf(response, "releasing this box from its company")
	}
	return nil
}

func (daemon Daemon) resetIfAsked(ctx context.Context, identity Identity) error {
	if !daemon.isResetAsked() {
		return nil
	}
	log.Printf("%s asks for this box to be reset; releasing it from its company", daemon.Places.ResetRequestPath)
	if errorValue := os.Remove(daemon.companyMarkerPath()); errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
		return errorValue
	}
	for {
		errorValue := daemon.Client.Release(ctx, identity)
		if errorValue == nil {
			break
		}
		log.Printf("releasing this box: %v; trying again in %s", errorValue, announceInterval)
		daemon.getOnlineWhileEmpty(ctx, identity)
		if errorValue := daemon.sleep(ctx, announceInterval); errorValue != nil {
			return errorValue
		}
	}
	if daemon.LockAdminPassword != nil {
		if errorValue := daemon.LockAdminPassword(ctx); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := os.Remove(daemon.Places.ResetRequestPath); errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
		return errorValue
	}
	log.Printf("this box belongs to no company now")
	return nil
}

func (daemon Daemon) isResetAsked() bool {
	if daemon.Places.ResetRequestPath == "" {
		return false
	}
	_, errorValue := os.Stat(daemon.Places.ResetRequestPath)
	return errorValue == nil
}
