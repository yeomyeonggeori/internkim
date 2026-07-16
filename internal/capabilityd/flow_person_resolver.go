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
	return missingFlowOwnerResolution()
}

func missingFlowOwnerResolution() flowOwnerResolution {
	return flowOwnerResolution{Failure: &flowTaskAddFailure{
		ErrorCode:    "flow_owner_not_found",
		FailureStage: "target_resolution",
		Message:      "task owner was not found; ask the user for a name, email, or @handle",
		Retryable:    true,
		SafeRetry:    true,
	}}
}

func ambiguousFlowOwnerResolution(matches []flowMemberForTool) flowOwnerResolution {
	return flowOwnerResolution{Failure: &flowTaskAddFailure{
		ErrorCode:    "flow_owner_ambiguous",
		FailureStage: "target_resolution",
		Message:      "task.add target is ambiguous; ask the user to choose one candidate by @handle",
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
	exactMatches := exactFlowMemberMatches(normalizedValue, members)
	if len(exactMatches) > 0 {
		return uniqueFlowMembers(exactMatches)
	}
	return uniqueFlowMembers(containedFlowMemberMatches(normalizedValue, members))
}

func exactFlowMemberMatches(normalizedValue string, members []flowMemberForTool) []flowMemberForTool {
	normalizedHandleValue := strings.TrimPrefix(normalizedValue, "@")
	matches := make([]flowMemberForTool, 0, len(members))
	for _, member := range members {
		if isExactFlowMemberMatch(normalizedValue, normalizedHandleValue, member) {
			matches = append(matches, member)
		}
	}
	return matches
}

func isExactFlowMemberMatch(normalizedValue string, normalizedHandleValue string, member flowMemberForTool) bool {
	return normalizedFlowMemberValue(member.ID) == normalizedValue ||
		normalizedFlowMemberValue(member.Email) == normalizedValue ||
		normalizedFlowMemberValue(member.Name) == normalizedValue ||
		normalizedFlowMemberValue(member.MattermostUsername) == normalizedHandleValue
}

func containedFlowMemberMatches(normalizedValue string, members []flowMemberForTool) []flowMemberForTool {
	normalizedHandleValue := strings.TrimPrefix(normalizedValue, "@")
	matches := make([]flowMemberForTool, 0, len(members))
	for _, member := range members {
		if flowMemberMatchesContainedValue(normalizedValue, normalizedHandleValue, member) {
			matches = append(matches, member)
		}
	}
	return matches
}

func flowMemberMatchesContainedValue(normalizedValue string, normalizedHandleValue string, member flowMemberForTool) bool {
	for _, value := range []string{member.Email, member.Name, member.MattermostUsername} {
		normalizedMemberValue := normalizedFlowMemberValue(value)
		if normalizedMemberValue == "" {
			continue
		}
		if strings.Contains(normalizedMemberValue, normalizedHandleValue) || strings.Contains(normalizedValue, normalizedMemberValue) {
			return true
		}
	}
	return false
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
