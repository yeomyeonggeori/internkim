package capabilityd

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type taskOwnerResolution struct {
	OwnerID string
	Failure *taskAddFailure
}

// A task belongs to whoever was named. Naming nobody makes it the requester's,
// which is the same rule an event follows.
func (service Service) resolveTaskOwner(ctx context.Context, personHints []string, requesterEmail string, members []taskMemberForTool) taskOwnerResolution {
	personHint := ""
	if len(personHints) > 0 {
		personHint = strings.TrimSpace(personHints[0])
	}
	if personHint == "" {
		personHint = requesterEmail
	}
	return service.resolveTaskOwnerHint(ctx, personHint, members)
}

// Who the hint names is the company's answer; which row on the board is theirs
// is this board's own bookkeeping, joined by the address the company gave.
func (service Service) resolveTaskOwnerHint(ctx context.Context, personHint string, members []taskMemberForTool) taskOwnerResolution {
	// A row id this board issued is already exact and names nobody else, so it is
	// read here rather than sent to a directory that never saw it.
	if member, isFound := itemWithHintIdentifier(strings.TrimSpace(personHint), members); isFound {
		return taskOwnerResolution{OwnerID: member.ID}
	}
	resolution, errorValue := service.resolveDirectoryPersonHint(ctx, personHint)
	if errorValue != nil {
		return taskOwnerResolution{Failure: unreachableDirectoryTaskResolution(errorValue)}
	}
	switch resolution.Outcome {
	case hintResolved:
		member, isOnTheBoard := taskMemberWithEmail(members, resolution.Match.Email)
		if !isOnTheBoard {
			return missingTaskOwnerResolution(members)
		}
		return taskOwnerResolution{OwnerID: member.ID}
	case hintAmbiguous:
		return ambiguousTaskOwnerResolution(taskMembersWithEmails(members, resolution.Candidates))
	case hintApproximate:
		return approximateTaskOwnerResolution(taskMembersWithEmails(members, resolution.Candidates))
	default:
		return missingTaskOwnerResolution(members)
	}
}

func unreachableDirectoryTaskResolution(errorValue error) *taskAddFailure {
	return &taskAddFailure{
		ErrorCode:    "task_owner_directory_unreachable",
		FailureStage: "target_resolution",
		Message:      "the company directory could not be reached, so who this names is not known yet: " + errorValue.Error(),
		Retryable:    true,
		SafeRetry:    true,
	}
}

func taskMemberWithEmail(members []taskMemberForTool, email string) (taskMemberForTool, bool) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return taskMemberForTool{}, false
	}
	for _, member := range members {
		if strings.ToLower(strings.TrimSpace(member.Email)) == normalizedEmail {
			return member, true
		}
	}
	return taskMemberForTool{}, false
}

func taskMembersWithEmails(members []taskMemberForTool, people []directoryPerson) []taskMemberForTool {
	matched := make([]taskMemberForTool, 0, len(people))
	for _, person := range people {
		if member, isOnTheBoard := taskMemberWithEmail(members, person.Email); isOnTheBoard {
			matched = append(matched, member)
			continue
		}
		matched = append(matched, taskMemberForTool{Name: person.Name, Email: person.Email})
	}
	return matched
}

func (service Service) resolveTaskUpdateParticipants(ctx context.Context, input taskUpdateInput, task taskForTool, members []taskMemberForTool) (*[]string, *taskAddFailure) {
	if input.ParticipantPersonHints == nil {
		return nil, nil
	}
	participantIDs, failure := service.resolveTaskParticipantIDs(ctx, *input.ParticipantPersonHints, task.OwnerID, members)
	if failure != nil {
		return nil, failure
	}
	return &participantIDs, nil
}

func (service Service) resolveTaskParticipantIDs(ctx context.Context, personHints []string, ownerID string, members []taskMemberForTool) ([]string, *taskAddFailure) {
	participantIDs := []string{strings.TrimSpace(ownerID)}
	for _, personHint := range personHints {
		resolution := service.resolveTaskOwnerHint(ctx, personHint, members)
		if resolution.Failure != nil {
			return nil, taskParticipantFailure(personHint, *resolution.Failure)
		}
		participantIDs = append(participantIDs, resolution.OwnerID)
	}
	return uniqueTaskParticipantIDs(participantIDs), nil
}

func taskParticipantFailure(personHint string, failure taskAddFailure) *taskAddFailure {
	trimmedHint := strings.TrimSpace(personHint)
	if failure.ErrorCode == "task_owner_ambiguous" {
		failure.ErrorCode = "task_participant_ambiguous"
		failure.Message = "participant " + trimmedHint + " matches more than one person; ask the user which one with ask_input, listing the candidates as choices"
		return &failure
	}
	failure.ErrorCode = "task_participant_not_found"
	failure.Message = "participant " + trimmedHint + " matched nobody; retry with one candidate's exact name, email, or @handle, or ask the user"
	return &failure
}

