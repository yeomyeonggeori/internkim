package box

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const adminPasswordCheckInterval = time.Minute

type PendingAdminPassword struct {
	CompanyID string       `json:"-"`
	SettingID string       `json:"settingID"`
	Sealed    SealedSecret `json:"sealed"`
}

type AdminPasswordOutcome string

const (
	AdminPasswordApplied AdminPasswordOutcome = "applied"
	AdminPasswordFailed  AdminPasswordOutcome = "failed"
)

func (client Client) PendingAdminPassword(ctx context.Context, identity Identity) (*PendingAdminPassword, error) {
	response, errorValue := client.post(ctx, identity, "/api/box/admin-password", nil)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, refusalOf(response, "asking for this box's admin password")
	}
	var answered struct {
		CompanyID string                `json:"companyID"`
		Change    *PendingAdminPassword `json:"change"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answered); errorValue != nil {
		return nil, fmt.Errorf("asking for this box's admin password: %w", errorValue)
	}
	if answered.Change == nil {
		return nil, nil
	}
	answered.Change.CompanyID = answered.CompanyID
	return answered.Change, nil
}

func (client Client) ReportAdminPassword(ctx context.Context, identity Identity, settingID string, result AdminPasswordOutcome) error {
	body, errorValue := json.Marshal(map[string]string{"settingID": settingID, "result": string(result)})
	if errorValue != nil {
		return errorValue
	}
	response, errorValue := client.post(ctx, identity, "/api/box/admin-password/outcome", body)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return refusalOf(response, "reporting this box's admin password outcome")
	}
	return nil
}

func (daemon Daemon) watchForAdminPassword(ctx context.Context, identity Identity) {
	var handledSettingID string
	var handledResult AdminPasswordOutcome
	for {
		handledSettingID, handledResult = daemon.checkForAdminPassword(ctx, identity, handledSettingID, handledResult)
		if errorValue := daemon.watcherSleep(ctx, adminPasswordCheckInterval); errorValue != nil {
			return
		}
	}
}

func (daemon Daemon) checkForAdminPassword(ctx context.Context, identity Identity, handledSettingID string, handledResult AdminPasswordOutcome) (string, AdminPasswordOutcome) {
	pending, errorValue := daemon.Client.PendingAdminPassword(ctx, identity)
	if errorValue != nil {
		log.Printf("asking for the admin password: %v", errorValue)
		return handledSettingID, handledResult
	}
	if pending == nil {
		return handledSettingID, handledResult
	}
	if pending.SettingID != handledSettingID {
		handledSettingID, handledResult = pending.SettingID, daemon.applyAdminPassword(ctx, identity, *pending)
	}
	if errorValue := daemon.Client.ReportAdminPassword(ctx, identity, handledSettingID, handledResult); errorValue != nil {
		log.Printf("reporting the admin password %s as %s: %v", handledSettingID, handledResult, errorValue)
	}
	return handledSettingID, handledResult
}

func (daemon Daemon) applyAdminPassword(ctx context.Context, identity Identity, pending PendingAdminPassword) AdminPasswordOutcome {
	purpose := AdminPasswordPurpose(pending.CompanyID, identity.EncryptionPublicKey(), pending.SettingID)
	password, errorValue := identity.OpenSecret(pending.Sealed, purpose)
	if errorValue != nil {
		log.Printf("opening the admin password %s: %v", pending.SettingID, errorValue)
		return AdminPasswordFailed
	}
	if password == "" || strings.ContainsAny(password, "\r\n\x00") {
		log.Printf("the admin password %s is empty or holds a line break", pending.SettingID)
		return AdminPasswordFailed
	}
	if errorValue := daemon.SetAdminPassword(ctx, password); errorValue != nil {
		log.Printf("setting the admin password %s: %v", pending.SettingID, errorValue)
		return AdminPasswordFailed
	}
	log.Printf("set the admin password %s", pending.SettingID)
	return AdminPasswordApplied
}
