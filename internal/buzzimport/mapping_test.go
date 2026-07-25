package buzzimport

import (
	"strings"
	"testing"
	"time"
)

type stubResolver struct {
	secretsByEmail    map[string]string
	pubkeysByUsername map[string]string
}

func (resolver stubResolver) SecretForEmail(email string) (string, error) {
	secret, isKnown := resolver.secretsByEmail[strings.ToLower(email)]
	if !isKnown {
		return "", errAuthorHasNoBuzzIdentity
	}
	return secret, nil
}

func (resolver stubResolver) PubkeyForUsername(username string) (string, bool) {
	pubkey, isKnown := resolver.pubkeysByUsername[strings.ToLower(username)]
	return pubkey, isKnown
}

func newStubResolver() stubResolver {
	return stubResolver{
		secretsByEmail: map[string]string{
			"lee@example.com":  testAuthorSecret,
			"kwak@example.com": "2278851e7a6068811817d7d1a9f3bb126279e84ad078e832908f785cd39c5aa8",
		},
		pubkeysByUsername: map[string]string{"kwak": strings.Repeat("c", 64)},
	}
}

func postAt(id string, userID string, rootID string, message string, minute int) MattermostPost {
	return MattermostPost{
		ID:        id,
		ChannelID: "mm-channel",
		UserID:    userID,
		RootID:    rootID,
		Message:   message,
		CreatedAt: time.Date(2026, time.March, 14, 9, minute, 0, 0, time.UTC),
	}
}

func basePlan(posts []MattermostPost) ChannelImportPlan {
	return ChannelImportPlan{
		BuzzChannelID: "4b24ca45-2860-42f4-bdca-4380f803d2aa",
		Posts:         posts,
		AuthorEmails:  map[string]string{"u-lee": "lee@example.com", "u-kwak": "kwak@example.com"},
	}
}

func TestPlanKeepsOrderAndResolvesRepliesToTheirRoot(t *testing.T) {
	plan := basePlan([]MattermostPost{
		postAt("p1", "u-lee", "", "회고 시작합니다", 10),
		postAt("p2", "u-kwak", "p1", "네 정리해둔 것 붙일게요", 12),
	})
	messages, skipped, errorValue := PlanChannelImport(plan, newStubResolver())
	if errorValue != nil {
		t.Fatalf("plan import: %v", errorValue)
	}
	if len(skipped) != 0 {
		t.Fatalf("expected nothing skipped, got %v", skipped)
	}
	if len(messages) != 2 {
		t.Fatalf("expected two messages, got %d", len(messages))
	}
	rootEvent, errorValue := BuildStreamEvent(messages[0])
	if errorValue != nil {
		t.Fatalf("rebuild root: %v", errorValue)
	}
	if messages[1].RootEventID != rootEvent.ID {
		t.Fatalf("the reply must point at the event its root became, got %q", messages[1].RootEventID)
	}
	if !messages[0].SentAt.Before(messages[1].SentAt) {
		t.Fatal("messages must stay in send order")
	}
}

func TestPlanSkipsAuthorsWithoutABuzzIdentityAndReportsThem(t *testing.T) {
	plan := basePlan([]MattermostPost{
		postAt("p1", "u-lee", "", "남는 메시지", 10),
		postAt("p2", "u-stranger", "", "신원 없는 작성자", 11),
	})
	plan.AuthorEmails["u-stranger"] = "stranger@example.com"

	messages, skipped, errorValue := PlanChannelImport(plan, newStubResolver())
	if errorValue != nil {
		t.Fatalf("plan import: %v", errorValue)
	}
	if len(messages) != 1 {
		t.Fatalf("expected the known author's message only, got %d", len(messages))
	}
	if len(skipped) != 1 || skipped[0] != "p2" {
		t.Fatalf("the unimportable post must be reported, got %v", skipped)
	}
}

func TestPlanSkipsARepliyWhoseRootWasNotImported(t *testing.T) {
	plan := basePlan([]MattermostPost{postAt("p2", "u-lee", "missing-root", "고아 답글", 12)})

	messages, skipped, errorValue := PlanChannelImport(plan, newStubResolver())
	if errorValue != nil {
		t.Fatalf("plan import: %v", errorValue)
	}
	if len(messages) != 0 {
		t.Fatal("a reply with no imported root must not be planted at the channel root")
	}
	if len(skipped) != 1 || skipped[0] != "p2" {
		t.Fatalf("the orphan reply must be reported, got %v", skipped)
	}
}

func TestPlanCarriesMentionsItCanResolve(t *testing.T) {
	plan := basePlan([]MattermostPost{postAt("p1", "u-lee", "", "@kwak 확인 부탁드립니다 @nobody", 10)})

	messages, _, errorValue := PlanChannelImport(plan, newStubResolver())
	if errorValue != nil {
		t.Fatalf("plan import: %v", errorValue)
	}
	if len(messages[0].MentionPubkeys) != 1 {
		t.Fatalf("expected only the resolvable mention, got %v", messages[0].MentionPubkeys)
	}
	if !strings.Contains(messages[0].Text, "@kwak") {
		t.Fatal("the original message text must survive unchanged")
	}
}
