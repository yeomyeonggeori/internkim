package capabilityd

import (
	"encoding/json"
	"testing"
)

func TestAnApprovedRecipientPassedBackAsTheApprovalShowedItReachesTheSamePerson(t *testing.T) {
	service := messageSendTargetService(t, messageSendTargetPeople())
	approved := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", messageSendTargetInput("박예시")))

	for _, shown := range []string{approved.Title, approved.ID} {
		passedBack := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", messageSendTargetInput(shown)))
		if passedBack.ID != approved.ID {
			t.Fatalf("%q was shown for %s and reached %+v", shown, approved.ID, passedBack)
		}
	}
}

func TestEveryCandidateTheModelReadsCarriesAPersonIDThatReachesThatCandidate(t *testing.T) {
	service := messageSendTargetService(t, platformDMAmbiguousTestPeople())
	question := resolveApprovalTargetThroughRoute(t, service, "message_send", messageSendTargetInput("lee"))

	var readByTheModel platformDMFailure
	if errorValue := json.Unmarshal([]byte(question.Content), &readByTheModel); errorValue != nil || len(readByTheModel.Candidates) != 2 {
		t.Fatalf("the model reads the candidates themselves, got %q", question.Content)
	}
	for _, candidate := range readByTheModel.Candidates {
		chosen := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", messageSendTargetInput(candidate.PersonID)))
		if chosen.ID != candidate.PersonID {
			t.Fatalf("candidate %+v passed back reached %+v", candidate, chosen)
		}
	}
}
