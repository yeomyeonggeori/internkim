package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func (service *Service) proxyUsers(responseWriter http.ResponseWriter, request *http.Request) {
	targetPath := strings.TrimPrefix(request.URL.Path, "/admin/api/users")
	if request.Method == http.MethodPost || request.Method == http.MethodDelete {
		if errorValue := service.ensureMattermostProvisionerAccount(request.Context()); errorValue != nil {
			log.Printf("Mattermost provisioner sync failed: %v", errorValue)
		}
	}
	var removedUser *adminUserMutation
	var accountWrite *centralplane.MemberWrite
	var upsertedEmail string
	var organizationMutationIdentities []organizationPersonIdentity
	var organizationMutation *organizationUserMutation
	if request.Method == http.MethodDelete {
		userRecord, errorValue := service.lookupRemovableUser(request.Context(), targetPath)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		removedUser = userRecord
		if removedUser != nil {
			identity, errorValue := service.resolveLocalOrganizationRemovalIdentity(request.Context(), removedUser.Email, removedUser.MemberID)
			if errorValue != nil {
				http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
				return
			}
			organizationMutationIdentities = organizationProxyMutationIdentities(removedUser.MemberID, identity)
			organizationMutation, errorValue = service.startOrganizationUserMutation(request.Context(), organizationMutationIdentities)
			if errorValue != nil {
				http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
				return
			}
			defer organizationMutation.completeAfterRequest(request.Context())
		}
		if removedUser != nil && strings.TrimSpace(removedUser.MattermostUserID) != "" {
			if errorValue := service.deactivateMattermostUserByID(request.Context(), removedUser.MattermostUserID); errorValue != nil {
				http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
				return
			}
		} else if removedUser != nil {
			if errorValue := service.deactivateMattermostUserByEmail(request.Context(), removedUser.Email); errorValue != nil {
				http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
				return
			}
		}
	}
	body := request.Body
	var temporaryPassword string
	var temporaryPasswordEmail string
	var upsertedName string
	var upsertedHireDate string
	var upsertedNote string
	var upsertedRole string
	var upsertedCircles []string
	var upsertedMattermostUserID string
	var upsertedUserID string
	var upsertedBlueclawUserID string
	var hasExplicitCircleMutation bool
	if request.Method == http.MethodPost {
		var rawPayload adminUserMutation
		if errorValue := json.NewDecoder(request.Body).Decode(&rawPayload); errorValue != nil {
			http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
			return
		}
		payload, explicitCircleMutation, errorValue := normalizeAdminUserPayload(rawPayload)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		hasExplicitCircleMutation = explicitCircleMutation
		payload.HireDate = strings.TrimSpace(payload.HireDate)
		claimedMemberID := payload.MemberID
		payload, identity, errorValue := service.resolveLocalOrganizationMutationIdentity(request.Context(), payload)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		upsertedBlueclawUserID = identity.MemberID
		upsertedEmail = payload.Email
		upsertedName = payload.Name
		upsertedHireDate = payload.HireDate
		upsertedNote = payload.Note
		if payload.Role != "admin" && strings.EqualFold(payload.Email, service.authenticatedCallerEmail(request)) {
			records, errorValue := service.companyUserRecords(request.Context())
			if errorValue != nil {
				http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
				return
			}
			for _, record := range records {
				if record.Role == "admin" && strings.EqualFold(record.Email, payload.Email) {
					payload.Role = "admin"
					break
				}
			}
		}
		payload.Circles = normalizeAdminUserCircles(payload.Circles, payload.Role)
		upsertedUserID = strings.TrimSpace(payload.MemberID)
		upsertedRole = payload.Role
		upsertedCircles = append([]string{}, payload.Circles...)
		isLastAdminDemotion, errorValue := service.isLastAdminDemotion(request.Context(), payload.Email, payload.Role)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		if isLastAdminDemotion {
			http.Error(responseWriter, "cannot demote the last admin user", http.StatusBadRequest)
			return
		}
		organizationMutationIdentities = organizationProxyMutationIdentities(claimedMemberID, identity)
		organizationMutation, errorValue = service.startOrganizationUserMutation(request.Context(), organizationMutationIdentities)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
		defer organizationMutation.completeAfterRequest(request.Context())
		provisionResult, errorValue := service.provisionMattermostUserWithPassword(request.Context(), payload, "")
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		payload.MattermostUserID = provisionResult.UserID
		payload.MattermostUsername = provisionResult.Username
		payload.Handle = provisionResult.Username
		payload.Status = provisionResult.Status
		upsertedMattermostUserID = payload.MattermostUserID
		temporaryPassword = provisionResult.TemporaryPassword
		temporaryPasswordEmail = payload.Email
		accountWrite = &centralplane.MemberWrite{
			Email:     payload.Email,
			Name:      payload.Name,
			Role:      payload.Role,
			Note:      payload.Note,
			Messenger: messengerAccountsOfPayload(payload),
		}
	}
	_ = body
	responseBody, statusCode, errorValue := service.answerAdminUsers(request.Context(), request.Method, targetPath, accountWrite)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	if statusCode >= 200 && statusCode < 300 {
		if request.Method == http.MethodPost {
			upsertedUserID = firstNonEmpty(upsertedUserID, userIDFromAdminUsersResponse(responseBody, upsertedEmail))
		}
		if request.Method == http.MethodPost && temporaryPassword != "" {
			var responseDocument map[string]any
			if errorValue := json.Unmarshal(responseBody, &responseDocument); errorValue == nil {
				responseDocument["temporaryPassword"] = temporaryPassword
				responseDocument["temporaryPasswordEmail"] = temporaryPasswordEmail
				responseBody, _ = json.Marshal(responseDocument)
			}
		}
		if upsertedEmail != "" {
			service.persistOrganizationHireDate(request.Context(), upsertedUserID, upsertedEmail, upsertedHireDate)
			var errorValue error
			if hasExplicitCircleMutation {
				errorValue = service.upsertBlueclawPerson(request.Context(), upsertedBlueclawUserID, upsertedEmail, upsertedName, upsertedRole, upsertedCircles, &upsertedNote)
			} else {
				errorValue = service.inviteBlueclawPerson(request.Context(), upsertedBlueclawUserID, upsertedEmail, upsertedName)
			}
			if errorValue != nil {
				http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
				return
			}
			if hasExplicitCircleMutation {
				if errorValue := service.syncMattermostUserCircleMemberships(request.Context(), adminUserMutation{
					Email:            upsertedEmail,
					Name:             upsertedName,
					Role:             upsertedRole,
					Circles:          upsertedCircles,
					MattermostUserID: upsertedMattermostUserID,
				}); errorValue != nil {
					log.Printf("Mattermost circle membership sync failed: %v", errorValue)
				}
			}
			service.triggerUsersSync(request.Context())
		}
		if request.Method == http.MethodDelete && removedUser != nil {
			if errorValue := service.removeBlueclawPerson(request.Context(), removedUser.Email); errorValue != nil {
				http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
				return
			}
			service.triggerUsersSync(request.Context())
		}
		if len(organizationMutationIdentities) > 0 {
			organizationMutation.completeAfterSourceMutation(request.Context())
		}
		if request.Method == http.MethodPost && shouldIncludeBlueclawPolicy(request) {
			if enhancedBody, errorValue := service.withBlueclawCircles(request.Context(), responseBody); errorValue == nil {
				responseBody = enhancedBody
			} else {
				log.Printf("Blueclaw circle merge failed: %v", errorValue)
			}
		}
	}
	if request.Method == http.MethodGet && statusCode >= 200 && statusCode < 300 {
		if shouldIncludeBlueclawPolicy(request) {
			if enhancedBody, errorValue := service.withBlueclawCircles(request.Context(), responseBody); errorValue == nil {
				responseBody = enhancedBody
			} else {
				log.Printf("Blueclaw circle merge failed: %v", errorValue)
			}
		}
		var usersResponse pagesUsersResponse
		if errorValue := json.Unmarshal(responseBody, &usersResponse); errorValue == nil && usersResponse.Records != nil {
			if metadataResponse, errorValue := service.organizationMetadataUsersResponse(request.Context(), usersResponse); errorValue == nil {
				usersResponse = metadataResponse.response
				usersResponse.Records = adminUserRecordsWithProfileImages(usersResponse.Records)
				if enhancedBody, errorValue := json.Marshal(usersResponse); errorValue == nil {
					responseBody = enhancedBody
				} else {
					log.Printf("Organization metadata response marshal failed: %v", errorValue)
				}
			} else {
				log.Printf("Organization metadata merge failed: %v", errorValue)
			}
			if len(usersResponse.Records) > 0 {
				if errorValue := service.ensureMattermostBotDirectChannelsForRecords(request.Context(), usersResponse.Records); errorValue != nil {
					log.Printf("Mattermost bot DM sync failed: %v", errorValue)
				}
			}
		}
	}
	if statusCode >= 200 && statusCode < 300 {
		responseBody = usersResponseBodyWithProfileImages(responseBody)
	}
	responseWriter.WriteHeader(statusCode)
	_, _ = responseWriter.Write(responseBody)
}

