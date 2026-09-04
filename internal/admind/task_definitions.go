package admind

import (
	"context"
	"errors"
)

var errTaskVocabularyUnnamed = errors.New("this device names no company, and the words a task may carry are the company's")
var errTaskVocabularyUnasked = errors.New("this device knows no administrator to ask the company's vocabulary as")

func (service *Service) companyAnswersATaskVocabulary(ctx context.Context) (bool, error) {
	client := service.centralPlane()
	if client == nil {
		return false, errTaskVocabularyUnnamed
	}
	requesterEmail := service.recordLinkActorEmail()
	if requesterEmail == "" {
		return false, errTaskVocabularyUnasked
	}
	labels, errorValue := client.TaskLabels(ctx, requesterEmail)
	if errorValue != nil {
		return false, errorValue
	}
	return len(labels.Businesses) > 0 || len(labels.Types) > 0, nil
}

// The words a task may carry belong to the company, and a device that names
// none has no vocabulary of its own to answer with.
func (service *Service) readTaskDefinitions(ctx context.Context) (taskDefinitions, error) {
	client := service.centralPlane()
	if client == nil {
		return taskDefinitions{}, errTaskVocabularyUnnamed
	}
	requesterEmail := taskActorOf(ctx)
	if requesterEmail == "" {
		requesterEmail = service.recordLinkActorEmail()
	}
	if requesterEmail == "" {
		return taskDefinitions{}, errTaskVocabularyUnnamed
	}
	labels, errorValue := client.TaskLabels(ctx, requesterEmail)
	if errorValue != nil {
		return taskDefinitions{}, errorValue
	}
	return taskDefinitions{
		Categories: labels.Businesses,
		Types:      labels.Types,
		Sizes:      defaultTaskSizeDefinitionsForLocale(service.adminLocale()),
	}, nil
}
