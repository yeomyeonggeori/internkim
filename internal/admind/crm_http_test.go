package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCRMHTTPCRUDPersistsAcrossServiceRestart(t *testing.T) {
	service := newCRMHTTPTestService(t)
	account := createCRMHTTPTestAccount(t, service, "owner@example.com", "person-owner", "team-sales")

	contactResponse := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/contacts", "owner@example.com", map[string]any{
		"accountID": account.ID, "name": "담당자", "email": "contact@example.com",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, contactResponse, http.StatusCreated)
	var contactDocument struct {
		Contact crmHTTPContact `json:"contact"`
	}
	decodeCRMHTTPTestResponse(t, contactResponse, &contactDocument)

	opportunityResponse := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities", "owner@example.com", map[string]any{
		"accountID": account.ID, "business": "Enterprise", "name": "계약",
		"pipeline": "sales", "ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
		"amountMinor": 1000000, "currencyCode": "KRW", "importance": "high",
		"dueAt": "2026-08-10T03:00:00Z", "dueTimeZone": "Asia/Seoul",
		"contacts": []map[string]any{{"contactID": contactDocument.Contact.ID, "isPrimary": true}},
	})
	requireCRMHTTPStatus(t, opportunityResponse, http.StatusCreated)
	var opportunityDocument struct {
		Opportunity crmHTTPOpportunity `json:"opportunity"`
	}
	decodeCRMHTTPTestResponse(t, opportunityResponse, &opportunityDocument)
	if opportunityDocument.Opportunity.DueAt != "2026-08-10T03:00:00Z" ||
		opportunityDocument.Opportunity.DueTimeZone != "Asia/Seoul" {
		t.Fatalf(
			"opportunity due time = %q in %q",
			opportunityDocument.Opportunity.DueAt,
			opportunityDocument.Opportunity.DueTimeZone,
		)
	}

	activityResponse := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/activities", "owner@example.com", map[string]any{
		"opportunityID": opportunityDocument.Opportunity.ID, "contactID": contactDocument.Contact.ID,
		"kind": "meeting", "title": "미팅", "occurredAt": "2026-08-03T04:00:00Z",
	})
	requireCRMHTTPStatus(t, activityResponse, http.StatusCreated)
	var activityDocument struct {
		Activity crmHTTPActivity `json:"activity"`
	}
	decodeCRMHTTPTestResponse(t, activityResponse, &activityDocument)

	contactUpdate := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/contacts/"+contactDocument.Contact.ID, "teammate@example.com", map[string]any{
		"accountID": account.ID, "name": "담당자 수정", "email": "updated@example.com",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, contactUpdate, http.StatusOK)

	opportunityUpdate := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/opportunities/"+opportunityDocument.Opportunity.ID, "teammate@example.com", map[string]any{
		"accountID": account.ID, "business": "Enterprise", "name": "계약 수정",
		"pipeline": "sales", "ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
		"dueAt": "2026-08-10T03:00:00Z", "dueTimeZone": "Asia/Seoul",
		"contacts": []map[string]any{{"contactID": contactDocument.Contact.ID, "isPrimary": true}},
	})
	requireCRMHTTPStatus(t, opportunityUpdate, http.StatusOK)

	activityUpdate := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/activities/"+activityDocument.Activity.ID, "teammate@example.com", map[string]any{
		"opportunityID": opportunityDocument.Opportunity.ID, "contactID": contactDocument.Contact.ID,
		"kind": "meeting", "title": "미팅 수정", "occurredAt": "2026-08-03T04:30:00Z",
	})
	requireCRMHTTPStatus(t, activityUpdate, http.StatusOK)

	restarted := NewService(service.Configuration)
	readResponse := crmHTTPTestRequest(t, restarted, http.MethodGet, "/crm/api/accounts/"+account.ID, "teammate@example.com", nil)
	requireCRMHTTPStatus(t, readResponse, http.StatusOK)
	var readDocument struct {
		Account crmHTTPAccount `json:"account"`
	}
	decodeCRMHTTPTestResponse(t, readResponse, &readDocument)
	if readDocument.Account.Name != "테스트 관계처" {
		t.Fatalf("account name = %q", readDocument.Account.Name)
	}
	opportunityReadResponse := crmHTTPTestRequest(
		t,
		restarted,
		http.MethodGet,
		"/crm/api/opportunities/"+opportunityDocument.Opportunity.ID,
		"teammate@example.com",
		nil,
	)
	requireCRMHTTPStatus(t, opportunityReadResponse, http.StatusOK)
	var opportunityReadDocument struct {
		Opportunity crmHTTPOpportunity `json:"opportunity"`
	}
	decodeCRMHTTPTestResponse(t, opportunityReadResponse, &opportunityReadDocument)
	if opportunityReadDocument.Opportunity.DueAt != "2026-08-10T03:00:00Z" ||
		opportunityReadDocument.Opportunity.DueTimeZone != "Asia/Seoul" {
		t.Fatalf(
			"persisted opportunity due time = %q in %q",
			opportunityReadDocument.Opportunity.DueAt,
			opportunityReadDocument.Opportunity.DueTimeZone,
		)
	}
	opportunityListResponse := crmHTTPTestRequest(t, restarted, http.MethodGet, "/crm/api/opportunities", "teammate@example.com", nil)
	requireCRMHTTPStatus(t, opportunityListResponse, http.StatusOK)
	var opportunityListDocument struct {
		Opportunities []crmHTTPOpportunity `json:"opportunities"`
	}
	decodeCRMHTTPTestResponse(t, opportunityListResponse, &opportunityListDocument)
	if len(opportunityListDocument.Opportunities) != 1 || len(opportunityListDocument.Opportunities[0].Contacts) != 1 ||
		opportunityListDocument.Opportunities[0].Contacts[0].ContactID != contactDocument.Contact.ID ||
		!opportunityListDocument.Opportunities[0].Contacts[0].IsPrimary {
		t.Fatalf("persisted opportunity list = %#v", opportunityListDocument.Opportunities)
	}
}

