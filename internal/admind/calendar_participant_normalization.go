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

func calendarParticipantPrimaryKey(participant calendarParticipant) string {
	return calendarParticipantIdentityPrimaryKey(calendarParticipantIdentity{
		PersonID: participant.PersonID,
		Name:     participant.Name,
		Email:    participant.Email,
	})
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

func calendarParticipantsIncludeParticipant(participants []calendarParticipant, candidate calendarParticipant) bool {
	normalizedCandidates := normalizeCalendarParticipants([]calendarParticipant{candidate})
	if len(normalizedCandidates) == 0 {
		return true
	}
	normalizedCandidate := normalizedCandidates[0]
	for _, participant := range normalizeCalendarParticipants(participants) {
		if calendarParticipantsSamePerson(participant, normalizedCandidate) {
			return true
		}
	}
	return false
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

func calendarParticipantsEqual(left []calendarParticipant, right []calendarParticipant) bool {
	normalizedLeft := normalizeCalendarParticipants(left)
	normalizedRight := normalizeCalendarParticipants(right)
	if len(normalizedLeft) != len(normalizedRight) {
		return false
	}
	for index, leftParticipant := range normalizedLeft {
		rightParticipant := normalizedRight[index]
		if leftParticipant.PersonID != rightParticipant.PersonID || leftParticipant.Name != rightParticipant.Name || leftParticipant.Email != rightParticipant.Email {
			return false
		}
	}
	return true
}

func calendarParticipantsSamePerson(left calendarParticipant, right calendarParticipant) bool {
	return calendarParticipantIdentitiesSamePerson(
		calendarParticipantIdentity{PersonID: left.PersonID, Name: left.Name, Email: left.Email},
		calendarParticipantIdentity{PersonID: right.PersonID, Name: right.Name, Email: right.Email},
	)
}

func calendarParticipantIdentitiesSamePerson(left calendarParticipantIdentity, right calendarParticipantIdentity) bool {
	if left.PersonID != "" && right.PersonID != "" {
		return strings.EqualFold(left.PersonID, right.PersonID)
	}
	if left.Email != "" && right.Email != "" {
		return strings.EqualFold(left.Email, right.Email)
	}
	if left.PersonID == "" && right.PersonID == "" && left.Email == "" && right.Email == "" && left.Name != "" && right.Name != "" {
		return strings.EqualFold(left.Name, right.Name)
	}
	return false
}
