package admind

import (
	"context"
)

// The words a task may carry belong to the company, and a device keeps its own
// list only while it names none. This is the gate the board reads through.
func (service *Service) companyTaskDefinitions(ctx context.Context) (taskDefinitions, bool, error) {
	client := service.centralPlane()
	if client == nil {
		return taskDefinitions{}, false, nil
	}
	requesterEmail := taskActorOf(ctx)
	if requesterEmail == "" {
		requesterEmail = service.recordLinkActorEmail()
	}
	if requesterEmail == "" {
		return taskDefinitions{}, false, nil
	}
	labels, errorValue := client.TaskLabels(ctx, requesterEmail)
	if errorValue != nil {
		return taskDefinitions{}, true, errorValue
	}
	return taskDefinitions{
		Categories: labels.Businesses,
		Types:      labels.Types,
		Sizes:      defaultTaskSizeDefinitionsForLocale(service.adminLocale()),
	}, true, nil
}