func TestCRMHTTPStageChangeActivityCanStayStageChangeOnlyUntilItsKindChanges(t *testing.T) {
	service := newCRMHTTPTestService(t)
	account := createCRMHTTPTestAccount(t, service, "owner@example.com", "person-owner", "team-sales")
	opportunityResponse := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities", "owner@example.com", map[string]any{
		"accountID": account.ID, "business": "Enterprise", "name": "계약", "pipeline": "sales",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, opportunityResponse, http.StatusCreated)
	var opportunityDocument struct {
		Opportunity crmHTTPOpportunity `json:"opportunity"`
	}
	decodeCRMHTTPTestResponse(t, opportunityResponse, &opportunityDocument)
	if errorValue := service.transitionCRMOpportunityStage(context.Background(), crmOpportunityStageTransition{
		OpportunityID: opportunityDocument.Opportunity.ID, Stage: "in_progress", StagePosition: 1024,
		OccurredAt: "2026-08-03T04:00:00Z", ActorPersonID: "person-owner",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	activities, errorValue := service.listCRMActivities(context.Background(), false)
	if errorValue != nil || len(activities) != 1 {
		t.Fatalf("stage change activities = %#v, error = %v", activities, errorValue)
	}
	stageChange := activities[0]
	payload := map[string]any{
		"accountID": stageChange.AccountID, "opportunityID": stageChange.OpportunityID,
		"business": stageChange.Business, "kind": "stage_change", "title": "단계 변경 메모",
		"occurredAt": stageChange.OccurredAt, "content": "세부 사항 수정",
	}
	updateResponse := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/activities/"+stageChange.ID, "owner@example.com", payload)
	requireCRMHTTPStatus(t, updateResponse, http.StatusOK)
	updatedOpportunity, _, errorValue := service.readCRMOpportunity(context.Background(), opportunityDocument.Opportunity.ID, false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if updatedOpportunity.Stage != "in_progress" || updatedOpportunity.StageChangedAt != "2026-08-03T04:00:00Z" {
		t.Fatalf("opportunity stage changed while editing activity = %#v", updatedOpportunity)
	}
	manualCreate := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/activities", "owner@example.com", payload)
	requireCRMHTTPStatus(t, manualCreate, http.StatusBadRequest)
	payload["kind"] = "note"
	requireCRMHTTPStatus(t, crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/activities/"+stageChange.ID, "owner@example.com", payload), http.StatusOK)
	payload["kind"] = "stage_change"
	requireCRMHTTPStatus(t, crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/activities/"+stageChange.ID, "owner@example.com", payload), http.StatusBadRequest)
}

func TestCRMHTTPAuthorizationAndArchiveLifecycle(t *testing.T) {
	service := newCRMHTTPTestService(t)
	account := createCRMHTTPTestAccount(t, service, "owner@example.com", "person-owner", "team-sales")
	contactResponse := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/contacts", "owner@example.com", map[string]any{
		"accountID": account.ID, "name": "담당자", "email": "contact@example.com",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, contactResponse, http.StatusCreated)
	var contactDocument struct {
		Contact crmHTTPContact `json:"contact"`
	}
	decodeCRMHTTPTestResponse(t, contactResponse, &contactDocument)

	opportunityResponse := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities", "owner@example.com", map[string]any{
		"accountID": account.ID, "name": "계약", "pipeline": "sales",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, opportunityResponse, http.StatusCreated)
	var opportunityDocument struct {
		Opportunity crmHTTPOpportunity `json:"opportunity"`
	}
	decodeCRMHTTPTestResponse(t, opportunityResponse, &opportunityDocument)

	activityResponse := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/activities", "owner@example.com", map[string]any{
		"opportunityID": opportunityDocument.Opportunity.ID, "kind": "meeting",
		"title": "미팅", "occurredAt": "2026-08-03T04:00:00Z",
	})
	requireCRMHTTPStatus(t, activityResponse, http.StatusCreated)
	var activityDocument struct {
		Activity crmHTTPActivity `json:"activity"`
	}
	decodeCRMHTTPTestResponse(t, activityResponse, &activityDocument)

	teamUpdate := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/accounts/"+account.ID, "teammate@example.com", map[string]any{
		"name": "팀 수정", "status": "active", "importance": "medium",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, teamUpdate, http.StatusOK)

	deniedUpdate := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/accounts/"+account.ID, "other@example.com", map[string]any{
		"name": "금지된 수정", "status": "active", "importance": "medium",
		"ownerPersonID": "person-other", "ownerCircleID": "team-other",
	})
	requireCRMHTTPStatus(t, deniedUpdate, http.StatusForbidden)

	deniedAccountTransfer := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/accounts/"+account.ID, "owner@example.com", map[string]any{
		"name": "금지된 소유권 이전", "status": "active", "importance": "medium",
		"ownerPersonID": "person-other", "ownerCircleID": "team-other",
	})
	requireCRMHTTPStatus(t, deniedAccountTransfer, http.StatusForbidden)

	deniedContactTransfer := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/contacts/"+contactDocument.Contact.ID, "owner@example.com", map[string]any{
		"accountID": account.ID, "name": "금지된 연락처 이전", "email": "contact@example.com",
		"ownerPersonID": "person-other", "ownerCircleID": "team-other",
	})
	requireCRMHTTPStatus(t, deniedContactTransfer, http.StatusForbidden)

	deniedOpportunityUpdate := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/opportunities/"+opportunityDocument.Opportunity.ID, "other@example.com", map[string]any{
		"accountID": account.ID, "name": "금지된 계약 수정", "pipeline": "sales",
		"ownerPersonID": "person-other", "ownerCircleID": "team-other",
	})
	requireCRMHTTPStatus(t, deniedOpportunityUpdate, http.StatusForbidden)

	deniedOpportunityTransfer := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/opportunities/"+opportunityDocument.Opportunity.ID, "owner@example.com", map[string]any{
		"accountID": account.ID, "name": "금지된 영업기회 이전", "pipeline": "sales",
		"ownerPersonID": "person-other", "ownerCircleID": "team-other",
	})
	requireCRMHTTPStatus(t, deniedOpportunityTransfer, http.StatusForbidden)

	deniedActivityUpdate := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/activities/"+activityDocument.Activity.ID, "other@example.com", map[string]any{
		"opportunityID": opportunityDocument.Opportunity.ID, "kind": "meeting",
		"title": "금지된 미팅 수정", "occurredAt": "2026-08-03T04:30:00Z",
	})
	requireCRMHTTPStatus(t, deniedActivityUpdate, http.StatusForbidden)

	ownerArchive := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/accounts/"+account.ID+"/archive", "owner@example.com", nil)
	requireCRMHTTPStatus(t, ownerArchive, http.StatusForbidden)

	adminArchive := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/accounts/"+account.ID+"/archive", "admin@example.com", nil)
	requireCRMHTTPStatus(t, adminArchive, http.StatusOK)

	hiddenRead := crmHTTPTestRequest(t, service, http.MethodGet, "/crm/api/accounts/"+account.ID, "owner@example.com", nil)
	requireCRMHTTPStatus(t, hiddenRead, http.StatusNotFound)

	archivedReadDenied := crmHTTPTestRequest(t, service, http.MethodGet, "/crm/api/accounts/"+account.ID+"?includeArchived=true", "owner@example.com", nil)
	requireCRMHTTPStatus(t, archivedReadDenied, http.StatusForbidden)

	adminRead := crmHTTPTestRequest(t, service, http.MethodGet, "/crm/api/accounts/"+account.ID+"?includeArchived=true", "admin@example.com", nil)
	requireCRMHTTPStatus(t, adminRead, http.StatusOK)

	adminRestore := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/accounts/"+account.ID+"/restore", "admin@example.com", nil)
	requireCRMHTTPStatus(t, adminRestore, http.StatusOK)

	restoredRead := crmHTTPTestRequest(t, service, http.MethodGet, "/crm/api/accounts/"+account.ID, "owner@example.com", nil)
	requireCRMHTTPStatus(t, restoredRead, http.StatusOK)

	for _, record := range []struct {
		path string
		id   string
	}{
		{path: "/crm/api/contacts/", id: contactDocument.Contact.ID},
		{path: "/crm/api/opportunities/", id: opportunityDocument.Opportunity.ID},
		{path: "/crm/api/activities/", id: activityDocument.Activity.ID},
	} {
		archiveResponse := crmHTTPTestRequest(t, service, http.MethodPost, record.path+record.id+"/archive", "admin@example.com", nil)
		requireCRMHTTPStatus(t, archiveResponse, http.StatusOK)
		hiddenResponse := crmHTTPTestRequest(t, service, http.MethodGet, record.path+record.id, "owner@example.com", nil)
		requireCRMHTTPStatus(t, hiddenResponse, http.StatusNotFound)
		restoreResponse := crmHTTPTestRequest(t, service, http.MethodPost, record.path+record.id+"/restore", "admin@example.com", nil)
		requireCRMHTTPStatus(t, restoreResponse, http.StatusOK)
	}

	unauthenticated := crmHTTPTestRequest(t, service, http.MethodGet, "/crm/api/accounts", "", nil)
	requireCRMHTTPStatus(t, unauthenticated, http.StatusUnauthorized)
}

