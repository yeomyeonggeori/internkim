package capabilityd

import "strings"

type flowOwnerResolution struct {
	OwnerID string
	Failure *flowTaskAddFailure
}

func resolveFlowOwner(input flowTaskAddInput, requesterEmail string, members []flowMemberForTool) flowOwnerResolution {
	personHint := strings.TrimSpace(input.TargetPersonHint)
	if personHint == "" {
		personHint = requesterEmail
	}
	return resolveFlowOwnerHint(personHint, members)
}

func resolveFlowOwnerHint(personHint string, members []flowMemberForTool) flowOwnerResolution {
	matches := matchingFlowMembers(personHint, members)
	if len(matches) == 1 {
		return flowOwnerResolution{OwnerID: matches[0].ID}
	}
	if len(matches) > 1 {
		return ambiguousFlowOwnerResolution(matches)
	}
	return missingFlowOwnerResolution(members)
}

func resolveFlowParticipantIDs(personHints []string, ownerID string, members []flowMemberForTool) ([]string, *flowTaskAddFailure) {
	participantIDs := []string{strings.TrimSpace(ownerID)}
	for _, personHint := range personHints {
		resolution := resolveFlowOwnerHint(personHint, members)
		if resolution.Failure != nil {
			return nil, flowParticipantFailure(personHint, *resolution.Failure)
		}
		participantIDs = append(participantIDs, resolution.OwnerID)
	}
	return uniqueFlowParticipantIDs(participantIDs), nil
}

func flowParticipantFailure(personHint string, failure flowTaskAddFailure) *flowTaskAddFailure {
	failure.Message = "task participant " + strings.TrimSpace(personHint) + " was not uniquely resolved"
	if failure.ErrorCode == "flow_owner_ambiguous" {
		failure.ErrorCode = "flow_participant_ambiguous"
		return &failure
	}
	failure.ErrorCode = "flow_participant_not_found"
	return &failure
}

func uniqueFlowParticipantIDs(participantIDs []string) []string {
	uniqueIDs := make([]string, 0, len(participantIDs))
	seen := map[string]bool{}
	for _, participantID := range participantIDs {
		participantID = strings.TrimSpace(participantID)
		if participantID == "" || seen[participantID] {
			continue
		}
		seen[participantID] = true
		uniqueIDs = append(uniqueIDs, participantID)
	}
	return uniqueIDs
}

func missingFlowOwnerResolution(members []flowMemberForTool) flowOwnerResolution {
	return flowOwnerResolution{Failure: &flowTaskAddFailure{
		ErrorCode:    "flow_owner_not_found",
		FailureStage: "target_resolution",
		Message:      "task owner was not found; retry with one candidate's exact name, email, or @handle, or ask the user",
		Candidates:   flowTaskAddCandidates(members),
		Retryable:    true,
		SafeRetry:    true,
	}}
}

func ambiguousFlowOwnerResolution(matches []flowMemberForTool) flowOwnerResolution {
	return flowOwnerResolution{Failure: &flowTaskAddFailure{
		ErrorCode:    "flow_owner_ambiguous",
		FailureStage: "target_resolution",
		Message:      "task_add target is ambiguous; ask the user to choose one candidate by @handle",
		Candidates:   flowTaskAddCandidates(matches),
		Retryable:    true,
		SafeRetry:    true,
	}}
}

func matchingFlowMembers(value string, members []flowMemberForTool) []flowMemberForTool {
	normalizedValue := strings.ToLower(strings.TrimSpace(value))
	if normalizedValue == "" {
		return nil
	}
	exactMatches := uniqueFlowMembers(exactFlowMemberMatches(normalizedValue, members))
	if len(exactMatches) > 0 {
		return exactMatches
	}
	return uniqueFlowMembers(nameContainingFlowMemberMatches(normalizedValue, members))
}

func nameContainingFlowMemberMatches(normalizedValue string, members []flowMemberForTool) []flowMemberForTool {
	matches := make([]flowMemberForTool, 0, len(members))
	for _, member := range members {
		memberName := normalizedFlowMemberValue(member.Name)
		if memberName != "" && strings.Contains(memberName, normalizedValue) {
			matches = append(matches, member)
		}
	}
	return matches
}

func exactFlowMemberMatches(normalizedValue string, members []flowMemberForTool) []flowMemberForTool {
	matches := make([]flowMemberForTool, 0, len(members))
	for _, member := range members {
		if isExactFlowMemberMatch(normalizedValue, member) {
			matches = append(matches, member)
		}
	}
	return matches
}

func isExactFlowMemberMatch(normalizedValue string, member flowMemberForTool) bool {
	return normalizedFlowMemberValue(member.ID) == normalizedValue ||
		normalizedFlowMemberValue(member.Email) == normalizedValue ||
		normalizedFlowMemberValue(member.Name) == normalizedValue ||
		isExactFlowMemberHandleMatch(normalizedValue, member.MattermostUsername)
}

func isExactFlowMemberHandleMatch(normalizedValue string, mattermostUsername string) bool {
	if !strings.HasPrefix(normalizedValue, "@") {
		return false
	}
	normalizedHandle := strings.TrimPrefix(normalizedFlowMemberValue(mattermostUsername), "@")
	return normalizedHandle != "" && normalizedHandle == strings.TrimPrefix(normalizedValue, "@")
}

func normalizedFlowMemberValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func uniqueFlowMembers(members []flowMemberForTool) []flowMemberForTool {
	seen := map[string]bool{}
	uniqueMembers := make([]flowMemberForTool, 0, len(members))
	for _, member := range members {
		key := strings.TrimSpace(member.ID)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		uniqueMembers = append(uniqueMembers, member)
	}
	return uniqueMembers
}

func flowTaskAddCandidates(members []flowMemberForTool) []flowTaskAddCandidate {
	candidates := make([]flowTaskAddCandidate, 0, len(members))
	for _, member := range members {
		candidates = append(candidates, flowTaskAddCandidate{
			ID:                 member.ID,
			Name:               member.Name,
			Email:              member.Email,
			MattermostUsername: member.MattermostUsername,
			Mention:            flowTaskAddMention(member),
		})
	}
	return candidates
}

func flowTaskAddMention(member flowMemberForTool) string {
	handle := strings.TrimSpace(member.MattermostUsername)
	if handle == "" {
		return ""
	}
	return "@" + strings.TrimPrefix(handle, "@")
}

func requesterFlowOwnerID(requesterEmail string, members []flowMemberForTool) string {
	resolution := resolveFlowOwnerHint(strings.TrimSpace(requesterEmail), members)
	if resolution.Failure != nil {
		return ""
	}
	return resolution.OwnerID
}