func uniqueTaskParticipantIDs(participantIDs []string) []string {
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
func missingTaskOwnerResolution(members []taskMemberForTool) taskOwnerResolution {
	return taskOwnerResolution{Failure: &taskAddFailure{
		ErrorCode:     "task_owner_not_found",
		FailureStage:  "target_resolution",
		Message:       "nobody matched that name and nobody came close; retry with one candidate's exact name, email, or @handle, or ask the user",
		Candidates:    taskAddCandidates(members),
		RecoveryHints: retryWithAnExactNameHint(),
		Retryable:     true,
		SafeRetry:     true,
	}}
}

// A name a couple of characters off is a typo rather than somebody else, and
// which person was meant is the user's to say, not this runtime's and not the
// model's.
func approximateTaskOwnerResolution(matches []taskMemberForTool) taskOwnerResolution {
	return taskOwnerResolution{Failure: &taskAddFailure{
		ErrorCode:     "task_owner_approximate",
		FailureStage:  "target_resolution",
		Message:       "nobody matched that name exactly, and these are the closest; ask the user with ask_input whether they meant one of them, offering none of them as a choice too. Do not choose one yourself",
		Candidates:    taskAddCandidates(matches),
		RecoveryHints: askTheUserToChooseHint(),
		Retryable:     true,
		SafeRetry:     true,
	}}
}

func (member taskMemberForTool) hintIdentifiers() []string {
	identifiers := []string{member.ID, member.Email}
	if handle := strings.TrimSpace(member.MattermostUsername); handle != "" {
		identifiers = append(identifiers, "@"+strings.TrimPrefix(handle, "@"))
	}
	return identifiers
}

func (member taskMemberForTool) hintTitle() string { return member.Name }

// A person is named, addressed, or mentioned, and each of those is mistyped in
// its own way: a name or a handle by a character or two, an address by the
// domain everyone in the company shares.
func (member taskMemberForTool) hintNearness(hint string) float64 {
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

func ambiguousTaskOwnerResolution(matches []taskMemberForTool) taskOwnerResolution {
	return taskOwnerResolution{Failure: &taskAddFailure{
		ErrorCode:     "task_owner_ambiguous",
		FailureStage:  "target_resolution",
		Message:       "the name matches more than one person; ask the user which one with ask_input, listing the candidates as choices",
		Candidates:    taskAddCandidates(matches),
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

func taskAddCandidates(members []taskMemberForTool) []taskAddCandidate {
	candidates := make([]taskAddCandidate, 0, len(members))
	for _, member := range members {
		candidates = append(candidates, taskAddCandidate{
			ID:                 member.ID,
			Name:               member.Name,
			Email:              member.Email,
			MattermostUsername: member.MattermostUsername,
			Mention:            taskAddMention(member),
		})
	}
	return candidates
}

func taskAddMention(member taskMemberForTool) string {
	handle := strings.TrimSpace(member.MattermostUsername)
	if handle == "" {
		return ""
	}
	return "@" + strings.TrimPrefix(handle, "@")
}

func (service Service) requesterTaskOwnerID(ctx context.Context, requesterEmail string, members []taskMemberForTool) string {
	resolution := service.resolveTaskOwnerHint(ctx, strings.TrimSpace(requesterEmail), members)
	if resolution.Failure != nil {
		return ""
	}
	return resolution.OwnerID
}

func taskWriteRefusal(errorValue error, task taskForTool, participantIDs *[]string, members []taskMemberForTool) (taskWriteRefusalFailure, bool) {
	var statusError taskAPIStatusError
	if !errors.As(errorValue, &statusError) || statusError.StatusCode != http.StatusForbidden {
		return taskWriteRefusalFailure{}, false
	}
	if participantIDs == nil {
		return taskWriteRefusalFailure{
			ErrorCode:    "task_task_write_forbidden",
			FailureStage: "authorization",
			Message:      "the requester may not change this task; only its owner, a participant, or an admin can",
		}, true
	}
	return taskWriteRefusalFailure{
		ErrorCode:    "task_task_assignment_forbidden",
		FailureStage: "authorization",
		Message: "only " + taskOwnerLabel(task, members) + " or an admin can change who takes part in this task, " +
			"so the requester cannot. Tell the user who has to make this change instead of retrying.",
	}, true
}

func taskOwnerLabel(task taskForTool, members []taskMemberForTool) string {
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
