package admind

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestCRMStoresPersistCoreRecords(t *testing.T) {
	ctx := context.Background()
	service := Service{Configuration: Configuration{StateDirectory: t.TempDir()}}
	audit := crmAuditFields{
		CreatedAt:         "2026-08-02T00:00:00Z",
		CreatedByPersonID: "person-owner",
		UpdatedAt:         "2026-08-02T00:00:00Z",
		UpdatedByPersonID: "person-owner",
	}
	account, errorValue := service.writeCRMAccount(ctx, crmAccount{
		ID:            "account-one",
		Name:          "관계처 하나",
		Status:        "active",
		Types:         []string{"customer", "partner"},
		Tags:          []string{"중요"},
		Importance:    "high",
		OwnerPersonID: "person-owner",
		Audit:         audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if account.ID != "account-one" || len(account.Types) != 2 {
		t.Fatalf("written account = %#v", account)
	}

	contact, errorValue := service.writeCRMContact(ctx, crmContact{
		ID:            "contact-one",
		AccountID:     account.ID,
		Name:          "담당자 하나",
		Email:         "contact@example.com",
		IsPrimary:     true,
		OwnerPersonID: "person-owner",
		Audit:         audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	opportunity, errorValue := service.writeCRMOpportunity(ctx, crmOpportunity{
		ID:            "opportunity-one",
		AccountID:     account.ID,
		Business:      "business-one",
		Name:          "진행 건 하나",
		Pipeline:      "sales",
		OwnerPersonID: "person-owner",
		AmountMinor:   crmInt64(1200000),
		Audit:         audit,
	}, []crmOpportunityContact{{ContactID: contact.ID, IsPrimary: true}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if opportunity.Stage != "waiting" || opportunity.StagePosition != 1024 || opportunity.CurrencyCode != "KRW" || opportunity.Importance != "medium" {
		t.Fatalf("opportunity defaults = %#v", opportunity)
	}

	activity, errorValue := service.writeCRMActivity(ctx, crmActivity{
		ID:            "activity-one",
		AccountID:     account.ID,
		ContactID:     contact.ID,
		OpportunityID: opportunity.ID,
		Business:      opportunity.Business,
		Kind:          "meeting",
		Title:         "첫 미팅",
		OccurredAt:    "2026-08-02T01:00:00Z",
		Content:       "요건 확인",
		Audit:         audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if activity.OpportunityID != opportunity.ID {
		t.Fatalf("written activity = %#v", activity)
	}
	_, errorValue = service.writeCRMActivity(ctx, crmActivity{
		ID:            "activity-invalid-business",
		AccountID:     account.ID,
		ContactID:     contact.ID,
		OpportunityID: opportunity.ID,
		Business:      "different-business",
		Kind:          "meeting",
		Title:         "잘못 연결된 미팅",
		OccurredAt:    "2026-08-02T01:30:00Z",
		Audit:         audit,
	})
	if errorValue == nil {
		t.Fatal("activity with mismatched opportunity business should fail")
	}

	readAccount, errorValue := service.readCRMAccount(ctx, account.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if readAccount.Name != account.Name || readAccount.Tags[0] != "중요" {
		t.Fatalf("read account = %#v", readAccount)
	}
	readContact, errorValue := service.readCRMContact(ctx, contact.ID, false)
	if errorValue != nil || readContact.Email != contact.Email {
		t.Fatalf("read contact = %#v, error = %v", readContact, errorValue)
	}
	readOpportunity, contacts, errorValue := service.readCRMOpportunity(ctx, opportunity.ID, false)
	if errorValue != nil || readOpportunity.Name != opportunity.Name || len(contacts) != 1 || !contacts[0].IsPrimary {
		t.Fatalf("read opportunity = %#v, contacts = %#v, error = %v", readOpportunity, contacts, errorValue)
	}
	readActivity, errorValue := service.readCRMActivity(ctx, activity.ID, false)
	if errorValue != nil || readActivity.Content != activity.Content {
		t.Fatalf("read activity = %#v, error = %v", readActivity, errorValue)
	}
	accounts, errorValue := service.listCRMAccounts(ctx, false)
	if errorValue != nil || len(accounts) != 1 {
		t.Fatalf("account list = %#v, error = %v", accounts, errorValue)
	}
	contactList, errorValue := service.listCRMContacts(ctx, false)
	if errorValue != nil || len(contactList) != 1 {
		t.Fatalf("contact list = %#v, error = %v", contactList, errorValue)
	}
	opportunities, errorValue := service.listCRMOpportunities(ctx, false)
	if errorValue != nil || len(opportunities) != 1 {
		t.Fatalf("opportunity list = %#v, error = %v", opportunities, errorValue)
	}
	activities, errorValue := service.listCRMActivities(ctx, false)
	if errorValue != nil || len(activities) != 1 {
		t.Fatalf("activity list = %#v, error = %v", activities, errorValue)
	}

	if errorValue := service.archiveCRMAccount(ctx, account.ID, "2026-08-02T02:00:00Z", "person-admin"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCRMAccount(ctx, account.ID, false); !errors.Is(errorValue, errCRMRecordNotFound) {
		t.Fatalf("read archived account error = %v", errorValue)
	}
	archivedAccount, errorValue := service.readCRMAccount(ctx, account.ID, true)
	if errorValue != nil || archivedAccount.Audit.ArchivedByPersonID != "person-admin" {
		t.Fatalf("archived account = %#v, error = %v", archivedAccount, errorValue)
	}
}

func TestCRMOpportunitySupportsContactOnlyCustomer(t *testing.T) {
	ctx := context.Background()
	service := Service{Configuration: Configuration{StateDirectory: t.TempDir()}}
	audit := crmAuditFields{
		CreatedAt:         "2026-08-02T00:00:00Z",
		CreatedByPersonID: "person-owner",
		UpdatedAt:         "2026-08-02T00:00:00Z",
		UpdatedByPersonID: "person-owner",
	}
	contact, errorValue := service.writeCRMContact(ctx, crmContact{
		ID:            "contact-b2c",
		Name:          "개인 고객",
		Phone:         "010-1234-5678",
		OwnerPersonID: "person-owner",
		Audit:         audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	opportunity, errorValue := service.writeCRMOpportunity(ctx, crmOpportunity{
		ID:            "opportunity-b2c",
		Name:          "개인 고객 상담",
		Pipeline:      "sales",
		OwnerPersonID: "person-owner",
		Audit:         audit,
	}, []crmOpportunityContact{{ContactID: contact.ID, IsPrimary: true}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if opportunity.AccountID != "" {
		t.Fatalf("contact-only opportunity account = %q", opportunity.AccountID)
	}
}

func TestCRMStageTransitionIsAtomicAndRecordsActivity(t *testing.T) {
	ctx := context.Background()
	service, opportunity := createCRMOpportunityForStoreTest(t)
	changedWithoutTransition := opportunity
	changedWithoutTransition.Stage = "in_progress"
	changedWithoutTransition.Audit.UpdatedAt = "2026-08-02T02:30:00Z"
	if _, errorValue := service.writeCRMOpportunity(ctx, changedWithoutTransition, nil); errorValue == nil {
		t.Fatal("generic CRM opportunity update should not change stage")
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.Exec(`
CREATE TRIGGER abort_stage_change_activity
BEFORE INSERT ON activity
WHEN NEW.kind = 'stage_change'
BEGIN
	SELECT RAISE(ABORT, 'forced stage history failure');
END`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	errorValue = service.transitionCRMOpportunityStage(ctx, crmOpportunityStageTransition{
		OpportunityID: opportunity.ID,
		Stage:         "in_progress",
		StagePosition: 1024,
		OccurredAt:    "2026-08-02T03:00:00Z",
		ActorPersonID: "person-owner",
	})
	if errorValue == nil {
		t.Fatal("stage transition should fail when history insert fails")
	}
	readOpportunity, _, errorValue := service.readCRMOpportunity(ctx, opportunity.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if readOpportunity.Stage != "waiting" {
		t.Fatalf("stage after rollback = %q, want lead", readOpportunity.Stage)
	}
	changedWithTransition := readOpportunity
	changedWithTransition.Name = "함께 저장할 이름"
	changedWithTransition.Audit.UpdatedAt = "2026-08-02T03:30:00Z"
	if _, errorValue := service.writeCRMOpportunityWithTransition(ctx, changedWithTransition, nil, &crmOpportunityStageTransition{
		OpportunityID: opportunity.ID,
		Stage:         "in_progress",
		StagePosition: 1024,
		OccurredAt:    "2026-08-02T03:30:00Z",
		ActorPersonID: "person-owner",
	}); errorValue == nil {
		t.Fatal("combined CRM opportunity update should roll back when transition history fails")
	}
	readOpportunity, _, errorValue = service.readCRMOpportunity(ctx, opportunity.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if readOpportunity.Name != opportunity.Name || readOpportunity.Stage != "waiting" {
		t.Fatalf("combined update after rollback = %#v", readOpportunity)
	}

	database, errorValue = service.openCRMDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec("DROP TRIGGER abort_stage_change_activity"); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	errorValue = service.transitionCRMOpportunityStage(ctx, crmOpportunityStageTransition{
		OpportunityID:    opportunity.ID,
		Stage:            "done",
		StagePosition:    1024,
		OccurredAt:       "2026-08-02T04:00:00Z",
		ActorPersonID:    "person-owner",
		BaseAmountMinor:  crmInt64(500000),
		BaseCurrencyCode: "KRW",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	readOpportunity, _, errorValue = service.readCRMOpportunity(ctx, opportunity.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if readOpportunity.Stage != "done" || readOpportunity.StageChangedAt != "2026-08-02T04:00:00Z" || readOpportunity.BaseAmountMinor == nil || *readOpportunity.BaseAmountMinor != 500000 {
		t.Fatalf("won opportunity = %#v", readOpportunity)
	}
	database, errorValue = service.openCRMDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var historyCount int
	if errorValue := database.QueryRow("SELECT COUNT(*) FROM activity WHERE opportunity_id = ? AND kind = 'stage_change'", opportunity.ID).Scan(&historyCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if historyCount != 1 {
		t.Fatalf("stage history count = %d, want 1", historyCount)
	}
}

func TestCRMStageTransitionRejectsEmptyLostReason(t *testing.T) {
	ctx := context.Background()
	service, opportunity := createCRMOpportunityForStoreTest(t)

	errorValue := service.transitionCRMOpportunityStage(ctx, crmOpportunityStageTransition{
		OpportunityID: opportunity.ID,
		Stage:         "lost",
		StagePosition: 1024,
		OccurredAt:    "2026-08-02T03:00:00Z",
		ActorPersonID: "person-owner",
		LostReason:    "   ",
	})
	if errorValue == nil {
		t.Fatal("lost transition without a reason should fail")
	}

	readOpportunity, _, errorValue := service.readCRMOpportunity(ctx, opportunity.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if readOpportunity.Stage != "waiting" || readOpportunity.LostReason != "" {
		t.Fatalf("opportunity after rejected lost transition = %#v", readOpportunity)
	}
}

func TestCRMStageTransitionStoresFreeTextLostReasonAndClearsItForNonLostTransitions(t *testing.T) {
	ctx := context.Background()
	service, opportunity := createCRMOpportunityForStoreTest(t)
	freeTextReason := "고객 예산 축소로 계약 보류"

	if errorValue := service.transitionCRMOpportunityStage(ctx, crmOpportunityStageTransition{
		OpportunityID:    opportunity.ID,
		Stage:            "lost",
		StagePosition:    1024,
		OccurredAt:       "2026-08-02T03:00:00Z",
		ActorPersonID:    "person-owner",
		LostReason:       "  " + freeTextReason + "  ",
		BaseAmountMinor:  crmInt64(500000),
		BaseCurrencyCode: "KRW",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	afterLostTransition, _, errorValue := service.readCRMOpportunity(ctx, opportunity.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if afterLostTransition.Stage != "lost" || afterLostTransition.LostReason != freeTextReason {
		t.Fatalf("opportunity after free-text lost transition = %#v, want lost reason %q", afterLostTransition, freeTextReason)
	}

	if errorValue := service.transitionCRMOpportunityStage(ctx, crmOpportunityStageTransition{
		OpportunityID:    opportunity.ID,
		Stage:            "done",
		StagePosition:    2048,
		OccurredAt:       "2026-08-02T03:30:00Z",
		ActorPersonID:    "person-owner",
		LostReason:       "should be dropped for a non-lost transition",
		BaseAmountMinor:  crmInt64(500000),
		BaseCurrencyCode: "KRW",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	afterWonTransition, _, errorValue := service.readCRMOpportunity(ctx, opportunity.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if afterWonTransition.Stage != "done" || afterWonTransition.LostReason != "" {
		t.Fatalf("opportunity after non-lost transition = %#v, want lost reason cleared", afterWonTransition)
	}
}

func TestCRMOpportunityGenericUpdatePreservesStagePosition(t *testing.T) {
	ctx := context.Background()
	service, opportunity := createCRMOpportunityForStoreTest(t)
	originalPosition := opportunity.StagePosition
	opportunity.Name = "이름만 변경"
	opportunity.StagePosition = 4096
	opportunity.Audit.UpdatedAt = "2026-08-02T04:30:00Z"

	updated, errorValue := service.writeCRMOpportunity(ctx, opportunity, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if updated.Name != "이름만 변경" || updated.StagePosition != originalPosition {
		t.Fatalf("updated opportunity = %#v", updated)
	}
}

func TestCRMRealizedAmountsCannotChange(t *testing.T) {
	ctx := context.Background()
	service, opportunity := createCRMOpportunityForStoreTest(t)
	if errorValue := service.transitionCRMOpportunityStage(ctx, crmOpportunityStageTransition{
		OpportunityID:    opportunity.ID,
		Stage:            "done",
		StagePosition:    1024,
		OccurredAt:       "2026-08-02T04:30:00Z",
		ActorPersonID:    "person-owner",
		BaseAmountMinor:  crmInt64(500000),
		BaseCurrencyCode: "KRW",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	realized, _, errorValue := service.readCRMOpportunity(ctx, opportunity.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	testCases := []struct {
		name   string
		change func(*crmOpportunity)
	}{
		{name: "amount", change: func(changed *crmOpportunity) { changed.AmountMinor = crmInt64(600000) }},
		{name: "currency", change: func(changed *crmOpportunity) { changed.CurrencyCode = "USD" }},
		{name: "base amount", change: func(changed *crmOpportunity) { changed.BaseAmountMinor = crmInt64(600000) }},
		{name: "base currency", change: func(changed *crmOpportunity) { changed.BaseCurrencyCode = "USD" }},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			changed := realized
			testCase.change(&changed)
			changed.Audit.UpdatedAt = "2026-08-02T04:31:00Z"
			if _, errorValue := service.writeCRMOpportunity(ctx, changed, nil); errorValue == nil {
				t.Fatalf("realized CRM %s update should fail", testCase.name)
			}
		})
	}
	if errorValue := service.transitionCRMOpportunityStage(ctx, crmOpportunityStageTransition{
		OpportunityID: opportunity.ID,
		Stage:         "waiting",
		StagePosition: 1024,
		OccurredAt:    "2026-08-02T04:32:00Z",
		ActorPersonID: "person-owner",
	}); errorValue == nil {
		t.Fatal("realized CRM opportunity should not return to an unrealized stage")
	}
}

func TestCRMStageChangeActivityCanBeEditedButCannotReturnAfterKindChange(t *testing.T) {
	ctx := context.Background()
	service, opportunity := createCRMOpportunityForStoreTest(t)
	if errorValue := service.transitionCRMOpportunityStage(ctx, crmOpportunityStageTransition{
		OpportunityID: opportunity.ID,
		Stage:         "in_progress",
		StagePosition: 1024,
		OccurredAt:    "2026-08-02T04:30:00Z",
		ActorPersonID: "person-owner",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	activities, errorValue := service.listCRMActivities(ctx, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(activities) != 1 || activities[0].Kind != "stage_change" {
		t.Fatalf("stage change activities = %#v", activities)
	}
	activity := activities[0]
	activity.Kind = "note"
	activity.Title = "변경된 기록"
	activity.Audit.UpdatedAt = "2026-08-02T04:31:00Z"
	written, errorValue := service.writeCRMActivity(ctx, activity)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written.Kind != "note" || written.Title != "변경된 기록" {
		t.Fatalf("rewritten stage change activity = %#v", written)
	}
	updatedOpportunity, _, errorValue := service.readCRMOpportunity(ctx, opportunity.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if updatedOpportunity.Stage != "in_progress" || updatedOpportunity.StageChangedAt != "2026-08-02T04:30:00Z" {
		t.Fatalf("opportunity stage changed while editing activity = %#v", updatedOpportunity)
	}
	activity.Kind = "stage_change"
	activity.Audit.UpdatedAt = "2026-08-02T04:32:00Z"
	if _, errorValue := service.writeCRMActivity(ctx, activity); errorValue == nil {
		t.Fatal("normal activity must not become stage change")
	}
}

func TestCRMStagePositionCollisionRebalancesOnlyCurrentStage(t *testing.T) {
	ctx := context.Background()
	service, first := createCRMOpportunityForStoreTest(t)
	second, errorValue := service.writeCRMOpportunity(ctx, crmOpportunity{
		ID:            "opportunity-two",
		AccountID:     first.AccountID,
		Name:          "두 번째 진행 건",
		Pipeline:      "sales",
		Stage:         "waiting",
		StagePosition: 2048,
		OwnerPersonID: "person-owner",
		Audit:         first.Audit,
	}, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.setCRMOpportunityStagePosition(ctx, second.ID, first.StagePosition, first.ID, "2026-08-02T05:00:00Z", "person-owner"); errorValue != nil {
		t.Fatal(errorValue)
	}
	first, _, errorValue = service.readCRMOpportunity(ctx, first.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	second, _, errorValue = service.readCRMOpportunity(ctx, second.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if first.StagePosition == second.StagePosition || first.StagePosition/1024 != float64(int(first.StagePosition/1024)) || second.StagePosition/1024 != float64(int(second.StagePosition/1024)) {
		t.Fatalf("rebalanced positions = %v, %v", first.StagePosition, second.StagePosition)
	}
}

func TestCRMStagePositionFallbackPreservesInsertionAnchor(t *testing.T) {
	ctx := context.Background()
	service, existing := createCRMOpportunityForStoreTest(t)
	left, errorValue := service.writeCRMOpportunity(ctx, crmOpportunity{
		ID: "opportunity-left", AccountID: existing.AccountID, Name: "왼쪽 진행 건", Pipeline: "sales",
		Stage: "waiting", StagePosition: 1, OwnerPersonID: "person-owner", Audit: existing.Audit,
	}, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	right, errorValue := service.writeCRMOpportunity(ctx, crmOpportunity{
		ID: "opportunity-right", AccountID: existing.AccountID, Name: "오른쪽 진행 건", Pipeline: "sales",
		Stage: "waiting", StagePosition: math.Nextafter(left.StagePosition, math.Inf(1)), OwnerPersonID: "person-owner", Audit: existing.Audit,
	}, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	moving, errorValue := service.writeCRMOpportunity(ctx, crmOpportunity{
		ID: "opportunity-moving", AccountID: existing.AccountID, Name: "이동 진행 건", Pipeline: "sales",
		Stage: "waiting", StagePosition: 0.5, OwnerPersonID: "person-owner", Audit: existing.Audit,
	}, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	midpoint := (left.StagePosition + right.StagePosition) / 2
	if midpoint != left.StagePosition {
		t.Fatalf("midpoint = %v, want collision with %v", midpoint, left.StagePosition)
	}
	if errorValue := service.setCRMOpportunityStagePosition(ctx, moving.ID, midpoint, right.ID, "2026-08-02T05:00:00Z", "person-owner"); errorValue != nil {
		t.Fatal(errorValue)
	}
	left, _, errorValue = service.readCRMOpportunity(ctx, left.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	moving, _, errorValue = service.readCRMOpportunity(ctx, moving.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	right, _, errorValue = service.readCRMOpportunity(ctx, right.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !(left.StagePosition < moving.StagePosition && moving.StagePosition < right.StagePosition) {
		t.Fatalf("anchored positions = left %v, moving %v, right %v", left.StagePosition, moving.StagePosition, right.StagePosition)
	}
}

func TestCRMOpportunityCreationPositionCollisionRebalancesStage(t *testing.T) {
	ctx := context.Background()
	service, first := createCRMOpportunityForStoreTest(t)
	second, errorValue := service.writeCRMOpportunity(ctx, crmOpportunity{
		ID:            "opportunity-creation-collision",
		AccountID:     first.AccountID,
		Name:          "같은 위치의 진행 건",
		Pipeline:      "sales",
		Stage:         "waiting",
		StagePosition: first.StagePosition,
		OwnerPersonID: "person-owner",
		Audit:         first.Audit,
	}, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	first, _, errorValue = service.readCRMOpportunity(ctx, first.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if first.StagePosition == second.StagePosition {
		t.Fatalf("created positions = %v, %v", first.StagePosition, second.StagePosition)
	}
	invalid := second
	invalid.ID = "opportunity-invalid-position"
	invalid.StagePosition = math.Inf(1)
	if _, errorValue := service.writeCRMOpportunity(ctx, invalid, nil); errorValue == nil {
		t.Fatal("infinite CRM stage position should fail")
	}
}

func TestCRMStageTransitionPositionCollisionRebalancesTargetStage(t *testing.T) {
	ctx := context.Background()
	service, moving := createCRMOpportunityForStoreTest(t)
	existing, errorValue := service.writeCRMOpportunity(ctx, crmOpportunity{
		ID:            "opportunity-qualified",
		AccountID:     moving.AccountID,
		Name:          "검토 중인 진행 건",
		Pipeline:      "sales",
		Stage:         "in_progress",
		StagePosition: 1024,
		OwnerPersonID: "person-owner",
		Audit:         moving.Audit,
	}, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.transitionCRMOpportunityStage(ctx, crmOpportunityStageTransition{
		OpportunityID:       moving.ID,
		Stage:               "in_progress",
		StagePosition:       existing.StagePosition,
		BeforeOpportunityID: existing.ID,
		OccurredAt:          "2026-08-02T05:00:00Z",
		ActorPersonID:       "person-owner",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	moving, _, errorValue = service.readCRMOpportunity(ctx, moving.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	existing, _, errorValue = service.readCRMOpportunity(ctx, existing.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if moving.StagePosition == existing.StagePosition {
		t.Fatalf("target stage positions = %v, %v", moving.StagePosition, existing.StagePosition)
	}
}

func TestCRMOpportunityPrimaryContactReplacementIgnoresInputOrder(t *testing.T) {
	ctx := context.Background()
	service, opportunity := createCRMOpportunityForStoreTest(t)
	first, errorValue := service.writeCRMContact(ctx, crmContact{
		ID: "contact-primary-first", AccountID: opportunity.AccountID, Name: "기존 주 연락처", Email: "first-primary@example.com",
		OwnerPersonID: "person-owner", Audit: opportunity.Audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	second, errorValue := service.writeCRMContact(ctx, crmContact{
		ID: "contact-primary-second", AccountID: opportunity.AccountID, Name: "새 주 연락처", Email: "second-primary@example.com",
		OwnerPersonID: "person-owner", Audit: opportunity.Audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = service.writeCRMOpportunity(ctx, opportunity, []crmOpportunityContact{
		{ContactID: first.ID, IsPrimary: true},
		{ContactID: second.ID},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	opportunity.Audit.UpdatedAt = "2026-08-02T05:00:00Z"
	_, errorValue = service.writeCRMOpportunity(ctx, opportunity, []crmOpportunityContact{
		{ContactID: second.ID, IsPrimary: true},
		{ContactID: first.ID},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, contacts, errorValue := service.readCRMOpportunity(ctx, opportunity.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(contacts) != 2 || contacts[0].ContactID != second.ID || !contacts[0].IsPrimary || contacts[1].IsPrimary {
		t.Fatalf("replaced CRM opportunity contacts = %#v", contacts)
	}
}

func TestCRMOpportunityRejectsCustomerModelChange(t *testing.T) {
	ctx := context.Background()
	service, accountOpportunity := createCRMOpportunityForStoreTest(t)
	contact, errorValue := service.writeCRMContact(ctx, crmContact{
		ID: "contact-customer-model", Name: "개인 고객", Email: "customer-model@example.com",
		OwnerPersonID: "person-owner", Audit: accountOpportunity.Audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	accountID := accountOpportunity.AccountID
	accountOpportunity.AccountID = ""
	accountOpportunity.Audit.UpdatedAt = "2026-08-02T05:00:00Z"
	if _, errorValue := service.writeCRMOpportunity(ctx, accountOpportunity, []crmOpportunityContact{{ContactID: contact.ID, IsPrimary: true}}); errorValue == nil || errorValue.Error() != "CRM opportunity cannot change between account and contact-only customers" {
		t.Fatalf("account to contact-only error = %v", errorValue)
	}
	contactOpportunity, errorValue := service.writeCRMOpportunity(ctx, crmOpportunity{
		ID: "opportunity-contact-model", Name: "개인 고객 진행 건", Pipeline: "sales",
		OwnerPersonID: "person-owner", Audit: accountOpportunity.Audit,
	}, []crmOpportunityContact{{ContactID: contact.ID, IsPrimary: true}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	contactOpportunity.AccountID = accountID
	contactOpportunity.Audit.UpdatedAt = "2026-08-02T05:01:00Z"
	if _, errorValue := service.writeCRMOpportunity(ctx, contactOpportunity, []crmOpportunityContact{}); errorValue == nil || errorValue.Error() != "CRM opportunity cannot change between account and contact-only customers" {
		t.Fatalf("contact-only to account error = %v", errorValue)
	}
}

func TestCRMOpportunityCanReplaceAccountAndContactsTogether(t *testing.T) {
	ctx := context.Background()
	service := Service{Configuration: Configuration{StateDirectory: t.TempDir()}}
	audit := crmAuditFields{
		CreatedAt:         "2026-08-02T00:00:00Z",
		CreatedByPersonID: "person-owner",
		UpdatedAt:         "2026-08-02T00:00:00Z",
		UpdatedByPersonID: "person-owner",
	}
	firstAccount, errorValue := service.writeCRMAccount(ctx, crmAccount{
		ID: "account-first", Name: "첫 관계처", Status: "active", Tags: []string{},
		Importance: "medium", OwnerPersonID: "person-owner", Audit: audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondAccount, errorValue := service.writeCRMAccount(ctx, crmAccount{
		ID: "account-second", Name: "두 번째 관계처", Status: "active", Tags: []string{},
		Importance: "medium", OwnerPersonID: "person-owner", Audit: audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	firstContact, errorValue := service.writeCRMContact(ctx, crmContact{
		ID: "contact-first", AccountID: firstAccount.ID, Name: "첫 담당자", Email: "first@example.com",
		OwnerPersonID: "person-owner", Audit: audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondContact, errorValue := service.writeCRMContact(ctx, crmContact{
		ID: "contact-second", AccountID: secondAccount.ID, Name: "두 번째 담당자", Email: "second@example.com",
		OwnerPersonID: "person-owner", Audit: audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	opportunity, errorValue := service.writeCRMOpportunity(ctx, crmOpportunity{
		ID: "opportunity-reassign", AccountID: firstAccount.ID, Name: "담당 관계처 변경", Pipeline: "sales",
		OwnerPersonID: "person-owner", Audit: audit,
	}, []crmOpportunityContact{{ContactID: firstContact.ID, IsPrimary: true}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	opportunity.AccountID = secondAccount.ID
	opportunity.Audit.UpdatedAt = "2026-08-02T05:00:00Z"
	updated, errorValue := service.writeCRMOpportunity(ctx, opportunity, []crmOpportunityContact{{ContactID: secondContact.ID, IsPrimary: true}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, contacts, errorValue := service.readCRMOpportunity(ctx, opportunity.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if updated.AccountID != secondAccount.ID || len(contacts) != 1 || contacts[0].ContactID != secondContact.ID {
		t.Fatalf("updated opportunity = %#v, contacts = %#v", updated, contacts)
	}
}

func TestCRMContactRejectsAccountChangeThatBreaksOpportunityLink(t *testing.T) {
	ctx := context.Background()
	service, opportunity := createCRMOpportunityForStoreTest(t)
	contact, errorValue := service.writeCRMContact(ctx, crmContact{
		ID: "contact-linked-account", AccountID: opportunity.AccountID, Name: "연결 담당자", Email: "linked-account@example.com",
		OwnerPersonID: "person-owner", Audit: opportunity.Audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.writeCRMOpportunity(ctx, opportunity, []crmOpportunityContact{{ContactID: contact.ID, IsPrimary: true}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	secondAccount, errorValue := service.writeCRMAccount(ctx, crmAccount{
		ID: "account-contact-target", Name: "이동 대상 관계처", Status: "active", Tags: []string{},
		Importance: "medium", OwnerPersonID: "person-owner", Audit: opportunity.Audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	contact.AccountID = secondAccount.ID
	contact.Audit.UpdatedAt = "2026-08-02T05:00:00Z"
	if _, errorValue := service.writeCRMContact(ctx, contact); errorValue == nil || !strings.Contains(errorValue.Error(), "opportunity contact account mismatch") {
		t.Fatalf("contact account change error = %v", errorValue)
	}
}

func TestCRMReferenceStoreUsesDynamicListsAndUnlimitedLinks(t *testing.T) {
	ctx := context.Background()
	service, opportunity := createCRMOpportunityForStoreTest(t)
	pipelines, errorValue := service.listCRMPipelines(ctx, true)
	if errorValue != nil || len(pipelines) != 6 {
		t.Fatalf("pipelines = %#v, error = %v", pipelines, errorValue)
	}

	first, errorValue := service.writeCRMResourceLink(ctx, crmResourceLink{
		ID: "resource-one", EntityType: "opportunity", EntityID: opportunity.ID,
		Service: "flow", ExternalResourceType: "task", ExternalResourceID: "task-one",
		CreatedAt: "2026-08-02T06:00:00Z", CreatedByPersonID: "person-owner",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = service.writeCRMResourceLink(ctx, crmResourceLink{
		ID: "resource-two", EntityType: "opportunity", EntityID: opportunity.ID,
		Service: "calendar", ExternalResourceType: "event", ExternalResourceID: "event-one",
		CreatedAt: "2026-08-02T06:01:00Z", CreatedByPersonID: "person-owner",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = service.writeCRMResourceLink(ctx, crmResourceLink{
		ID: "resource-duplicate", EntityType: "opportunity", EntityID: opportunity.ID,
		Service: "flow", ExternalResourceType: "task", ExternalResourceID: "task-one",
		CreatedAt: "2026-08-02T06:02:00Z", CreatedByPersonID: "person-owner",
	})
	if errorValue == nil {
		t.Fatal("duplicate active CRM resource link should fail")
	}
	links, errorValue := service.listCRMResourceLinks(ctx, "opportunity", opportunity.ID, true)
	if errorValue != nil || len(links) != 2 {
		t.Fatalf("active links = %#v, error = %v", links, errorValue)
	}
	if errorValue := service.removeCRMResourceLink(ctx, first.ID, "2026-08-02T07:00:00Z", "person-owner"); errorValue != nil {
		t.Fatal(errorValue)
	}
	links, errorValue = service.listCRMResourceLinks(ctx, "opportunity", opportunity.ID, true)
	if errorValue != nil || len(links) != 1 {
		t.Fatalf("active links after removal = %#v, error = %v", links, errorValue)
	}
}

func createCRMOpportunityForStoreTest(t *testing.T) (*Service, crmOpportunity) {
	t.Helper()
	ctx := context.Background()
	service := &Service{Configuration: Configuration{StateDirectory: t.TempDir()}}
	audit := crmAuditFields{
		CreatedAt:         "2026-08-02T00:00:00Z",
		CreatedByPersonID: "person-owner",
		UpdatedAt:         "2026-08-02T00:00:00Z",
		UpdatedByPersonID: "person-owner",
	}
	account, errorValue := service.writeCRMAccount(ctx, crmAccount{
		ID: "account-stage", Name: "단계 관계처", Status: "active", Tags: []string{},
		Importance: "medium", OwnerPersonID: "person-owner", Audit: audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	opportunity, errorValue := service.writeCRMOpportunity(ctx, crmOpportunity{
		ID: "opportunity-stage", AccountID: account.ID, Name: "단계 진행 건", Pipeline: "sales",
		Stage: "waiting", StagePosition: 1024, OwnerPersonID: "person-owner", AmountMinor: crmInt64(500000), Audit: audit,
	}, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return service, opportunity
}

func crmInt64(value int64) *int64 {
	return &value
}

func TestCRMRecordsSurviveAServiceRestart(t *testing.T) {
	ctx := context.Background()
	stateDirectory := t.TempDir()
	audit := crmAuditFields{
		CreatedAt:         "2026-08-02T00:00:00Z",
		CreatedByPersonID: "person-owner",
		UpdatedAt:         "2026-08-02T00:00:00Z",
		UpdatedByPersonID: "person-owner",
	}

	writer := Service{Configuration: Configuration{StateDirectory: stateDirectory}}
	account, errorValue := writer.writeCRMAccount(ctx, crmAccount{
		ID: "account-restart", Name: "재시작 관계처", Status: "active", Tags: []string{},
		Importance: "medium", OwnerPersonID: "person-owner", Audit: audit,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	opportunity, errorValue := writer.writeCRMOpportunity(ctx, crmOpportunity{
		ID: "opportunity-restart", AccountID: account.ID, Name: "재시작 진행 건", Pipeline: "sales",
		Stage: "waiting", StagePosition: 1024, OwnerPersonID: "person-owner", AmountMinor: crmInt64(500000), Audit: audit,
	}, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	reader := Service{Configuration: Configuration{StateDirectory: stateDirectory}}
	opportunities, errorValue := reader.listCRMOpportunities(ctx, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(opportunities) != 1 || opportunities[0].ID != opportunity.ID || opportunities[0].Name != "재시작 진행 건" {
		t.Fatalf("opportunities after restart = %#v", opportunities)
	}
	if opportunities[0].AccountID != account.ID || opportunities[0].Stage != "waiting" {
		t.Fatalf("opportunity lost its links after restart = %#v", opportunities[0])
	}
}
