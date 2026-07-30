package admind

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	attendanceStatusSyncInterval     = time.Minute
	attendanceClockInStatusEmoji     = "office"
	attendanceClockOutStatusEmoji    = "wave"
	mattermostRecentCustomStatusPref = "recent_custom_statuses"
	mattermostCustomStatusCategory   = "custom_status"
)

type mattermostCustomStatus struct {
	Emoji     string `json:"emoji"`
	Text      string `json:"text"`
	Duration  string `json:"duration"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type mattermostPreference struct {
	UserID   string `json:"user_id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Value    string `json:"value"`
}

func (service *Service) startMattermostAttendanceStatusSync(ctx context.Context) {
	if strings.TrimSpace(readTrimmedFile(service.Configuration.MattermostAdminPasswordPath)) == "" {
		return
	}
	go func() {
		lastStatusByUser := map[string]string{}
		seededPresetsByUser := map[string]string{}
		service.seedMattermostAttendanceStatuses(ctx, lastStatusByUser, seededPresetsByUser)
		ticker := time.NewTicker(attendanceStatusSyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				service.syncMattermostAttendanceStatuses(ctx, lastStatusByUser, seededPresetsByUser)
			}
		}
	}()
}

func (service *Service) seedMattermostAttendanceStatuses(ctx context.Context, lastStatusByUser map[string]string, seededPresetsByUser map[string]string) {
	service.walkMattermostAttendanceStatuses(ctx, seededPresetsByUser, func(user mattermostUserRecord) {
		lastStatusByUser[user.ID] = mattermostCustomStatusText(user)
	})
}

func (service *Service) syncMattermostAttendanceStatuses(ctx context.Context, lastStatusByUser map[string]string, seededPresetsByUser map[string]string) {
	service.walkMattermostAttendanceStatuses(ctx, seededPresetsByUser, func(user mattermostUserRecord) {
		service.applyAttendanceStatusChange(ctx, user, lastStatusByUser)
	})
}

func (service *Service) walkMattermostAttendanceStatuses(
	ctx context.Context,
	seededPresetsByUser map[string]string,
	visitUser func(mattermostUserRecord),
) {
	syncContext, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	adminToken, errorValue := service.mattermostAdminToken(syncContext)
	if errorValue != nil {
		log.Printf("attendance status sync admin token failed: %v", errorValue)
		return
	}
	teamRecord, errorValue := service.ensureMattermostTeam(syncContext, adminToken)
	if errorValue != nil {
		log.Printf("attendance status sync team lookup failed: %v", errorValue)
		return
	}
	users, errorValue := service.teamMattermostUsers(syncContext, adminToken, teamRecord.ID)
	if errorValue != nil {
		log.Printf("attendance status sync user list failed: %v", errorValue)
		return
	}
	presets := service.attendanceStatusPresets()
	presetsSignature := attendanceStatusPresetsSignature(presets)
	for _, user := range users {
		service.ensureAttendanceStatusPresetsForUser(syncContext, adminToken, user.ID, presets, presetsSignature, seededPresetsByUser)
		visitUser(user)
	}
}

func (service *Service) applyAttendanceStatusChange(ctx context.Context, user mattermostUserRecord, lastStatusByUser map[string]string) {
	statusText := mattermostCustomStatusText(user)
	if lastStatusByUser[user.ID] == statusText {
		return
	}
	command, isCommand, errorValue := service.parseAttendancePostCommand(statusText)
	if !isCommand || command.IsTimeUpdate {
		if errorValue != nil {
			log.Printf("attendance status %q for user %s not applied: %v", statusText, user.ID, errorValue)
		}
		lastStatusByUser[user.ID] = statusText
		return
	}
	payload := mattermostInteractivePayload{UserID: user.ID}
	payload.Context.LocationID = command.LocationID
	if _, errorValue := service.recordAttendanceFromMattermost(ctx, payload, command.Kind); errorValue != nil {
		log.Printf("attendance status %q for user %s failed: %v", statusText, user.ID, errorValue)
		return
	}
	lastStatusByUser[user.ID] = statusText
}

func mattermostCustomStatusText(user mattermostUserRecord) string {
	raw, found := user.Props["customStatus"]
	if !found || len(raw) == 0 {
		return ""
	}
	if text := parseMattermostCustomStatusText(raw); text != "" {
		return text
	}
	var encoded string
	if json.Unmarshal(raw, &encoded) != nil {
		return ""
	}
	return parseMattermostCustomStatusText(json.RawMessage(encoded))
}

func parseMattermostCustomStatusText(raw json.RawMessage) string {
	var status mattermostCustomStatus
	if json.Unmarshal(raw, &status) != nil {
		return ""
	}
	return strings.TrimSpace(status.Text)
}

