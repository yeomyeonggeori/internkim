package admind

import (
	"context"
	"log/slog"
	"time"
)

const organizationUserMutationCompletionTimeout = 5 * time.Second

type organizationUserMutation struct {
	service       *Service
	keys          []organizationPeopleCacheKey
	identityCount int
	isComplete    bool
}

func (service *Service) startOrganizationUserMutation(ctx context.Context, identities []organizationPersonIdentity) (*organizationUserMutation, error) {
	keys, errorValue := service.beginOrganizationUserMutation(ctx, identities)
	if errorValue != nil {
		return nil, errorValue
	}
	return &organizationUserMutation{service: service, keys: keys, identityCount: len(identities)}, nil
}

func (mutation *organizationUserMutation) complete(ctx context.Context) error {
	if mutation == nil || mutation.isComplete {
		return nil
	}
	if errorValue := mutation.service.completeOrganizationUserMutation(ctx, mutation.keys); errorValue != nil {
		return errorValue
	}
	mutation.isComplete = true
	return nil
}

func (mutation *organizationUserMutation) completeAfterSourceMutation(ctx context.Context) {
	if errorValue := mutation.complete(ctx); errorValue != nil {
		slog.Warn(
			"organization user cache mutation completion failed",
			"identity_count", mutation.identityCount,
			"error", errorValue.Error(),
		)
	}
}

func (mutation *organizationUserMutation) completeAfterRequest(ctx context.Context) {
	if mutation == nil || mutation.isComplete {
		return
	}
	completionContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), organizationUserMutationCompletionTimeout)
	defer cancel()
	if errorValue := mutation.complete(completionContext); errorValue != nil {
		slog.WarnContext(
			completionContext,
			"organization user cache mutation completion failed",
			"identity_count", mutation.identityCount,
			"error", errorValue.Error(),
		)
	}
}