func TestCRMHTTPValidatesInputsAndUsesDedicatedOpportunityActions(t *testing.T) {
	service := newCRMHTTPTestService(t)
	account := createCRMHTTPTestAccount(t, service, "owner@example.com", "person-owner", "team-sales")

	invalidTimeZone := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities", "owner@example.com", map[string]any{
		"accountID": account.ID, "name": "잘못된 시간대", "pipeline": "sales",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
		"dueAt": "2026-08-10T03:00:00Z", "dueTimeZone": "Not/AZone",
	})
	requireCRMHTTPStatus(t, invalidTimeZone, http.StatusBadRequest)

	terminalCreate := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities", "owner@example.com", map[string]any{
		"accountID": account.ID, "name": "직접 종결", "pipeline": "sales",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
		"amountMinor": 500000, "currencyCode": "KRW",
		"transition": map[string]any{
			"stage": "lost", "stagePosition": 1024, "occurredAt": "2026-08-03T04:30:00Z",
			"lostReason": "no_budget", "baseAmountMinor": 500000, "baseCurrencyCode": "KRW",
		},
	})
	requireCRMHTTPStatus(t, terminalCreate, http.StatusCreated)
	var terminalDocument struct {
		Opportunity crmHTTPOpportunity `json:"opportunity"`
	}
	decodeCRMHTTPTestResponse(t, terminalCreate, &terminalDocument)
	if terminalDocument.Opportunity.Stage != "lost" || terminalDocument.Opportunity.BaseAmountMinor == nil || *terminalDocument.Opportunity.BaseAmountMinor != 500000 {
		t.Fatalf("terminal opportunity = %#v", terminalDocument.Opportunity)
	}

	missingReference := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/contacts", "owner@example.com", map[string]any{
		"accountID": "missing", "name": "없는 관계처", "email": "missing@example.com",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, missingReference, http.StatusConflict)

	opportunityResponse := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities", "owner@example.com", map[string]any{
		"accountID": account.ID, "name": "단계 이동", "pipeline": "sales",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, opportunityResponse, http.StatusCreated)
	var opportunityDocument struct {
		Opportunity crmHTTPOpportunity `json:"opportunity"`
	}
	decodeCRMHTTPTestResponse(t, opportunityResponse, &opportunityDocument)

	genericStageChange := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/opportunities/"+opportunityDocument.Opportunity.ID, "owner@example.com", map[string]any{
		"accountID": account.ID, "name": "단계 우회", "pipeline": "partnership",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, genericStageChange, http.StatusConflict)

	transition := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities/"+opportunityDocument.Opportunity.ID+"/transition", "owner@example.com", map[string]any{
		"stage": "in_progress", "stagePosition": 1024, "occurredAt": "2026-08-03T05:00:00Z",
	})
	requireCRMHTTPStatus(t, transition, http.StatusOK)

	repeatedTransition := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities/"+opportunityDocument.Opportunity.ID+"/transition", "owner@example.com", map[string]any{
		"stage": "in_progress", "stagePosition": 1024, "occurredAt": "2026-08-03T05:05:00Z",
	})
	requireCRMHTTPStatus(t, repeatedTransition, http.StatusConflict)

	combinedUpdate := crmHTTPTestRequest(t, service, http.MethodPut, "/crm/api/opportunities/"+opportunityDocument.Opportunity.ID, "owner@example.com", map[string]any{
		"accountID": account.ID, "name": "원자적 종결", "pipeline": "sales",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
		"amountMinor": 700000, "currencyCode": "KRW",
		"transition": map[string]any{
			"stage": "done", "stagePosition": 1024, "occurredAt": "2026-08-03T05:06:00Z",
			"baseAmountMinor": 700000, "baseCurrencyCode": "KRW",
		},
	})
	requireCRMHTTPStatus(t, combinedUpdate, http.StatusOK)
	var combinedDocument struct {
		Opportunity crmHTTPOpportunity `json:"opportunity"`
	}
	decodeCRMHTTPTestResponse(t, combinedUpdate, &combinedDocument)
	if combinedDocument.Opportunity.Name != "원자적 종결" || combinedDocument.Opportunity.Stage != "done" {
		t.Fatalf("combined opportunity update = %#v", combinedDocument.Opportunity)
	}

	selfAnchor := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities/"+opportunityDocument.Opportunity.ID+"/position", "owner@example.com", map[string]any{
		"position": 2048, "beforeOpportunityID": opportunityDocument.Opportunity.ID, "updatedAt": "2026-08-03T05:06:00Z",
	})
	requireCRMHTTPStatus(t, selfAnchor, http.StatusConflict)

	anchorResponse := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities", "owner@example.com", map[string]any{
		"accountID": account.ID, "name": "다른 단계 기준", "pipeline": "sales",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, anchorResponse, http.StatusCreated)
	var anchorDocument struct {
		Opportunity crmHTTPOpportunity `json:"opportunity"`
	}
	decodeCRMHTTPTestResponse(t, anchorResponse, &anchorDocument)

	crossStageAnchor := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities/"+opportunityDocument.Opportunity.ID+"/position", "owner@example.com", map[string]any{
		"position": 2048, "beforeOpportunityID": anchorDocument.Opportunity.ID, "updatedAt": "2026-08-03T05:07:00Z",
	})
	requireCRMHTTPStatus(t, crossStageAnchor, http.StatusConflict)

	position := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/opportunities/"+opportunityDocument.Opportunity.ID+"/position", "owner@example.com", map[string]any{
		"position": 2048, "updatedAt": "2026-08-03T05:10:00Z",
	})
	requireCRMHTTPStatus(t, position, http.StatusOK)
}

