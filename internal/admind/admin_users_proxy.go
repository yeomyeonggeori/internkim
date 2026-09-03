package admind

import (
	"context"
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
	if request.Method == http.MethodDelete && request.URL.Query().Get("purge") == "true" {
		targetURL += "&purge=true"
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
	}
	body := request.Body
	var upsertedName string
	var upsertedHireDate string
	var upsertedNote string
	var upsertedRole string
	var upsertedCircles []string
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
		memberID, errorValue := service.personIDForMutation(request.Context(), payload.Email, payload.Name)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		payload.MemberID = memberID
		upsertedBlueclawUserID = memberID
		upsertedEmail = payload.Email
		upsertedName = payload.Name
		upsertedHireDate = payload.HireDate
		upsertedNote = payload.Note
		if payload.Role != "admin" && strings.EqualFold(payload.Email, service.authenticatedCallerEmail(request)) {
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
		payload.Handle = firstNonEmpty(strings.TrimSpace(payload.Handle), handleFromEmail(payload.Email))
		payload.Status = firstNonEmpty(strings.TrimSpace(payload.Status), "active")
		proxyPayload := fleetAccountUpsertPayload(payload, fleetID)
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
	proxyRequest.Header.Set("X-INTERNKIM-FLEET-ID", fleetID)
	proxyRequest.Header.Set("X-INTERNKIM-FLEET-SECRET", fleetSecret)
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
		if upsertedEmail != "" {
			service.persistOrganizationHireDate(request.Context(), upsertedEmail, upsertedHireDate)
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
			service.triggerUsersSync(request.Context())
			go service.seatAndNameOneMemberInBuzz(context.Background(), upsertedEmail, upsertedName)
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
		if errorValue := json.Unmarshal(responseBody, &usersResponse); errorValue == nil && usersResponse.Records != nil {
			if described, errorValue := service.organizationMetadataUsersResponse(request.Context(), usersResponse); errorValue == nil {
				if enhancedBody, errorValue := json.Marshal(described); errorValue == nil {
					responseBody = enhancedBody
				} else {
					log.Printf("Organization metadata response marshal failed: %v", errorValue)
				}
			} else {
				log.Printf("Organization metadata merge failed: %v", errorValue)
			}
		}
	}
	responseWriter.WriteHeader(response.StatusCode)
	_, _ = responseWriter.Write(responseBody)
}

func fleetAccountUpsertPayload(payload adminUserMutation, fleetID string) map[string]any {
	return map[string]any{
		"handle":             payload.Handle,
		"name":               payload.Name,
		"note":               payload.Note,
		"fleet_id":           fleetID,
		"email":              payload.Email,
		"role":               payload.Role,
		"mattermostUserID":   payload.MattermostUserID,
		"mattermostUsername": payload.MattermostUsername,
		"status":             payload.Status,
	}
}

// A handle is an identifier the company issues, not something a messenger hands
// back. The address already carries one everybody recognises.
func handleFromEmail(email string) string {
	address := strings.ToLower(strings.TrimSpace(email))
	if index := strings.Index(address, "@"); index > 0 {
		return address[:index]
	}
	return address
}
