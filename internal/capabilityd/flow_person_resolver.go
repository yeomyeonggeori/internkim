package capabilityd

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type flowOwnerResolution struct {
	OwnerID string
	Failure *flowTaskAddFailure
}

// A task belongs to whoever was named. Naming nobody makes it the requester's,
// which is the same rule an event follows.
func (service Service) resolveFlowOwner(ctx context.Context, personHints []string, requesterEmail string, members []flowMemberForTool) flowOwnerResolution {
	personHint := ""
	if len(personHints) > 0 {
		personHint = strings.TrimSpace(personHints[0])
	}
	if personHint == "" {
		personHint = requesterEmail
	}
	return service.resolveFlowOwnerHint(ctx, personHint, members)
}

// Who the hint names is the company's answer; which row on the board is theirs
// is this board's own bookkeeping, joined by the address the company gave.
func (service Service) resolveFlowOwnerHint(ctx context.Context, personHint string, members []flowMemberForTool) flowOwnerResolution {
	// A row id this board issued is already exact and names nobody else, so it is
	// read here rather than sent to a directory that never saw it.
	if member, isFound := itemWithHintIdentifier(strings.TrimSpace(personHint), members); isFound {
		return flowOwnerResolution{OwnerID: member.ID}
	}
	resolution, errorValue := service.resolveDirectoryPersonHint(ctx, personHint)
	if errorValue != nil {
		return flowOwnerResolution{Failure: unreachableDirectoryFlowResolution(errorValue)}
	}
	switch resolution.Outcome {
	case hintResolved:
		member, isOnTheBoard := flowMemberWithEmail(members, resolution.Match.Email)
		if !isOnTheBoard {
			return missingFlowOwnerResolution(members)
		}
		return flowOwnerResolution{OwnerID: member.ID}
	case hintAmbiguous:
		return ambiguousFlowOwnerResolution(flowMembersWithEmails(members, resolution.Candidates))
	case hintApproximate:
		return approximateFlowOwnerResolution(flowMembersWithEmails(members, resolution.Candidates))
	default:
		return missingFlowOwnerResolution(members)
	}
}

func unreachableDirectoryFlowResolution(errorValue error) *flowTaskAddFailure {
	return &flowTaskAddFailure{
		ErrorCode:    "flow_owner_directory_unreachable",
		FailureStage: "target_resolution",
		Message:      "the company directory could not be reached, so who this names is not known yet: " + errorValue.Error(),
		Retryable:    true,
		SafeRetry:    true,
	}
}

func flowMemberWithEmail(members []flowMemberForTool, email string) (flowMemberForTool, bool) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return flowMemberForTool{}, false
	}
	for _, member := range members {
		if strings.ToLower(strings.TrimSpace(member.Email)) == normalizedEmail {
			return member, true
		}
	}
	return flowMemberForTool{}, false
}

func flowMembersWithEmails(members []flowMemberForTool, people []directoryPerson) []flowMemberForTool {
	matched := make([]flowMemberForTool, 0, len(people))
	for _, person := range people {
		if member, isOnTheBoard := flowMemberWithEmail(members, person.Email); isOnTheBoard {
			matched = append(matched, member)
			continue
		}
		matched = append(matched, flowMemberForTool{Name: person.Name, Email: person.Email})
	}
	return matched
}

func (service Service) resolveFlowTaskUpdateParticipants(ctx context.Context, input flowTaskUpdateInput, task flowTaskForTool, members []flowMemberForTool) (*[]string, *flowTaskAddFailure) {
	if input.ParticipantPersonHints == nil {
		return nil, nil
	}
	participantIDs, failure := service.resolveFlowParticipantIDs(ctx, *input.ParticipantPersonHints, task.OwnerID, members)
	if failure != nil {
		return nil, failure
	}
	return &participantIDs, nil
}

func (service Service) resolveFlowParticipantIDs(ctx context.Context, personHints []string, ownerID string, members []flowMemberForTool) ([]string, *flowTaskAddFailure) {
	participantIDs := []string{strings.TrimSpace(ownerID)}
	for _, personHint := range personHints {
		resolution := service.resolveFlowOwnerHint(ctx, personHint, members)
		if resolution.Failure != nil {
			return nil, flowParticipantFailure(personHint, *resolution.Failure)
		}
		participantIDs = append(participantIDs, resolution.OwnerID)
	}
	return uniqueFlowParticipantIDs(participantIDs), nil
}

