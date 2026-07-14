package admind

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
)

var errLastAdminDemotion = errors.New("cannot demote the last admin user")

type localMattermostTeamMember struct {
	UserID string `json:"user_id"`
	Roles  string `json:"roles"`
}

func (service *Service) localListUsers(responseWriter http.ResponseWriter, request *http.Request) {
	response, errorValue := service.buildLocalUsersResponse(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeLocalUsersResponse(responseWriter, request, response)
}

func (service *Service) buildLocalUsersResponse(ctx context.Context) (pagesUsersResponse, error) {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	users, errorValue := service.teamMattermostUsers(ctx, token, teamRecord.ID)
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	roleByUserID, errorValue := service.localMattermostTeamRoles(ctx, token, teamRecord.ID)
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	return pagesUsersResponse{Records: adminUserRecordsWithProfileImages(localAdminUserRecords(users, roleByUserID))}, nil
}

func (service *Service) localUpsertUser(responseWriter http.ResponseWriter, request *http.Request) {
	payload, hasExplicitCircleMutation, isValid := localAdminUserPayload(responseWriter, request)
	if !isValid {
		return
	}
	identities := []orgchartPersonIdentity{{UserID: payload.UserID, Email: payload.Email}}
	mutation, errorValue := service.startOrgchartUserMutation(request.Context(), identities)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer mutation.completeAfterRequest(request.Context())
	payload, provisionResult, errorValue := service.applyLocalUserMutation(request.Context(), payload, hasExplicitCircleMutation)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), statusForUserMutationError(errorValue))
		return
	}
	if errorValue := mutation.complete(request.Context()); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
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

func (service *Service) localUpsertUsersBatch(responseWriter http.ResponseWriter, request *http.Request) {
	var batchRequest struct {
		Users []adminUserMutation `json:"users"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&batchRequest); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(batchRequest.Users) == 0 {
		http.Error(responseWriter, "users required", http.StatusBadRequest)
		return
	}
	type normalizedBatchUser struct {
		payload                   adminUserMutation
		hasExplicitCircleMutation bool
	}
	normalizedUsers := make([]normalizedBatchUser, 0, len(batchRequest.Users))
	identities := make([]orgchartPersonIdentity, 0, len(batchRequest.Users))
	for _, rawPayload := range batchRequest.Users {
		payload, hasExplicitCircleMutation, errorValue := normalizeAdminUserPayload(rawPayload)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		normalizedUsers = append(normalizedUsers, normalizedBatchUser{payload: payload, hasExplicitCircleMutation: hasExplicitCircleMutation})
		identities = append(identities, orgchartPersonIdentity{UserID: payload.UserID, Email: payload.Email})
	}
	mutation, errorValue := service.startOrgchartUserMutation(request.Context(), identities)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer mutation.completeAfterRequest(request.Context())
	temporaryPassword := ""
	temporaryPasswordEmail := ""
	for _, normalizedUser := range normalizedUsers {
		payload, provisionResult, errorValue := service.applyLocalUserMutation(request.Context(), normalizedUser.payload, normalizedUser.hasExplicitCircleMutation)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), statusForUserMutationError(errorValue))
			return
		}
		if temporaryPassword == "" && provisionResult.TemporaryPassword != "" {
			temporaryPassword = provisionResult.TemporaryPassword
			temporaryPasswordEmail = payload.Email
		}
	}
	if errorValue := mutation.complete(request.Context()); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.triggerUsersSync(request.Context())
	response, errorValue := service.buildLocalUsersResponse(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseBody, errorValue := service.localUsersResponseBody(request.Context(), response)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if temporaryPassword != "" {
		responseBody = localUsersBodyWithTemporaryPassword(responseBody, temporaryPassword, temporaryPasswordEmail)
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_, _ = responseWriter.Write(responseBody)
}

func (service *Service) applyLocalUserMutation(ctx context.Context, payload adminUserMutation, hasExplicitCircleMutation bool) (adminUserMutation, mattermostProvisionResult, error) {
	userID, errorValue := service.localBlueclawPersonIDByEmail(ctx, payload.Email)
	if errorValue != nil {
		return payload, mattermostProvisionResult{}, errorValue
	}
	if strings.TrimSpace(payload.UserID) == "" {
		payload.UserID = firstNonEmpty(userID, newInternKimUserID())
	}
	payload.Circles = normalizeAdminUserCircles(payload.Circles, payload.Role)
	if payload.Role != "admin" {
		isLastAdmin, errorValue := service.localIsLastMattermostAdminByEmail(ctx, payload.Email)
		if errorValue != nil {
			return payload, mattermostProvisionResult{}, errorValue
		}
		if isLastAdmin {
			return payload, mattermostProvisionResult{}, errLastAdminDemotion
		}
	}
	systemSafePayload := payload
	systemSafePayload.Role = "member"
	provisionResult, errorValue := service.provisionMattermostUserWithPassword(ctx, systemSafePayload, "")
	if errorValue != nil {
		return payload, mattermostProvisionResult{}, errorValue
	}
	payload.MattermostUserID = provisionResult.UserID
	payload.MattermostUsername = provisionResult.Username
	payload.Handle = provisionResult.Username
	payload.Status = provisionResult.Status
	if errorValue := service.applyLocalMattermostTeamRole(ctx, provisionResult.UserID, payload.Role == "admin"); errorValue != nil {
		return payload, mattermostProvisionResult{}, errorValue
	}
	if errorValue := service.localSaveBlueclawPerson(ctx, payload, hasExplicitCircleMutation); errorValue != nil {
		return payload, mattermostProvisionResult{}, errorValue
	}
	if hasExplicitCircleMutation {
		if errorValue := service.syncMattermostUserCircleMemberships(ctx, payload); errorValue != nil {
			log.Printf("Mattermost circle membership sync failed: %v", errorValue)
		}
	}
	return payload, provisionResult, nil
}

func statusForUserMutationError(errorValue error) int {
	if errors.Is(errorValue, errLastAdminDemotion) {
		return http.StatusBadRequest
	}
	return http.StatusBadGateway
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
	identities := []orgchartPersonIdentity{{UserID: userRecord.ID, Email: userRecord.Email}}
	mutation, errorValue := service.startOrgchartUserMutation(request.Context(), identities)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer mutation.completeAfterRequest(request.Context())
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
	if errorValue := mutation.complete(request.Context()); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
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
	return service.upsertBlueclawPerson(ctx, personID, userRecord.Email, name, "member", []string{"staff"}, nil)
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
		return service.upsertBlueclawPerson(ctx, payload.UserID, payload.Email, payload.Name, payload.Role, payload.Circles, &payload.Note)
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
	response.Records = adminUserRecordsWithProfileImages(response.Records)
	for index := range response.Records {
		applyDefaultOrgchartMetadata(&response.Records[index])
	}
	responseBody, errorValue := json.Marshal(response)
	if errorValue != nil {
		return nil, errorValue
	}
	enhancedBody, errorValue := service.withBlueclawCircles(ctx, responseBody)
	if errorValue != nil {
		log.Printf("Blueclaw circle merge failed: %v", errorValue)
		enhancedBody = responseBody
	}
	bodyWithCircles := enhancedBody
	enhancedBody, errorValue = service.withOrgchartMetadata(ctx, bodyWithCircles)
	if errorValue != nil {
		log.Printf("Orgchart metadata merge failed: %v", errorValue)
		return bodyWithCircles, nil
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
