package capabilityd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type directoryPerson struct {
	MemberID  string            `json:"memberID"`
	Email     string            `json:"email"`
	Name      string            `json:"name"`
	Messenger map[string]string `json:"messenger,omitempty"`
}

func (person directoryPerson) hintIdentifiers() []string {
	identifiers := []string{person.MemberID, person.Email}
	// A handle is an identifier only when it is written as one. Bare, it is a word
	// somebody might have meant as a name.
	for _, account := range person.Messenger {
		if trimmedAccount := strings.TrimSpace(account); trimmedAccount != "" {
			identifiers = append(identifiers, "@"+strings.TrimPrefix(trimmedAccount, "@"))
		}
	}
	return identifiers
}

func (person directoryPerson) hintTitle() string { return person.Name }

func (person directoryPerson) hintNearness(hint string) float64 {
	nearness := typoNearness(normalizedHintValue(hint), normalizedHintValue(person.Name))
	if addressNearness := emailNearness(normalizedHintValue(hint), normalizedHintValue(person.Email)); addressNearness > nearness {
		nearness = addressNearness
	}
	return nearness
}

// Who a name refers to is the company's answer, and the ladder that turns a
// hint into that answer is the same one everywhere. A tool that kept its own
// list could only know the people who had already spoken to this agent.
func (service Service) resolveDirectoryPersonHint(ctx context.Context, personHint string) (hintResolution[directoryPerson], error) {
	people, errorValue := service.directoryPeople(ctx)
	if errorValue != nil {
		return hintResolution[directoryPerson]{}, errorValue
	}
	return resolveHint(personHint, people, nil), nil
}

func (service Service) directoryPeople(ctx context.Context) ([]directoryPerson, error) {
	endpoint := strings.TrimRight(service.Configuration.AdmindBaseURL, "/") + "/admin/api/directory/people"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if errorValue != nil {
		return nil, errorValue
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("the company directory answered %d", response.StatusCode)
	}
	var document struct {
		People []directoryPerson `json:"people"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&document); errorValue != nil {
		return nil, errorValue
	}
	return document.People, nil
}

func directoryPersonNames(people []directoryPerson) []string {
	names := make([]string, 0, len(people))
	for _, person := range people {
		names = append(names, firstNonEmpty(person.Name, person.Email))
	}
	return names
}

func platformDMRecipientsFromDirectoryPeople(people []directoryPerson) []platformDMRecipient {
	recipients := make([]platformDMRecipient, 0, len(people))
	for _, person := range people {
		recipients = append(recipients, platformDMRecipient{
			PersonID:    strings.TrimSpace(person.MemberID),
			DisplayName: strings.TrimSpace(person.Name),
			Emails:      normalizedPlatformDMEmails([]string{person.Email}),
		})
	}
	return recipients
}

// A hint the company cannot place is not a recipient. Answering with the closest
// account this agent already knows would send somebody else's message.
func (service Service) namedDirectoryPerson(ctx context.Context, personHint string) (directoryPerson, platformDMFailure, bool) {
	resolution, errorValue := service.resolveDirectoryPersonHint(ctx, personHint)
	if errorValue != nil {
		return directoryPerson{}, platformDMUnavailableFailure(errorValue), true
	}
	switch resolution.Outcome {
	case hintResolved:
		return resolution.Match, platformDMFailure{}, false
	case hintAmbiguous, hintApproximate:
		message := fmt.Sprintf("recipient %q is ambiguous: %s", personHint, strings.Join(directoryPersonNames(resolution.Candidates), ", "))
		failure := platformDMStaticFailure("recipient_ambiguous", "recipient_resolve", message)
		failure.Candidates = platformDMRecipientsFromDirectoryPeople(resolution.Candidates)
		return directoryPerson{}, failure, true
	default:
		return directoryPerson{}, platformDMRecipientNotFoundFailure(personHint), true
	}
}