func TestCRMHTTPReturnsInternalErrorForDatabaseFailure(t *testing.T) {
	service := newCRMHTTPTestService(t)
	if errorValue := os.WriteFile(filepath.Join(service.Configuration.StateDirectory, "crm.sqlite"), []byte("not a sqlite database"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	response := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/accounts", "owner@example.com", map[string]any{
		"name": "저장 실패", "status": "active", "ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, response, http.StatusInternalServerError)
}

func TestCRMHTTPValidatesOwnerAssignments(t *testing.T) {
	service := newCRMHTTPTestService(t)

	invalidCircle := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/accounts", "owner@example.com", map[string]any{
		"name": "잘못된 담당 팀", "status": "active",
		"ownerPersonID": "person-owner", "ownerCircleID": "team-other",
	})
	requireCRMHTTPStatus(t, invalidCircle, http.StatusBadRequest)

	invalidPerson := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/accounts", "owner@example.com", map[string]any{
		"name": "잘못된 담당자", "status": "active",
		"ownerPersonID": "missing-person", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, invalidPerson, http.StatusBadRequest)

	teamAssignment := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/accounts", "owner@example.com", map[string]any{
		"name": "같은 팀 배정", "status": "active",
		"ownerPersonID": "person-teammate", "ownerCircleID": "team-sales",
	})
	requireCRMHTTPStatus(t, teamAssignment, http.StatusCreated)

	adminAssignment := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/accounts", "admin@example.com", map[string]any{
		"name": "관리자 교차 팀 배정", "status": "active",
		"ownerPersonID": "person-other", "ownerCircleID": "team-other",
	})
	requireCRMHTTPStatus(t, adminAssignment, http.StatusCreated)
}

