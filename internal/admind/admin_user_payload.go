package admind

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type adminUserMutation struct {
	UserID                 string   `json:"userID,omitempty"`
	Handle                 string   `json:"handle,omitempty"`
	Name                   string   `json:"name,omitempty"`
	Email                  string   `json:"email"`
	Image                  string   `json:"image,omitempty"`
	HireDate               string   `json:"hireDate,omitempty"`
	Note                   string   `json:"note,omitempty"`
	Role                   string   `json:"role"`
	Circles                []string `json:"circles,omitempty"`
	JobTitle               string   `json:"jobTitle,omitempty"`
	PositionLevel          int      `json:"positionLevel,omitempty"`
	GroupID                string   `json:"groupID,omitempty"`
	PhoneNumber            string   `json:"phoneNumber,omitempty"`
	SupervisorID           string   `json:"supervisorID,omitempty"`
	ProjectIDs             []string `json:"projectIDs,omitempty"`
	TeamRole               string   `json:"teamRole,omitempty"`
	EmploymentStatus       string   `json:"employmentStatus,omitempty"`
	IsOrganizationVisible  bool     `json:"isOrganizationVisible,omitempty"`
	MattermostUserID       string   `json:"mattermostUserID,omitempty"`
	MattermostUsername     string   `json:"mattermostUsername,omitempty"`
	Status                 string   `json:"status,omitempty"`
	TemporaryPassword      string   `json:"temporaryPassword,omitempty"`
	TemporaryPasswordEmail string   `json:"temporaryPasswordEmail,omitempty"`
}

func localAdminUserPayload(responseWriter http.ResponseWriter, request *http.Request) (adminUserMutation, bool, bool) {
	var rawPayload adminUserMutation
	if errorValue := json.NewDecoder(request.Body).Decode(&rawPayload); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return adminUserMutation{}, false, false
	}
	payload, hasExplicitCircleMutation, errorValue := normalizeAdminUserPayload(rawPayload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return adminUserMutation{}, false, false
	}
	return payload, hasExplicitCircleMutation, true
}

func normalizeAdminUserPayload(payload adminUserMutation) (adminUserMutation, bool, error) {
	hasExplicitCircleMutation := payload.Circles != nil
	payload.Email = strings.ToLower(strings.TrimSpace(payload.Email))
	if payload.Email == "" {
		return adminUserMutation{}, false, errors.New("email required")
	}
	payload.Image = ""
	payload.Handle = normalizeMattermostHandle(firstNonEmpty(payload.Handle, mattermostUsernameBase(payload.Email)))
	if !isValidMattermostHandle(payload.Handle) {
		return adminUserMutation{}, false, errors.New("handle must start with a letter and contain 3-22 lowercase letters, numbers, dots, dashes, or underscores")
	}
	payload.Name = firstNonEmpty(strings.TrimSpace(payload.Name), payload.Handle)
	payload.Note = strings.TrimSpace(payload.Note)
	payload.Role = normalizeAdminUserRole(payload.Role)
	return payload, hasExplicitCircleMutation, nil
}