func (service *Service) syncMattermostCustomStatusToAttendance(ctx context.Context, userToken string, kind string, location attendanceLocation) {
	status, ok := service.attendanceCustomStatusForKind(kind, location)
	if !ok {
		return
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/users/me/status/custom", userToken, status, nil); errorValue != nil {
		log.Printf("attendance custom status write-back failed: %v", errorValue)
	}
}

func (service *Service) attendanceCustomStatusForKind(kind string, location attendanceLocation) (mattermostCustomStatus, bool) {
	locationID := strings.TrimSpace(location.ID)
	for _, action := range service.mattermostAttendanceEntryActions() {
		label := strings.TrimSpace(action.Name)
		if label == "" {
			continue
		}
		actionKind := strings.TrimSpace(action.Integration.Context.Action)
		if kind == attendanceKindClockOut && actionKind == attendanceClockOutAction {
			return mattermostCustomStatus{Emoji: attendanceClockOutStatusEmoji, Text: label}, true
		}
		if kind == attendanceKindClockIn && actionKind == attendanceClockInAction && strings.TrimSpace(action.Integration.Context.LocationID) == locationID {
			return mattermostCustomStatus{Emoji: attendanceClockInStatusEmoji, Text: label}, true
		}
	}
	return mattermostCustomStatus{}, false
}

func (service *Service) attendanceStatusPresets() []mattermostCustomStatus {
	actions := service.mattermostAttendanceEntryActions()
	presets := make([]mattermostCustomStatus, 0, len(actions))
	for _, action := range actions {
		label := strings.TrimSpace(action.Name)
		if label == "" {
			continue
		}
		emoji := attendanceClockInStatusEmoji
		if strings.TrimSpace(action.Integration.Context.Action) == attendanceClockOutAction {
			emoji = attendanceClockOutStatusEmoji
		}
		presets = append(presets, mattermostCustomStatus{Emoji: emoji, Text: label})
	}
	return presets
}

func attendanceStatusPresetsSignature(presets []mattermostCustomStatus) string {
	texts := make([]string, 0, len(presets))
	for _, preset := range presets {
		texts = append(texts, preset.Text)
	}
	sort.Strings(texts)
	return strings.Join(texts, "\x00")
}

func (service *Service) ensureAttendanceStatusPresetsForUser(ctx context.Context, adminToken string, userID string, presets []mattermostCustomStatus, presetsSignature string, seededPresetsByUser map[string]string) {
	if seededPresetsByUser[userID] == presetsSignature {
		return
	}
	existing := service.readMattermostRecentCustomStatuses(ctx, adminToken, userID)
	if customStatusPresetsEqual(existing, presets) {
		seededPresetsByUser[userID] = presetsSignature
		return
	}
	if errorValue := service.writeMattermostRecentCustomStatuses(ctx, adminToken, userID, presets); errorValue != nil {
		log.Printf("attendance status preset sync for user %s failed: %v", userID, errorValue)
		return
	}
	seededPresetsByUser[userID] = presetsSignature
}

func customStatusPresetsEqual(existing []mattermostCustomStatus, presets []mattermostCustomStatus) bool {
	if len(existing) != len(presets) {
		return false
	}
	for index := range presets {
		if strings.TrimSpace(existing[index].Text) != strings.TrimSpace(presets[index].Text) {
			return false
		}
	}
	return true
}

func (service *Service) readMattermostRecentCustomStatuses(ctx context.Context, adminToken string, userID string) []mattermostCustomStatus {
	var preference mattermostPreference
	path := "/api/v4/users/" + url.PathEscape(userID) + "/preferences/" + mattermostCustomStatusCategory + "/name/" + mattermostRecentCustomStatusPref
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, adminToken, nil, &preference); errorValue != nil {
		return nil
	}
	var statuses []mattermostCustomStatus
	if json.Unmarshal([]byte(preference.Value), &statuses) != nil {
		return nil
	}
	return statuses
}

func (service *Service) writeMattermostRecentCustomStatuses(ctx context.Context, adminToken string, userID string, statuses []mattermostCustomStatus) error {
	value, errorValue := json.Marshal(statuses)
	if errorValue != nil {
		return errorValue
	}
	preferences := []mattermostPreference{{
		UserID:   userID,
		Category: mattermostCustomStatusCategory,
		Name:     mattermostRecentCustomStatusPref,
		Value:    string(value),
	}}
	path := "/api/v4/users/" + url.PathEscape(userID) + "/preferences"
	return service.mattermostRequest(ctx, http.MethodPut, path, adminToken, preferences, nil)
}
