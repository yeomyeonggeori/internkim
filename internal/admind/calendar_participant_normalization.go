package admind

import "strings"

func calendarParticipantsFromPeople(people []string) []calendarParticipant {
	participants := make([]calendarParticipantIdentity, 0, len(people))
	for _, person := range people {
		participants = append(participants, calendarParticipantIdentity{Name: person})
	}
	return calendarParticipantsFromIdentities(participants)
}

func normalizeCalendarParticipants(values []calendarParticipant) []calendarParticipant {
	return calendarParticipantsFromIdentities(calendarParticipantIdentities(values))
}

func normalizeCalendarParticipantIdentities(values []calendarParticipantIdentity) []calendarParticipantIdentity {
	participants := make([]calendarParticipantIdentity, 0, len(values))
	seenParticipants := map[string]bool{}
	for _, value := range values {
		participant := calendarParticipantIdentity{
			PersonID: strings.TrimSpace(value.PersonID),
			Name:     strings.TrimSpace(value.Name),
			Email:    strings.ToLower(strings.TrimSpace(value.Email)),
		}
		if participant.Name == "" {
			participant.Name = strings.TrimSuffix(participant.Email, "@"+emailDomain(participant.Email))
		}
		key := calendarParticipantIdentityPrimaryKey(participant)
		if key == "" || seenParticipants[key] {
			continue
		}
		seenParticipants[key] = true
		participants = append(participants, participant)
	}
	return participants
}

func calendarParticipantIdentities(participants []calendarParticipant) []calendarParticipantIdentity {
	identities := make([]calendarParticipantIdentity, 0, len(participants))
	for _, participant := range participants {
		identities = append(identities, calendarParticipantIdentity{
			PersonID: participant.PersonID,
			Name:     participant.Name,
			Email:    participant.Email,
		})
	}
	return normalizeCalendarParticipantIdentities(identities)
}

func calendarParticipantsFromIdentities(identities []calendarParticipantIdentity) []calendarParticipant {
	normalizedIdentities := normalizeCalendarParticipantIdentities(identities)
	participants := make([]calendarParticipant, 0, len(normalizedIdentities))
	for _, identity := range normalizedIdentities {
		participants = append(participants, calendarParticipant{
			PersonID: identity.PersonID,
			Name:     identity.Name,
			Email:    identity.Email,
		})
	}
	return participants
}

func calendarParticipantIdentityPrimaryKey(participant calendarParticipantIdentity) string {
	if participant.PersonID != "" {
		return "id:" + strings.ToLower(participant.PersonID)
	}
	if participant.Email != "" {
		return "email:" + participant.Email
	}
	if participant.Name != "" {
		return "name:" + strings.ToLower(participant.Name)
	}
	return ""
}

func calendarParticipantNames(participants []calendarParticipant) []string {
	names := make([]string, 0, len(participants))
	for _, participant := range normalizeCalendarParticipants(participants) {
		if participant.Name != "" {
			names = append(names, participant.Name)
		}
	}
	return names
}
