package admind

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func (service *Service) proxyUsers(responseWriter http.ResponseWriter, request *http.Request) {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID == "" || fleetSecret == "" {
		http.Error(responseWriter, "device auth is not configured", http.StatusServiceUnavailable)
		return
	}
	targetPath := strings.TrimPrefix(request.URL.Path, "/admin/api/users")
	targetURL := strings.TrimRight(service.Configuration.APIBaseURL, "/") + "/api/users" + targetPath
	if request.Method == http.MethodGet || request.Method == http.MethodDelete {
		targetURL += "?fleet_id=" + url.QueryEscape(fleetID)
	}
	if request.Method == http.MethodPost || request.Method == http.MethodDelete {
		if errorValue := service.ensureMattermostProvisionerAccount(request.Context()); errorValue != nil {
			log.Printf("Mattermost provisioner sync failed: %v", errorValue)
		}
	}
	var removedUser *adminUserMutation
	var upsertedEmail string
	if request.Method == http.MethodDelete {
		userRecord, errorValue := service.lookupRemovableUser(request.Context(), fleetID, fleetSecret, targetPath)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		removedUser = userRecord
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
	var upsertedNote string
	var upsertedRole string
	var upsertedCircles []string
	var upsertedMattermostUserID string
	var upsertedUserID string
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
		userID, errorValue := service.userIDForAdminUserMutation(request.Context(), fleetID, fleetSecret, payload)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		payload.UserID = userID
		upsertedEmail = payload.Email
		upsertedName = payload.Name
		upsertedNote = payload.Note
		if payload.Role != "admin" && strings.EqualFold(payload.Email, authenticatedCallerEmail(request)) {
			records, errorValue := service.lookupUserRecords(request.Context(), fleetID, fleetSecret)
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
		upsertedUserID = strings.TrimSpace(payload.UserID)
		upsertedRole = payload.Role
		upsertedCircles = append([]string{}, payload.Circles...)
		isLastAdminDemotion, errorValue := service.isLastAdminDemotion(request.Context(), fleetID, fleetSecret, payload.Email, payload.Role)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		if isLastAdminDemotion {
			http.Error(responseWriter, "cannot demote the last admin user", http.StatusBadRequest)
			return
		}
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
		proxyPayload := map[string]any{
			"userID":             payload.UserID,
			"handle":             payload.Handle,
			"name":               payload.Name,
			"hireDate":           payload.HireDate,
			"note":               payload.Note,
			"fleet_id":           fleetID,
			"email":              payload.Email,
			"role":               payload.Role,
			"mattermostUserID":   payload.MattermostUserID,
			"mattermostUsername": payload.MattermostUsername,
			"status":             payload.Status,
		}
		document, errorValue := json.Marshal(proxyPayload)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
		body = io.NopCloser(strings.NewReader(string(document)))
	}
	proxyRequest, errorValue := http.NewRequestWithContext(request.Context(), request.Method, targetURL, body)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	proxyRequest.Header.Set("Content-Type", "application/json")
	proxyRequest.Header.Set("X-InternKim-Fleet-ID", fleetID)
	proxyRequest.Header.Set("X-InternKim-Fleet-Secret", fleetSecret)
	client := service.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, errorValue := client.Do(proxyRequest)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		responseWriter.Header().Set("Content-Type", contentType)
	}
	responseBody, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
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
			var errorValue error
			if hasExplicitCircleMutation {
				errorValue = service.upsertBlueclawPersonWithNote(request.Context(), upsertedUserID, upsertedEmail, upsertedName, upsertedRole, upsertedCircles, upsertedNote)
			} else {
				errorValue = service.inviteBlueclawPerson(request.Context(), upsertedUserID, upsertedEmail, upsertedName)
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
		if request.Method == http.MethodPost && shouldIncludeBlueclawPolicy(request) {
			if enhancedBody, errorValue := service.withBlueclawCircles(request.Context(), responseBody); errorValue == nil {
				responseBody = enhancedBody
			} else {
				log.Printf("Blueclaw circle merge failed: %v", errorValue)
			}
		}
	}
	if request.Method == http.MethodGet && response.StatusCode >= 200 && response.StatusCode < 300 {
		if shouldIncludeBlueclawPolicy(request) {
			if enhancedBody, errorValue := service.withBlueclawCircles(request.Context(), responseBody); errorValue == nil {
				responseBody = enhancedBody
			} else {
				log.Printf("Blueclaw circle merge failed: %v", errorValue)
			}
		}
		var usersResponse pagesUsersResponse
		if errorValue := json.Unmarshal(responseBody, &usersResponse); errorValue == nil && len(usersResponse.Records) > 0 {
			if errorValue := service.ensureMattermostBotDirectChannelsForRecords(request.Context(), usersResponse.Records); errorValue != nil {
				log.Printf("Mattermost bot DM sync failed: %v", errorValue)
			}
		}
	}
	responseWriter.WriteHeader(response.StatusCode)
	_, _ = responseWriter.Write(responseBody)
}
