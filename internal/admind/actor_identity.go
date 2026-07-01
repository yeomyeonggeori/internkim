package admind

import (
	"context"
	"strings"
)

type userActor struct {
	UserID string
	Email  string
	Name   string
	Role   string
}

func (actor userActor) isAdmin() bool {
	return actor.Role == adminUserRoleAdmin
}

func (service *Service) resolveUserActorByEmail(ctx context.Context, email string) (userActor, bool, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return userActor{}, false, nil
	}
	if service.hasDeviceAuth() {
		actor, found, errorValue := service.resolveUserActorFromUserRecords(ctx, normalizedEmail)
		return actor, found, errorValue
	}
	personID, errorValue := service.resolveMemoryPersonIDFromPolicy(ctx, normalizedEmail)
	if errorValue != nil || personID == "" {
		return userActor{}, false, errorValue
	}
	return userActor{UserID: personID, Email: normalizedEmail, Name: normalizedEmail}, true, nil
}

func (service *Service) resolveUserActorFromUserRecords(ctx context.Context, email string) (userActor, bool, error) {
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return userActor{}, false, errorValue
	}
	for _, record := range records {
		if strings.EqualFold(record.Email, email) && isActiveFlowUser(record) {
			return userActorFromAdminUserRecord(record), true, nil
		}
	}
	return userActor{}, false, nil
}

func userActorFromAdminUserRecord(record adminUserMutation) userActor {
	return userActor{
		UserID: strings.TrimSpace(record.UserID),
		Email:  strings.ToLower(strings.TrimSpace(record.Email)),
		Name:   strings.TrimSpace(record.Name),
		Role:   normalizeAdminUserRole(record.Role),
	}
}
