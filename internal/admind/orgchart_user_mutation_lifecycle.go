package admind

import (
	"context"
	"log"
	"time"
)

const orgchartUserMutationCompletionTimeout = 5 * time.Second

type orgchartUserMutation struct {
	service    *Service
	identities []orgchartPersonIdentity
	isComplete bool
}

func (service *Service) startOrgchartUserMutation(ctx context.Context, identities []orgchartPersonIdentity) (*orgchartUserMutation, error) {
	if errorValue := service.beginOrgchartUserMutation(ctx, identities); errorValue != nil {
		return nil, errorValue
	}
	return &orgchartUserMutation{service: service, identities: identities}, nil
}

func (mutation *orgchartUserMutation) complete(ctx context.Context) error {
	if mutation == nil || mutation.isComplete {
		return nil
	}
	if errorValue := mutation.service.completeOrgchartUserMutation(ctx, mutation.identities); errorValue != nil {
		return errorValue
	}
	mutation.isComplete = true
	return nil
}

func (mutation *orgchartUserMutation) completeAfterRequest(ctx context.Context) {
	if mutation == nil || mutation.isComplete {
		return
	}
	completionContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), orgchartUserMutationCompletionTimeout)
	defer cancel()
	if errorValue := mutation.complete(completionContext); errorValue != nil {
		log.Printf("Orgchart user cache mutation completion failed: %v", errorValue)
	}
}