func TestCRMHTTPRejectsWhitespaceOnlyRequiredFields(t *testing.T) {
	service := newCRMHTTPTestService(t)
	account := createCRMHTTPTestAccount(t, service, "owner@example.com", "person-owner", "team-sales")

	requests := []struct {
		path    string
		payload map[string]any
	}{
		{path: "/crm/api/accounts", payload: map[string]any{
			"name": "   ", "status": "active", "ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
		}},
		{path: "/crm/api/contacts", payload: map[string]any{
			"accountID": account.ID, "name": "   ", "email": "contact@example.com",
			"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
		}},
		{path: "/crm/api/contacts", payload: map[string]any{
			"accountID": account.ID, "name": "담당자", "email": "   ", "phone": "   ",
			"ownerPersonID": "person-owner", "ownerCircleID": "team-sales",
		}},
		{path: "/crm/api/activities", payload: map[string]any{
			"accountID": account.ID, "kind": "note", "title": "   ", "occurredAt": "2026-08-03T04:00:00Z",
		}},
		{path: "/crm/api/activities", payload: map[string]any{
			"accountID": "   ", "kind": "note", "title": "메모", "occurredAt": "2026-08-03T04:00:00Z",
		}},
	}

	for _, request := range requests {
		response := crmHTTPTestRequest(t, service, http.MethodPost, request.path, "owner@example.com", request.payload)
		requireCRMHTTPStatus(t, response, http.StatusBadRequest)
	}
}

