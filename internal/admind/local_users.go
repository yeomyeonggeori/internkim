package admind

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type localMattermostTeamMember struct {
	UserID string `json:"user_id"`
	Roles  string `json:"roles"`
}

func (service *Service) localListUsers(responseWriter http.ResponseWriter, request *http.Request) {
	token, errorValue := service.mattermostAdminToken(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	teamRecord, errorValue := service.ensureMattermostTeam(request.Context(), token)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	users, errorValue := service.teamMattermostUsers(request.Context(), token, teamRecord.ID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	roleByUserID, errorValue := service.localMattermostTeamRoles(request.Context(), token, teamRecord.ID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	response := pagesUsersResponse{Records: localAdminUserRecords(users, roleByUserID)}
	service.writeLocalUsersResponse(responseWriter, request, response)
}

func (service *Service) localUpsertUser(responseWriter http.ResponseWriter, request *http.Request) {
	payload, hasExplicitCircleMutation, isValid := localAdminUserPayload(responseWriter, request)
	if !isValid {
		return
	}
	userID, errorValue := service.localBlueclawPersonIDByEmail(request.Context(), payload.Email)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if strings.TrimSpace(payload.UserID) == "" {
		payload.UserID = firstNonEmpty(userID, newInternKimUserID())
	}
	payload.Circles = normalizeAdminUserCircles(payload.Circles, payload.Role)
	if payload.Role != "admin" {
		isLastAdmin, errorValue := service.localIsLastMattermostAdminByEmail(request.Context(), payload.Email)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		if isLastAdmin {
			http.Error(responseWriter, "cannot demote the last admin user", http.StatusBadRequest)
			return
		}
	}
	systemSafePayload := payload
	systemSafePayload.Role = "member"
	provisionResult, errorValue := service.provisionMattermostUserWithPassword(request.Context(), systemSafePayload, "")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	payload.MattermostUserID = provisionResult.UserID
	payload.MattermostUsername = provisionResult.Username
	payload.Handle = provisionResult.Username
	payload.Status = provisionResult.Status
	if errorValue := service.applyLocalMattermostTeamRole(request.Context(), provisionResult.UserID, payload.Role == "admin"); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if errorValue := service.localSaveBlueclawPerson(request.Context(), payload, hasExplicitCircleMutation); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if hasExplicitCircleMutation {
		if errorValue := service.syncMattermostUserCircleMemberships(request.Context(), payload); errorValue != nil {
			log.Printf("Mattermost circle membership sync failed: %v", errorValue)
		}
	}
	service.triggerUsersSync(request.Context())
	response := pagesUsersResponse{Records: []adminUserMutation{payload}}
	responseBody, errorValue := service.localUsersResponseBody(request.Context(), response)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if provisionResult.TemporaryPassword != "" {
		responseBody = localUsersBodyWithTemporaryPassword(responseBody, provisionResult.TemporaryPassword, payload.Email)
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_, _ = responseWriter.Write(responseBody)
}

func (service *Service) localRemoveUser(responseWriter http.ResponseWriter, request *http.Request, idOrEmail string) {
	token, errorValue := service.mattermostAdminToken(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	userRecord, found, errorValue := service.localMattermostUserByIDOrEmail(request.Context(), token, idOrEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if !found {
		service.writeJSON(responseWriter, map[string]bool{"ok": true})
		return
	}
	isLastAdmin, errorValue := service.localIsLastMattermostAdmin(request.Context(), token, userRecord.ID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if isLastAdmin {
		http.Error(responseWriter, "cannot remove the last admin user", http.StatusConflict)
		return
	}
	if errorValue := service.deactivateMattermostUserByID(request.Context(), userRecord.ID); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if errorValue := service.demoteLocalBlueclawPersonBeforeRemoval(request.Context(), userRecord); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if errorValue := service.removeBlueclawPerson(request.Context(), userRecord.Email); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.triggerUsersSync(request.Context())
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func (service *Service) localResetUserPassword(responseWriter http.ResponseWriter, request *http.Request, idOrEmail string) {
	token, errorValue := service.mattermostAdminToken(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	userRecord, found, errorValue := service.localMattermostUserByIDOrEmail(request.Context(), token, idOrEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if !found {
		http.Error(responseWriter, "user not found", http.StatusNotFound)
		return
	}
	resetResult, errorValue := service.resetMattermostUserPasswordAndHistory(request.Context(), adminUserMutation{
		Email:            userRecord.Email,
		MattermostUserID: userRecord.ID,
	})
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]any{
		"temporaryPassword":      resetResult.TemporaryPassword,
		"temporaryPasswordEmail": userRecord.Email,
	})
}

func localAdminUserPayload(responseWriter http.ResponseWriter, request *http.Request) (adminUserMutation, bool, bool) {
	var payload adminUserMutation
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return adminUserMutation{}, false, false
	}
	hasExplicitCircleMutation := payload.Circles != nil
	payload.Email = strings.ToLower(strings.TrimSpace(payload.Email))
	if payload.Email == "" {
		http.Error(responseWriter, "email required", http.StatusBadRequest)
		return adminUserMutation{}, false, false
	}
	payload.Handle = normalizeMattermostHandle(firstNonEmpty(payload.Handle, mattermostUsernameBase(payload.Email)))
	if !isValidMattermostHandle(payload.Handle) {
		http.Error(responseWriter, "handle must start with a letter and contain 3-22 lowercase letters, numbers, dots, dashes, or underscores", http.StatusBadRequest)
		return adminUserMutation{}, false, false
	}
	payload.Name = firstNonEmpty(strings.TrimSpace(payload.Name), payload.Handle)
	payload.Role = normalizeAdminUserRole(payload.Role)
	return payload, hasExplicitCircleMutation, true
}

func localAdminUserRecords(users []mattermostUserRecord, roleByUserID map[string]string) []adminUserMutation {
	records := make([]adminUserMutation, 0, len(users))
	for _, userRecord := range users {
		records = append(records, adminUserMutation{
			Email:              strings.ToLower(strings.TrimSpace(userRecord.Email)),
			Handle:             userRecord.Username,
			Name:               firstNonEmpty(userRecord.Nickname, userRecord.DisplayName, userRecord.Username),
			Role:               localMattermostAdminRole(userRecord, roleByUserID[userRecord.ID]),
			MattermostUserID:   userRecord.ID,
			MattermostUsername: userRecord.Username,
			Status:             "active",
		})
	}
	return records
}

func localMattermostAdminRole(userRecord mattermostUserRecord, teamRoles string) string {
	if strings.Contains(" "+userRecord.Roles+" ", " system_admin ") {
		return "admin"
	}
	if strings.Contains(" "+teamRoles+" ", " team_admin ") {
		return "admin"
	}
	return "member"
}

func (service *Service) localMattermostTeamRoles(ctx context.Context, token string, teamID string) (map[string]string, error) {
	var members []localMattermostTeamMember
	path := "/api/v4/teams/" + url.PathEscape(teamID) + "/members"
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &members); errorValue != nil {
		return nil, errorValue
	}
	roleByUserID := map[string]string{}
	for _, member := range members {
		roleByUserID[member.UserID] = member.Roles
	}
	return roleByUserID, nil
}

func (service *Service) localMattermostUserByIDOrEmail(ctx context.Context, token string, idOrEmail string) (mattermostUserRecord, bool, error) {
	decodedValue, errorValue := url.PathUnescape(strings.Trim(idOrEmail, "/"))
	if errorValue != nil {
		return mattermostUserRecord{}, false, errorValue
	}
	if strings.Contains(decodedValue, "@") {
		return service.findMattermostUserByEmail(ctx, token, strings.ToLower(strings.TrimSpace(decodedValue)))
	}
	return service.findMattermostUserByID(ctx, token, strings.TrimSpace(decodedValue))
}

func (service *Service) localIsLastMattermostAdmin(ctx context.Context, token string, userID string) (bool, error) {
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return false, errorValue
	}
	users, errorValue := service.teamMattermostUsers(ctx, token, teamRecord.ID)
	if errorValue != nil {
		return false, errorValue
	}
	roleByUserID, errorValue := service.localMattermostTeamRoles(ctx, token, teamRecord.ID)
	if errorValue != nil {
		return false, errorValue
	}
	adminCount := 0
	isTargetAdmin := false
	for _, userRecord := range users {
		if localMattermostAdminRole(userRecord, roleByUserID[userRecord.ID]) != "admin" {
			continue
		}
		adminCount++
		if userRecord.ID == userID {
			isTargetAdmin = true
		}
	}
	return isTargetAdmin && adminCount <= 1, nil
}

func (service *Service) localIsLastMattermostAdminByEmail(ctx context.Context, email string) (bool, error) {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	userRecord, found, errorValue := service.findMattermostUserByEmail(ctx, token, email)
	if errorValue != nil || !found {
		return false, errorValue
	}
	return service.localIsLastMattermostAdmin(ctx, token, userRecord.ID)
}

func (service *Service) localBlueclawPersonIDByEmail(ctx context.Context, email string) (string, error) {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return "", errorValue
	}
	people, _ := policyDocument["people"].([]any)
	for _, value := range people {
		person, isPerson := value.(map[string]any)
		if !isPerson || !blueclawPersonHasEmail(person, email) {
			continue
		}
		return strings.TrimSpace(mattermostPolicyString(person["personID"])), nil
	}
	return "", nil
}

func (service *Service) demoteLocalBlueclawPersonBeforeRemoval(ctx context.Context, userRecord mattermostUserRecord) error {
	personID, errorValue := service.localBlueclawPersonIDByEmail(ctx, userRecord.Email)
	if errorValue != nil {
		return errorValue
	}
	if personID == "" {
		return nil
	}
	name := firstNonEmpty(userRecord.Nickname, userRecord.DisplayName, userRecord.Username)
	return service.upsertBlueclawPerson(ctx, personID, userRecord.Email, name, "member", []string{"staff"})
}

func (service *Service) applyLocalMattermostTeamRole(ctx context.Context, userID string, isAdmin bool) error {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return errorValue
	}
	body := map[string]bool{"scheme_user": true, "scheme_admin": isAdmin}
	path := "/api/v4/teams/" + url.PathEscape(teamRecord.ID) + "/members/" + url.PathEscape(userID) + "/schemeRoles"
	return service.mattermostRequest(ctx, http.MethodPut, path, token, body, nil)
}

func (service *Service) localSaveBlueclawPerson(ctx context.Context, payload adminUserMutation, hasExplicitCircleMutation bool) error {
	if hasExplicitCircleMutation {
		return service.upsertBlueclawPerson(ctx, payload.UserID, payload.Email, payload.Name, payload.Role, payload.Circles)
	}
	return service.inviteBlueclawPerson(ctx, payload.UserID, payload.Email, payload.Name)
}

func (service *Service) writeLocalUsersResponse(responseWriter http.ResponseWriter, request *http.Request, response pagesUsersResponse) {
	responseBody, errorValue := service.localUsersResponseBody(request.Context(), response)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_, _ = responseWriter.Write(responseBody)
}

func (service *Service) localUsersResponseBody(ctx context.Context, response pagesUsersResponse) ([]byte, error) {
	responseBody, errorValue := json.Marshal(response)
	if errorValue != nil {
		return nil, errorValue
	}
	enhancedBody, errorValue := service.withBlueclawCircles(ctx, responseBody)
	if errorValue != nil {
		log.Printf("Blueclaw circle merge failed: %v", errorValue)
		return responseBody, nil
	}
	return enhancedBody, nil
}

func localUsersBodyWithTemporaryPassword(responseBody []byte, temporaryPassword string, email string) []byte {
	var responseDocument map[string]any
	if json.Unmarshal(responseBody, &responseDocument) != nil {
		return responseBody
	}
	responseDocument["temporaryPassword"] = temporaryPassword
	responseDocument["temporaryPasswordEmail"] = email
	updatedBody, errorValue := json.Marshal(responseDocument)
	if errorValue != nil {
		return responseBody
	}
	return updatedBody
}