func (service *Service) answerAdminUsers(ctx context.Context, method string, targetPath string, write *centralplane.MemberWrite) ([]byte, int, error) {
	switch method {
	case http.MethodPost:
		if write == nil {
			return nil, 0, fmt.Errorf("a user write needs a payload")
		}
		if errorValue := service.saveCompanyUserRecord(ctx, *write); errorValue != nil {
			return nil, 0, errorValue
		}
	case http.MethodDelete:
		email := strings.TrimPrefix(targetPath, "/")
		if decodedEmail, errorValue := url.PathUnescape(email); errorValue == nil {
			email = decodedEmail
		}
		if errorValue := service.withdrawCompanyUser(ctx, email); errorValue != nil {
			return nil, 0, errorValue
		}
	case http.MethodGet:
	default:
		return nil, http.StatusMethodNotAllowed, nil
	}
	records, errorValue := service.companyUserRecords(ctx)
	if errorValue != nil {
		return nil, 0, errorValue
	}
	document, errorValue := json.Marshal(pagesUsersResponse{Records: records})
	if errorValue != nil {
		return nil, 0, errorValue
	}
	return document, http.StatusOK, nil
}

func messengerAccountsOfPayload(payload adminUserMutation) map[string]string {
	accounts := map[string]string{}
	if account := strings.TrimSpace(payload.MattermostUserID); account != "" {
		accounts["mattermost"] = account
	}
	if account := strings.TrimSpace(payload.MattermostUsername); account != "" {
		accounts["mattermostUsername"] = account
	}
	return accounts
}
