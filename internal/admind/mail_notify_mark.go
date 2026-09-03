package admind

import (
	"context"
)

const mailNotifyMarkFile = "mail-notify.json"

type mailNotifyMarks struct {
	SeenUpTo map[string]uint32 `json:"seenUpTo"`
}

func (service *Service) readMailNotifyMark(ctx context.Context, actorEmail string) (uint32, bool, error) {
	service.mailNotifyMarkMutex.Lock()
	defer service.mailNotifyMarkMutex.Unlock()

	held, errorValue := service.heldMailNotifyMarks(ctx)
	if errorValue != nil {
		return 0, false, errorValue
	}
	seenUpTo, marked := held.SeenUpTo[actorEmail]
	return seenUpTo, marked, nil
}

func (service *Service) writeMailNotifyMark(ctx context.Context, actorEmail string, seenUpTo uint32) error {
	service.mailNotifyMarkMutex.Lock()
	defer service.mailNotifyMarkMutex.Unlock()

	held, errorValue := service.heldMailNotifyMarks(ctx)
	if errorValue != nil {
		return errorValue
	}
	held.SeenUpTo[actorEmail] = seenUpTo
	return service.writeMachineState(mailNotifyMarkFile, held)
}

func (service *Service) heldMailNotifyMarks(ctx context.Context) (mailNotifyMarks, error) {
	var held mailNotifyMarks
	found, errorValue := service.readMachineState(ctx, mailNotifyMarkFile, &held,
		"the mail notifier's marks could not be read, so it adopts what is there now")
	if errorValue != nil || !found {
		return mailNotifyMarks{SeenUpTo: map[string]uint32{}}, errorValue
	}
	if held.SeenUpTo == nil {
		held.SeenUpTo = map[string]uint32{}
	}
	return held, nil
}
