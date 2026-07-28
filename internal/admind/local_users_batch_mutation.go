package admind

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const localUsersBatchMaximumRequestBytes int64 = 1 << 20
const localUsersBatchMaximumUsers = 50

type localUsersBatchRequest struct {
	Users []adminUserMutation `json:"users"`
}

type normalizedLocalBatchUser struct {
	payload                   adminUserMutation
	hasExplicitCircleMutation bool
}

func (service *Service) localUpsertUsersBatch(responseWriter http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, localUsersBatchMaximumRequestBytes)
	var batchRequest localUsersBatchRequest
	decoder := json.NewDecoder(request.Body)
	if errorValue := decoder.Decode(&batchRequest); errorValue != nil {
		writeLocalUsersBatchDecodeError(responseWriter, errorValue)
		return
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		writeLocalUsersBatchDecodeError(responseWriter, errorValue)
		return
	}
	if errorValue := validateAdminUserBatchSize(batchRequest.Users); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	normalizedUsers, errorValue := normalizeLocalBatchUsers(batchRequest.Users)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	resolvedUsers, identities, errorValue := service.resolveLocalBatchUserIdentities(request.Context(), normalizedUsers)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	mutation, errorValue := service.startOrganizationUserMutation(request.Context(), identities)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer mutation.completeAfterRequest(request.Context())
	temporaryPassword, temporaryPasswordEmail, errorValue := service.applyNormalizedLocalBatchUsers(request.Context(), resolvedUsers)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), statusForUserMutationError(errorValue))
		return
	}
	mutation.completeAfterSourceMutation(request.Context())
	service.triggerUsersSync(request.Context())
	service.writeLocalBatchUsersResponse(responseWriter, request, temporaryPassword, temporaryPasswordEmail)
}

func writeLocalUsersBatchDecodeError(responseWriter http.ResponseWriter, errorValue error) {
	var maximumBytesError *http.MaxBytesError
	if errors.As(errorValue, &maximumBytesError) {
		http.Error(responseWriter, fmt.Sprintf("request body exceeds maximum size of %d bytes", maximumBytesError.Limit), http.StatusBadRequest)
		return
	}
	http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
}

func normalizeLocalBatchUsers(users []adminUserMutation) ([]normalizedLocalBatchUser, error) {
	normalizedUsers := make([]normalizedLocalBatchUser, 0, len(users))
	normalizedEmails := make([]string, 0, len(users))
	for _, rawPayload := range users {
		payload, hasExplicitCircleMutation, errorValue := normalizeAdminUserPayload(rawPayload)
		if errorValue != nil {
			return nil, errorValue
		}
		normalizedUsers = append(normalizedUsers, normalizedLocalBatchUser{payload: payload, hasExplicitCircleMutation: hasExplicitCircleMutation})
		normalizedEmails = append(normalizedEmails, payload.Email)
	}
	if errorValue := validateUniqueAdminUserEmails(normalizedEmails); errorValue != nil {
		return nil, errorValue
	}
	return normalizedUsers, nil
}

func (service *Service) resolveLocalBatchUserIdentities(ctx context.Context, users []normalizedLocalBatchUser) ([]normalizedLocalBatchUser, []organizationPersonIdentity, error) {
	resolvedUsers := append([]normalizedLocalBatchUser(nil), users...)
	identities := make([]organizationPersonIdentity, 0, len(users))
	for index, normalizedUser := range users {
		payload, identity, errorValue := service.resolveLocalOrganizationMutationIdentity(ctx, normalizedUser.payload)
		if errorValue != nil {
			return nil, nil, errorValue
		}
		resolvedUsers[index].payload = payload
		identities = append(identities, identity)
	}
	return resolvedUsers, identities, nil
}

func (service *Service) applyNormalizedLocalBatchUsers(ctx context.Context, users []normalizedLocalBatchUser) (string, string, error) {
	temporaryPassword := ""
	temporaryPasswordEmail := ""
	for _, user := range users {
		payload, provisionResult, errorValue := service.applyLocalUserMutation(ctx, user.payload, user.hasExplicitCircleMutation)
		if errorValue != nil {
			return "", "", errorValue
		}
		if temporaryPassword == "" && provisionResult.TemporaryPassword != "" {
			temporaryPassword = provisionResult.TemporaryPassword
			temporaryPasswordEmail = payload.Email
		}
	}
	return temporaryPassword, temporaryPasswordEmail, nil
}

func (service *Service) writeLocalBatchUsersResponse(responseWriter http.ResponseWriter, request *http.Request, temporaryPassword string, temporaryPasswordEmail string) {
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
