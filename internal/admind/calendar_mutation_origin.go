package admind

import (
	"context"
	"errors"
	"strings"
)

type calendarMutationOrigin struct {
	ClientID string
	Sequence int64
}

func normalizeCalendarMutationOrigin(clientID *string, sequence *int64) (*calendarMutationOrigin, error) {
	if clientID == nil && sequence == nil {
		return nil, nil
	}
	if clientID == nil || sequence == nil {
		return nil, errors.New("mutationClientID and mutationSequence must be provided together")
	}
	normalizedClientID := strings.TrimSpace(*clientID)
	if normalizedClientID == "" {
		return nil, errors.New("mutationClientID is required")
	}
	if *sequence <= 0 {
		return nil, errors.New("mutationSequence must be a positive integer")
	}
	return &calendarMutationOrigin{ClientID: normalizedClientID, Sequence: *sequence}, nil
}

func replaceCalendarMutationOrigin(ctx context.Context, queryRunner calendarSQLRunner, eventID string, resultingUpdatedAt string, origin *calendarMutationOrigin) error {
	if _, errorValue := queryRunner.ExecContext(ctx, `DELETE FROM calendar_event_mutation_origins WHERE event_id = ?`, strings.TrimSpace(eventID)); errorValue != nil {
		return errorValue
	}
	if origin == nil {
		return nil
	}
	_, errorValue := queryRunner.ExecContext(ctx, `
INSERT INTO calendar_event_mutation_origins(event_id, resulting_updated_at, client_id, sequence)
VALUES (?, ?, ?, ?)`,
		strings.TrimSpace(eventID),
		strings.TrimSpace(resultingUpdatedAt),
		origin.ClientID,
		origin.Sequence,
	)
	return errorValue
}