func flowParticipantFailure(personHint string, failure flowTaskAddFailure) *flowTaskAddFailure {
	trimmedHint := strings.TrimSpace(personHint)
	if failure.ErrorCode == "flow_owner_ambiguous" {
		failure.ErrorCode = "flow_participant_ambiguous"
		failure.Message = "participant " + trimmedHint + " matches more than one person; ask the user which one with ask_input, listing the candidates as choices"
		return &failure
	}
	failure.ErrorCode = "flow_participant_not_found"
	failure.Message = "participant " + trimmedHint + " matched nobody; retry with one candidate's exact name, email, or @handle, or ask the user"
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

// A directory is short and complete, so naming everyone in it is an answer.
// A calendar is neither, which is why the ladder itself hands back nothing.
func missingFlowOwnerResolution(members []flowMemberForTool) flowOwnerResolution {
	return flowOwnerResolution{Failure: &flowTaskAddFailure{
		ErrorCode:     "flow_owner_not_found",
		FailureStage:  "target_resolution",
		Message:       "nobody matched that name and nobody came close; retry with one candidate's exact name, email, or @handle, or ask the user",
		Candidates:    flowTaskAddCandidates(members),
		RecoveryHints: retryWithAnExactNameHint(),
		Retryable:     true,
		SafeRetry:     true,
	}}
}

// A name a couple of characters off is a typo rather than somebody else, and
// which person was meant is the user's to say, not this runtime's and not the
// model's.
func approximateFlowOwnerResolution(matches []flowMemberForTool) flowOwnerResolution {
	return flowOwnerResolution{Failure: &flowTaskAddFailure{
		ErrorCode:     "flow_owner_approximate",
		FailureStage:  "target_resolution",
		Message:       "nobody matched that name exactly, and these are the closest; ask the user with ask_input whether they meant one of them, offering none of them as a choice too. Do not choose one yourself",
		Candidates:    flowTaskAddCandidates(matches),
		RecoveryHints: askTheUserToChooseHint(),
		Retryable:     true,
		SafeRetry:     true,
	}}
}

func (member flowMemberForTool) hintIdentifiers() []string {
	identifiers := []string{member.ID, member.Email}
	if handle := strings.TrimSpace(member.MattermostUsername); handle != "" {
		identifiers = append(identifiers, "@"+strings.TrimPrefix(handle, "@"))
	}
	return identifiers
}

func (member flowMemberForTool) hintTitle() string { return member.Name }

// A person is named, addressed, or mentioned, and each of those is mistyped in
// its own way: a name or a handle by a character or two, an address by the
// domain everyone in the company shares.
func (member flowMemberForTool) hintNearness(hint string) float64 {
	nearness := typoNearness(normalizedHintValue(hint), normalizedHintValue(member.Name))
	if addressNearness := emailNearness(normalizedHintValue(hint), normalizedHintValue(member.Email)); addressNearness > nearness {
		nearness = addressNearness
	}
	handleNearness := typoNearness(
		strings.TrimPrefix(normalizedHintValue(hint), "@"),
		strings.TrimPrefix(normalizedHintValue(member.MattermostUsername), "@"),
	)
	if handleNearness > nearness {
		nearness = handleNearness
	}
	return nearness
}

func retryWithAnExactNameHint() []capabilities.RecoveryHint {
	return []capabilities.RecoveryHint{{
		Action:    "retry_with_an_exact_name_from_the_candidates",
		ToolNames: []string{"person_list", "ask_input"},
		Reason:    "the person the user named is still required; dropping them from the call answers a different request than the one that was made",
	}}
}

func ambiguousFlowOwnerResolution(matches []flowMemberForTool) flowOwnerResolution {
	return flowOwnerResolution{Failure: &flowTaskAddFailure{
		ErrorCode:     "flow_owner_ambiguous",
		FailureStage:  "target_resolution",
		Message:       "the name matches more than one person; ask the user which one with ask_input, listing the candidates as choices",
		Candidates:    flowTaskAddCandidates(matches),
		RecoveryHints: askTheUserToChooseHint(),
		Retryable:     true,
		SafeRetry:     true,
	}}
}

func askTheUserToChooseHint() []capabilities.RecoveryHint {
	return []capabilities.RecoveryHint{{
		Action:    "ask_the_user_to_choose_a_candidate",
		ToolNames: []string{"ask_input"},
		Reason:    "only the user can say which person they meant",
	}}
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

func (service Service) requesterFlowOwnerID(ctx context.Context, requesterEmail string, members []flowMemberForTool) string {
	resolution := service.resolveFlowOwnerHint(ctx, strings.TrimSpace(requesterEmail), members)
	if resolution.Failure != nil {
		return ""
	}
	return resolution.OwnerID
}

func flowTaskWriteRefusal(errorValue error, task flowTaskForTool, participantIDs *[]string, members []flowMemberForTool) (flowTaskWriteRefusalFailure, bool) {
	var statusError flowAPIStatusError
	if !errors.As(errorValue, &statusError) || statusError.StatusCode != http.StatusForbidden {
		return flowTaskWriteRefusalFailure{}, false
	}
	if participantIDs == nil {
		return flowTaskWriteRefusalFailure{
			ErrorCode:    "flow_task_write_forbidden",
			FailureStage: "authorization",
			Message:      "the requester may not change this task; only its owner, a participant, or an admin can",
		}, true
	}
	return flowTaskWriteRefusalFailure{
		ErrorCode:    "flow_task_assignment_forbidden",
		FailureStage: "authorization",
		Message: "only " + flowTaskOwnerLabel(task, members) + " or an admin can change who takes part in this task, " +
			"so the requester cannot. Tell the user who has to make this change instead of retrying.",
	}, true
}

func flowTaskOwnerLabel(task flowTaskForTool, members []flowMemberForTool) string {
	if ownerName := strings.TrimSpace(task.OwnerName); ownerName != "" {
		return ownerName
	}
	for _, member := range members {
		if member.ID == strings.TrimSpace(task.OwnerID) {
			return strings.TrimSpace(member.Name)
		}
	}
	return "the task owner"
}