func TestCRMHTTPDefinitionsListsBusinessesAndRequiresAuthorization(t *testing.T) {
	service := newCRMHTTPTestService(t)

	emptyResponse := crmHTTPTestRequest(t, service, http.MethodGet, "/crm/api/definitions", "owner@example.com", nil)
	requireCRMHTTPStatus(t, emptyResponse, http.StatusOK)
	var emptyDocument struct {
		Definitions struct {
			Businesses []string `json:"businesses"`
		} `json:"definitions"`
	}
	decodeCRMHTTPTestResponse(t, emptyResponse, &emptyDocument)
	if emptyDocument.Definitions.Businesses == nil || len(emptyDocument.Definitions.Businesses) != 0 {
		t.Fatalf("businesses with no categories = %#v", emptyDocument.Definitions.Businesses)
	}

	definitions, errorValue := service.readFlowDefinitions(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	definitions.Categories = []string{"제조업", "유통업"}
	if errorValue := service.writeFlowDefinitions(context.Background(), definitions); errorValue != nil {
		t.Fatal(errorValue)
	}

	populatedResponse := crmHTTPTestRequest(t, service, http.MethodGet, "/crm/api/definitions", "owner@example.com", nil)
	requireCRMHTTPStatus(t, populatedResponse, http.StatusOK)
	var populatedDocument struct {
		Definitions struct {
			Businesses []string `json:"businesses"`
		} `json:"definitions"`
	}
	decodeCRMHTTPTestResponse(t, populatedResponse, &populatedDocument)
	if len(populatedDocument.Definitions.Businesses) != 2 ||
		populatedDocument.Definitions.Businesses[0] != "제조업" ||
		populatedDocument.Definitions.Businesses[1] != "유통업" {
		t.Fatalf("populated businesses = %#v", populatedDocument.Definitions.Businesses)
	}

	unauthorizedResponse := crmHTTPTestRequest(t, service, http.MethodGet, "/crm/api/definitions", "", nil)
	requireCRMHTTPStatus(t, unauthorizedResponse, http.StatusUnauthorized)
}

func newCRMHTTPTestService(t *testing.T) *Service {
	t.Helper()
	stateDirectory := t.TempDir()
	if errorValue := os.WriteFile(filepath.Join(stateDirectory, "users-sync.json"), []byte(`{"users":["owner@example.com","teammate@example.com","other@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(Configuration{
		StateDirectory: stateDirectory,
		DatabasePath:   filepath.Join(stateDirectory, "state.sqlite"),
		ListenAddress:  "127.0.0.1:0",
		AdminEmailPath: writeTestFile(t, "admin@example.com"),
	})
	profiles := []organizationProfile{
		{UserID: "person-owner", Email: "owner@example.com", GroupID: "team-sales", EmploymentStatus: "active"},
		{UserID: "person-teammate", Email: "teammate@example.com", GroupID: "team-sales", EmploymentStatus: "active"},
		{UserID: "person-other", Email: "other@example.com", GroupID: "team-other", EmploymentStatus: "active"},
		{UserID: "person-admin", Email: "admin@example.com", GroupID: "team-admin", EmploymentStatus: "active"},
	}
	if errorValue := service.writeOrganizationProfiles(t.Context(), profiles); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.storePolicyUserRecords([]adminUserMutation{
		{UserID: "person-owner", Email: "owner@example.com", Circles: []string{"staff", "team-sales"}, Status: "active"},
		{UserID: "person-teammate", Email: "teammate@example.com", Circles: []string{"staff", "team-sales"}, Status: "active"},
		{UserID: "person-other", Email: "other@example.com", Circles: []string{"staff", "team-other"}, Status: "active"},
		{UserID: "person-admin", Email: "admin@example.com", Circles: []string{"staff", "team-admin"}, Status: "active"},
	})
	return service
}

func createCRMHTTPTestAccount(t *testing.T, service *Service, actorEmail string, ownerPersonID string, ownerCircleID string) crmHTTPAccount {
	t.Helper()
	response := crmHTTPTestRequest(t, service, http.MethodPost, "/crm/api/accounts", actorEmail, map[string]any{
		"name": "테스트 관계처", "status": "active", "types": []string{"customer"},
		"importance": "medium", "ownerPersonID": ownerPersonID, "ownerCircleID": ownerCircleID,
	})
	requireCRMHTTPStatus(t, response, http.StatusCreated)
	var document struct {
		Account crmHTTPAccount `json:"account"`
	}
	decodeCRMHTTPTestResponse(t, response, &document)
	if document.Account.ID == "" {
		t.Fatal("account ID is empty")
	}
	return document.Account
}

func crmHTTPTestRequest(t *testing.T, service *Service, method string, target string, actorEmail string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		content, errorValue := json.Marshal(payload)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		body = bytes.NewReader(content)
	}
	request := httptest.NewRequest(method, target, body)
	request.RemoteAddr = "127.0.0.1:12345"
	if actorEmail != "" {
		request.Header.Set("X-Forwarded-Email", actorEmail)
	}
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	return response
}

func requireCRMHTTPStatus(t *testing.T, response *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if response.Code != expected {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, expected, response.Body.String())
	}
}

func decodeCRMHTTPTestResponse(t *testing.T, response *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if errorValue := json.NewDecoder(response.Body).Decode(destination); errorValue != nil {
		t.Fatal(errorValue)
	}
}
