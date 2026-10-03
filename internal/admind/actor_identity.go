package admind

import (
	"context"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/personname"
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
	return service.resolveUserActorFromUserRecords(ctx, normalizedEmail)
}

func (service *Service) resolveUserActorFromUserRecords(ctx context.Context, email string) (userActor, bool, error) {
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return userActor{}, false, errorValue
	}
	for _, record := range records {
		if strings.EqualFold(record.Email, email) && isActiveTaskUser(record) {
			actor := userActorFromAdminUserRecord(record)
			actor.Name = personname.Render(actor.Name, service.workspaceLanguage(ctx))
			return actor, true, nil
		}
	}
	return userActor{}, false, nil
}

func userActorFromAdminUserRecord(record adminUserMutation) userActor {
	return userActor{
		UserID: strings.TrimSpace(record.MemberID),
		Email:  strings.ToLower(strings.TrimSpace(record.Email)),
		Name:   strings.TrimSpace(record.Name),
		Role:   normalizeAdminUserRole(record.Role),
	}
}
