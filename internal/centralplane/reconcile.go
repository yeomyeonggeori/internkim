package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ReconciledWorkCalendarDay struct {
	Date        string `json:"date"`
	WorkMode    string `json:"workMode"`
	WorkingDate bool   `json:"workingDate"`
	Holiday     bool   `json:"holiday"`
}

type ReconciledWorkBreakPeriod struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type ReconciledWorkPolicy struct {
	WorkMode            string                      `json:"workMode"`
	WorkingWeekdays     []int                       `json:"workingWeekdays"`
	DailyTargetMinutes  int                         `json:"dailyTargetMinutes"`
	WeeklyTargetMinutes int                         `json:"weeklyTargetMinutes"`
	ReferenceStartTime  string                      `json:"referenceStartTime"`
	FixedStartTime      string                      `json:"fixedStartTime"`
	FixedEndTime        string                      `json:"fixedEndTime"`
	CoreTimeEnabled     bool                        `json:"coreTimeEnabled"`
	CoreStartTime       string                      `json:"coreStartTime"`
	CoreEndTime         string                      `json:"coreEndTime"`
	BreakPeriods        []ReconciledWorkBreakPeriod `json:"breakPeriods"`
	NightStartTime      string                      `json:"nightStartTime"`
	NightEndTime        string                      `json:"nightEndTime"`
}

type ReconcileWindow struct {
	Platform     string
	WorkMode     string
	WorkPolicy   *ReconciledWorkPolicy
	From         time.Time
	To           time.Time
	WorkCalendar []ReconciledWorkCalendarDay
}

type ReconcileResult struct {
	Added   int      `json:"added"`
	Removed int      `json:"removed"`
	Refused []string `json:"refused"`
}

func (client *Client) ReconcileAttendance(ctx context.Context, window ReconcileWindow) (ReconcileResult, error) {
	if window.WorkCalendar == nil {
		window.WorkCalendar = []ReconciledWorkCalendarDay{}
	}
	offered := map[string]any{
		"platform":     window.Platform,
		"workMode":     window.WorkMode,
		"from":         window.From.UTC().Format(time.RFC3339),
		"to":           window.To.UTC().Format(time.RFC3339),
		"workCalendar": window.WorkCalendar,
	}
	if window.WorkPolicy != nil {
		offered["workPolicy"] = window.WorkPolicy
	}
	payload, errorValue := json.Marshal(offered)
	if errorValue != nil {
		return ReconcileResult{}, errorValue
	}

	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(client.settings.AppURL, "/")+"/api/agent/attendance-reconcile", bytes.NewReader(payload))
	if errorValue != nil {
		return ReconcileResult{}, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return ReconcileResult{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return ReconcileResult{}, fmt.Errorf("central plane refused the reconciliation: %s", response.Status)
	}

	var result ReconcileResult
	if errorValue := json.NewDecoder(response.Body).Decode(&result); errorValue != nil {
		return ReconcileResult{}, errorValue
	}
	return result, nil
}
