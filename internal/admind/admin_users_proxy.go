package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

var errBadRequestBody = errors.New("invalid request body")

var errLastAdminDemotion = errors.New("cannot demote the last admin user")

type adminUserWrite struct {
	payload                   adminUserMutation
	hasExplicitCircleMutation bool
}

func (service *Service) proxyUsers(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	var removedUser *adminUserMutation
	if request.Method == http.MethodDelete {
		userRecord, errorValue := service.lookupRemovableUser(ctx, strings.TrimPrefix(request.URL.Path, "/admin/api/users"))
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		removedUser = userRecord
	}
	var write *adminUserWrite
	if request.Method == http.MethodPost {
		asked, statusCode, errorValue := service.adminUserWriteOf(request)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), statusCode)
			return
		}
		write = asked
	}
	response, errorValue := service.answerAdminUsers(ctx, request.Method, write, removedUser)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if write != nil {
		if errorValue := service.carryAdminUserWriteToBlueclaw(ctx, *write); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
	}
	if removedUser != nil {
		if errorValue := service.removeBlueclawPerson(ctx, removedUser.Email); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		service.triggerUsersSync(ctx)
	}
	service.writeDirectoryUsersResponse(responseWriter, request, response)
}

func (service *Service) adminUserWriteOf(request *http.Request) (*adminUserWrite, int, error) {
	var rawPayload adminUserMutation
	if errorValue := json.NewDecoder(request.Body).Decode(&rawPayload); errorValue != nil {
		return nil, http.StatusBadRequest, errBadRequestBody
	}
	payload, hasExplicitCircleMutation, errorValue := normalizeAdminUserPayload(rawPayload)
	if errorValue != nil {
		return nil, http.StatusBadRequest, errorValue
	}
	payload.HireDate = strings.TrimSpace(payload.HireDate)
	memberID, errorValue := service.personIDForMutation(request.Context(), payload.Email, payload.Name)
	if errorValue != nil {
		return nil, http.StatusBadGateway, errorValue
	}
	payload.MemberID = memberID
	if payload.Role != "admin" && strings.EqualFold(payload.Email, service.authenticatedCallerEmail(request)) {
		isAdmin, errorValue := service.isCompanyAdminEmail(request.Context(), payload.Email)
		if errorValue != nil {
			return nil, http.StatusBadGateway, errorValue
		}
		if isAdmin {
			payload.Role = "admin"
		}
	}
	payload.Circles = normalizeAdminUserCircles(payload.Circles, payload.Role)
	isLastAdminDemotion, errorValue := service.isLastAdminDemotion(request.Context(), payload.Email, payload.Role)
	if errorValue != nil {
		return nil, http.StatusBadGateway, errorValue
	}
	if isLastAdminDemotion {
		return nil, http.StatusBadRequest, errLastAdminDemotion
	}
	return &adminUserWrite{payload: payload, hasExplicitCircleMutation: hasExplicitCircleMutation}, http.StatusOK, nil
}

func (service *Service) isCompanyAdminEmail(ctx context.Context, email string) (bool, error) {
	records, errorValue := service.companyUserRecords(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	for _, record := range records {
		if record.Role == "admin" && strings.EqualFold(record.Email, email) {
			return true, nil
		}
	}
	return false, nil
}

// The write goes through the company's member door and the answer is what the
// company then says, so the console shows the directory rather than an echo of
// what it asked for.
func (service *Service) answerAdminUsers(ctx context.Context, method string, write *adminUserWrite, removedUser *adminUserMutation) (pagesUsersResponse, error) {
	switch {
	case method == http.MethodPost && write != nil:
		if errorValue := service.saveCompanyUserRecord(ctx, accountWriteOf(write.payload)); errorValue != nil {
			return pagesUsersResponse{}, errorValue
		}
	case method == http.MethodDelete && removedUser != nil:
		if errorValue := service.withdrawCompanyUser(ctx, removedUser.Email); errorValue != nil {
			return pagesUsersResponse{}, errorValue
		}
	}
	records, errorValue := service.companyUserRecords(ctx)
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	return pagesUsersResponse{Records: records}, nil
}

func accountWriteOf(payload adminUserMutation) centralplane.MemberWrite {
	return centralplane.MemberWrite{
		Email: payload.Email,
		Name:  payload.Name,
		Role:  payload.Role,
		Note:  payload.Note,
	}
}

func (service *Service) carryAdminUserWriteToBlueclaw(ctx context.Context, write adminUserWrite) error {
	payload := write.payload
	service.persistOrganizationHireDate(ctx, payload.Email, payload.HireDate)
	var errorValue error
	if write.hasExplicitCircleMutation {
		errorValue = service.upsertBlueclawPerson(ctx, payload.MemberID, payload.Email, payload.Name, payload.Role, payload.Circles, &payload.Note)
	} else {
		errorValue = service.inviteBlueclawPerson(ctx, payload.MemberID, payload.Email, payload.Name)
	}
	if errorValue != nil {
		return errorValue
	}
	service.triggerUsersSync(ctx)
	inTheBackgroundWithin(buzzDatabaseRequestBudget, func(backgroundContext context.Context) {
		service.seatAndNameOneMemberInBuzz(backgroundContext, payload.Email, payload.Name)
	})
	return nil
}
