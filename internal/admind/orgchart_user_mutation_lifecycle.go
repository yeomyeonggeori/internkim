package admind

import (
	"context"
	"log/slog"
	"time"
)

const orgchartUserMutationCompletionTimeout = 5 * time.Second

type orgchartUserMutation struct {
	service       *Service
	keys          []orgchartPeopleCacheKey
	identityCount int
	isComplete    bool
}

func (service *Service) startOrgchartUserMutation(ctx context.Context, identities []orgchartPersonIdentity) (*orgchartUserMutation, error) {
	keys, errorValue := service.beginOrgchartUserMutation(ctx, identities)
	if errorValue != nil {
		return nil, errorValue
	}
	return &orgchartUserMutation{service: service, keys: keys, identityCount: len(identities)}, nil
}

func (mutation *orgchartUserMutation) complete(ctx context.Context) error {
	if mutation == nil || mutation.isComplete {
		return nil
	}
	if errorValue := mutation.service.completeOrgchartUserMutation(ctx, mutation.keys); errorValue != nil {
		return errorValue
	}
	mutation.isComplete = true
	return nil
}

func (mutation *orgchartUserMutation) completeAfterSourceMutation(ctx context.Context) {
	if errorValue := mutation.complete(ctx); errorValue != nil {
		slog.Warn(
			"orgchart user cache mutation completion failed",
			"identity_count", mutation.identityCount,
			"error", errorValue.Error(),
		)
	}
}

func (mutation *orgchartUserMutation) completeAfterRequest(ctx context.Context) {
	if mutation == nil || mutation.isComplete {
		return
	}
	completionContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), orgchartUserMutationCompletionTimeout)
	defer cancel()
	if errorValue := mutation.complete(completionContext); errorValue != nil {
		slog.WarnContext(
			completionContext,
			"orgchart user cache mutation completion failed",
			"identity_count", mutation.identityCount,
			"error", errorValue.Error(),
		)
	}
}
