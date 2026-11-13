package capabilityd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/personname"
)

type directoryPerson struct {
	MemberID  string            `json:"memberID"`
	Email     string            `json:"email"`
	Name      string            `json:"name"`
	JobTitle  string            `json:"jobTitle,omitempty"`
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

func (person directoryPerson) hintRank(hint string) int {
	return int(personname.ReachOf(hint, person.named()))
}

func (person directoryPerson) hintNearness(hint string) float64 {
	asked := normalizedHintValue(hint)
	nearness := max(
		typoNearness(asked, normalizedHintValue(person.Name)),
		emailNearness(asked, normalizedHintValue(person.Email)),
		personname.ReferenceNearness(hint, person.named()),
	)
	for _, account := range person.Messenger {
		nearness = max(nearness, typoNearness(strings.TrimPrefix(asked, "@"), normalizedHintValue(strings.TrimPrefix(strings.TrimSpace(account), "@"))))
	}
	return nearness
}

func (person directoryPerson) named() personname.Person {
	return personname.Person{Name: person.Name, JobTitle: person.JobTitle}
}

func (service Service) resolvePersonHint(ctx context.Context, personHint string, responseLanguage string) (directoryPerson, platformDMFailure, bool) {
	directory, failure, isRefused := service.companyDirectory(ctx)
	if isRefused {
		return directoryPerson{}, failure, true
	}
	return personNamedBy(personHint, directory, responseLanguage)
}

func (service Service) resolvePersonHints(ctx context.Context, personHints []string, responseLanguage string) ([]directoryPerson, platformDMFailure, bool) {
	directory, failure, isRefused := service.companyDirectory(ctx)
	if isRefused {
		return nil, failure, true
	}
	return peopleNamedBy(personHints, directory, responseLanguage)
}

func peopleNamedBy(personHints []string, directory []directoryPerson, responseLanguage string) ([]directoryPerson, platformDMFailure, bool) {
	people := []directoryPerson{}
	for _, personHint := range personHints {
		person, failure, isRefused := personNamedBy(personHint, directory, responseLanguage)
		if isRefused {
			return nil, failure, true
		}
		if !slices.ContainsFunc(people, func(existing directoryPerson) bool { return existing.MemberID == person.MemberID }) {
			people = append(people, person)
		}
	}
	return people, platformDMFailure{}, false
}

func (service Service) companyDirectory(ctx context.Context) ([]directoryPerson, platformDMFailure, bool) {
	directory, errorValue := service.directoryPeople(ctx)
	if errorValue != nil {
		return nil, platformDMUnavailableFailure(errorValue), true
	}
	return directory, platformDMFailure{}, false
}

func personNamedBy(personHint string, directory []directoryPerson, responseLanguage string) (directoryPerson, platformDMFailure, bool) {
	resolution := resolveHint(personHint, directory, nil)
	switch resolution.Outcome {
	case hintResolved:
		return resolution.Match, platformDMFailure{}, false
	case hintAmbiguous:
		return directoryPerson{}, personQuestion(resolution, responseLanguage, fmt.Sprintf(
			"%q matches more than one person, listed in candidates. Ask which one is meant, naming each by name and address; then pass the chosen candidate's personID.", personHint)), true
	case hintApproximate:
		return directoryPerson{}, personQuestion(resolution, responseLanguage, fmt.Sprintf(
			"no one is called %q; the nearest are listed in candidates. Ask whether one of them was meant, naming each by name and address and offering \"none of these\" as a choice; then pass the chosen candidate's personID.", personHint)), true
	default:
		return directoryPerson{}, platformDMRecipientNotFoundFailure(personHint), true
	}
}

func personQuestion(resolution hintResolution[directoryPerson], responseLanguage string, message string) platformDMFailure {
	failure := platformDMStaticFailure("interaction_required", "target_resolution", message)
	failure.Retryable = true
	failure.SafeRetry = true
	failure.Candidates = personCandidates(resolution.Candidates, responseLanguage)
	return failure
}

func personCandidates(people []directoryPerson, responseLanguage string) []personShown {
	candidates := make([]personShown, 0, len(people))
	for _, person := range people {
		candidate := shownPerson(person, responseLanguage)
		candidate.Emails = normalizedPlatformDMEmails([]string{person.Email})
		candidates = append(candidates, candidate)
	}
	return candidates
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
