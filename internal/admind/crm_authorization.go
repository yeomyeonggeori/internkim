package admind

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

var errCRMAuthenticationRequired = errors.New("CRM authentication required")
var errCRMPermissionDenied = errors.New("CRM permission denied")
var errCRMActorIdentityMissing = errors.New("CRM actor has no organization identity")
var errCRMInvalidOwnerAssignment = errors.New("CRM owner assignment is invalid")

type crmActor struct {
	Email     string
	PersonID  string
	CircleIDs map[string]struct{}
	IsAdmin   bool
}

func (service *Service) resolveCRMActor(request *http.Request) (crmActor, error) {
	email := service.webActorEmail(request)
	if email == "" {
		return crmActor{}, errCRMAuthenticationRequired
	}
	if !service.isTaskMemberActor(request.Context(), email) {
		return crmActor{}, errCRMPermissionDenied
	}
	personID := ""
	circleIDs := map[string]struct{}{}
	for _, user := range service.organizationChartUserRecords(request) {
		if !strings.EqualFold(strings.TrimSpace(user.Email), strings.TrimSpace(email)) {
			continue
		}
		personID = strings.TrimSpace(user.MemberID)
		for _, circleID := range user.Circles {
			circleID = strings.TrimSpace(circleID)
			if circleID != "" {
				circleIDs[circleID] = struct{}{}
			}
		}
		break
	}
	return crmActor{
		Email:     email,
		PersonID:  personID,
		CircleIDs: circleIDs,
		IsAdmin:   service.isTaskAdminEmail(request.Context(), email),
	}, nil
}

func (actor crmActor) canEdit(ownerPersonID string, ownerCircleID string) bool {
	if actor.PersonID != "" && actor.PersonID == strings.TrimSpace(ownerPersonID) {
		return true
	}
	_, found := actor.CircleIDs[strings.TrimSpace(ownerCircleID)]
	return strings.TrimSpace(ownerCircleID) != "" && found
}

func (actor crmActor) requireMutationIdentity() error {
	if strings.TrimSpace(actor.PersonID) == "" {
		return errCRMActorIdentityMissing
	}
	return nil
}

func (service *Service) validateCRMAssignment(request *http.Request, actor crmActor, ownerPersonID string, ownerCircleID string) error {
	ownerPersonID = strings.TrimSpace(ownerPersonID)
	ownerCircleID = strings.TrimSpace(ownerCircleID)
	ownerCircleIDs := map[string]struct{}{}
	ownerFound := false
	for _, user := range service.organizationChartUserRecords(request) {
		if strings.TrimSpace(user.MemberID) != ownerPersonID {
			continue
		}
		ownerFound = true
		for _, circleID := range user.Circles {
			circleID = strings.TrimSpace(circleID)
			if circleID != "" {
				ownerCircleIDs[circleID] = struct{}{}
			}
		}
		break
	}
	if !ownerFound {
		return errCRMInvalidOwnerAssignment
	}
	if ownerCircleID != "" {
		if _, found := ownerCircleIDs[ownerCircleID]; !found {
			return errCRMInvalidOwnerAssignment
		}
	}
	if actor.IsAdmin || ownerPersonID == actor.PersonID && ownerCircleID == "" {
		return nil
	}
	if ownerCircleID != "" {
		if _, found := actor.CircleIDs[ownerCircleID]; found {
			return nil
		}
	}
	return errCRMPermissionDenied
}

func (service *Service) authorizeCRMActivity(ctx context.Context, actor crmActor, activity crmActivity) error {
	checked := false
	if activity.AccountID != "" {
		account, errorValue := service.readCRMAccount(ctx, activity.AccountID, false)
		if errorValue != nil {
			return errorValue
		}
		checked = true
		if !actor.canEdit(account.OwnerPersonID, account.OwnerCircleID) {
			return errCRMPermissionDenied
		}
	}
	if activity.ContactID != "" {
		contact, errorValue := service.readCRMContact(ctx, activity.ContactID, false)
		if errorValue != nil {
			return errorValue
		}
		checked = true
		if !actor.canEdit(contact.OwnerPersonID, contact.OwnerCircleID) {
			return errCRMPermissionDenied
		}
	}
	if activity.OpportunityID != "" {
		opportunity, _, errorValue := service.readCRMOpportunity(ctx, activity.OpportunityID, false)
		if errorValue != nil {
			return errorValue
		}
		checked = true
		if !actor.canEdit(opportunity.OwnerPersonID, opportunity.OwnerCircleID) {
			return errCRMPermissionDenied
		}
	}
	if !checked {
		return errCRMPermissionDenied
	}
	return nil
}

func (service *Service) resolveCRMActivityReferences(ctx context.Context, activity crmActivity) (crmActivity, error) {
	if activity.OpportunityID != "" {
		opportunity, _, errorValue := service.readCRMOpportunity(ctx, activity.OpportunityID, false)
		if errorValue != nil {
			return crmActivity{}, errorValue
		}
		activity.AccountID = opportunity.AccountID
		activity.Business = opportunity.Business
		return activity, nil
	}
	if activity.ContactID != "" {
		contact, errorValue := service.readCRMContact(ctx, activity.ContactID, false)
		if errorValue != nil {
			return crmActivity{}, errorValue
		}
		if activity.AccountID != "" && activity.AccountID != contact.AccountID {
			return crmActivity{}, errCRMPermissionDenied
		}
		activity.AccountID = contact.AccountID
	}
	return activity, nil
}
