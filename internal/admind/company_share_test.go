package admind

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompanyShareConfigurationUsesProtectedReadableJSON(t *testing.T) {
	stateDirectory := t.TempDir()
	workspaceDirectory := filepath.Join(t.TempDir(), "workspace")
	service := NewService(Configuration{StateDirectory: stateDirectory, BlueclawWorkspacePath: workspaceDirectory})
	settings := saveCompanyShareTestSettings(t, service, "share-secret")
	settings.Languages = []string{"en", "ja"}
	if errorValue := service.writeCompanyShareSettingsFile(settings); errorValue != nil {
		t.Fatal(errorValue)
	}

	configurationPath := filepath.Join(workspaceDirectory, ".protected", "company-share.json")
	document, errorValue := os.ReadFile(configurationPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(document), "passwordHash") || strings.Contains(string(document), "share-secret") {
		t.Fatalf("protected configuration exposed password material: %s", document)
	}
	configurationInfo, errorValue := os.Stat(configurationPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	directoryInfo, errorValue := os.Stat(filepath.Dir(configurationPath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configurationInfo.Mode().Perm() != 0o644 || directoryInfo.Mode().Perm() != 0o755 {
		t.Fatalf("protected modes = file %o, directory %o", configurationInfo.Mode().Perm(), directoryInfo.Mode().Perm())
	}
	accessInfo, errorValue := os.Stat(filepath.Join(stateDirectory, companyShareAccessFileName))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if accessInfo.Mode().Perm() != 0o600 {
		t.Fatalf("access state mode = %o", accessInfo.Mode().Perm())
	}
}

func TestCompanyShareSettingsAPIRejectsUnknownJSONFields(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	request := httptest.NewRequest(http.MethodPut, "/admin/api/company-share", strings.NewReader(`{
		"enabled": false,
		"sessionHours": 24,
		"languages": ["en"],
		"unknownField": true
	}`))
	response := httptest.NewRecorder()

	service.updateCompanyShareSettings(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("update status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestCompanyShareSettingsMigrateFromPrivateState(t *testing.T) {
	stateDirectory := t.TempDir()
	workspaceDirectory := filepath.Join(t.TempDir(), "workspace")
	legacyDocument := `{
		"enabled": true,
		"passwordHash": "legacy-hash",
		"accessVersion": 7,
		"sessionHours": 24,
		"profileFields": ["description"],
		"metricNames": [],
		"recordIDs": [],
		"documentIDs": []
	}`
	if errorValue := os.WriteFile(filepath.Join(stateDirectory, companyShareLegacySettingsName), []byte(legacyDocument), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(Configuration{StateDirectory: stateDirectory, BlueclawWorkspacePath: workspaceDirectory})

	settings, errorValue := service.readCompanyShareSettings()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if settings.PasswordHash != "legacy-hash" || settings.AccessVersion != 7 {
		t.Fatalf("legacy access state was not migrated: %#v", settings)
	}
	if _, errorValue := os.Stat(filepath.Join(workspaceDirectory, ".protected", companyShareConfigurationFileName)); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestCompanyShareSettingsNeverExposePasswordHash(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	request := httptest.NewRequest(http.MethodPut, "/admin/api/company-share", strings.NewReader(`{
		"enabled": true,
		"password": "share-secret",
		"sessionHours": 24,
		"profileFields": ["description", "bankAccount"],
		"metricNames": [],
		"recordIDs": []
	}`))
	response := httptest.NewRecorder()

	service.updateCompanyShareSettings(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "share-secret") || strings.Contains(response.Body.String(), "passwordHash") {
		t.Fatalf("response exposed password material: %s", response.Body.String())
	}
	settings, errorValue := service.readCompanyShareSettings()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if settings.PasswordHash == "" || settings.PasswordHash == "share-secret" {
		t.Fatalf("password was not hashed: %q", settings.PasswordHash)
	}
	if len(settings.ProfileFields) != 1 || settings.ProfileFields[0] != "description" {
		t.Fatalf("unsafe profile fields were not filtered: %#v", settings.ProfileFields)
	}
}

func TestCompanyShareSnapshotContainsOnlyPublishedProjection(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	info := companyInfo{
		Name:            localizedText{"ko": "테스트 회사", "en": "Test Company"},
		Description:     localizedText{"ko": "한국어 소개", "en": "English profile"},
		BankAccount:     localizedText{"ko": "민감한 계좌"},
		LegalAttributes: map[string]map[string]string{"ko": {"사업자번호": "000-00-00000"}},
		Email:           "company@example.com",
	}
	if errorValue := service.writeCompanyInfoFile(info); errorValue != nil {
		t.Fatal(errorValue)
	}
	insertCompanyShareTestData(t, service)
	settings := defaultCompanyShareSettings()
	settings.Languages = []string{"en", "ko"}
	settings.ProfileFields = []string{"description", "email"}
	settings.MetricNames = []string{"annualRevenue"}
	settings.RecordIDs = []string{"public-record"}
	settings.ContactEmail = "contact@example.com"

	snapshot, errorValue := service.buildCompanyShareSnapshot(t.Context(), settings, time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := json.Marshal(snapshot)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	serialized := string(document)
	for _, sensitiveValue := range []string{"민감한 계좌", "사업자번호", "000-00-00000", "internal note", "secret-attribute", "private-record"} {
		if strings.Contains(serialized, sensitiveValue) {
			t.Fatalf("snapshot exposed %q: %s", sensitiveValue, serialized)
		}
	}
	if !strings.Contains(serialized, "한국어 소개") || !strings.Contains(serialized, "annualRevenue") || !strings.Contains(serialized, `"currency":"KRW"`) || !strings.Contains(serialized, `"valueUSD":870000`) || !strings.Contains(serialized, "공개 이력") {
		t.Fatalf("snapshot omitted selected content: %s", serialized)
	}
}

func TestCompanyShareSnapshotPublishesOnlyApprovedEvidence(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	if errorValue := service.writeCompanyInfoFile(companyInfo{Name: localizedText{"ko": "테스트 회사", "en": "Test Company"}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	insertCompanyShareTestData(t, service)
	settings := defaultCompanyShareSettings()
	settings.MetricNames = []string{"annualRevenue"}
	settings.MetricContexts = map[string]companyShareMetricContext{
		"annualRevenue": {ShowSource: true, EvidenceRole: "growth"},
	}
	settings.RecordIDs = []string{"public-record"}
	settings.RecordContexts = map[string]companyShareRecordContext{
		"public-record": {
			Titles:        map[string]string{"ko": "프리시드 투자 유치", "en": "Pre-seed funding"},
			Descriptions:  map[string]string{"ko": "제품 검증을 위한 자금을 확보했습니다.", "en": "Capital secured for product validation."},
			AttributeKeys: []string{"round"},
		},
	}
	settings.DocumentIDs = []string{"public-document"}

	snapshot, errorValue := service.buildCompanyShareSnapshot(t.Context(), settings, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := json.Marshal(snapshot)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	serialized := string(document)
	for _, publicValue := range []string{"internal note", "Pre-seed funding", "Seed", "수상 확인서", "선정 근거 요약"} {
		if !strings.Contains(serialized, publicValue) {
			t.Fatalf("snapshot omitted approved evidence %q: %s", publicValue, serialized)
		}
	}
	for _, privateValue := range []string{"secret-attribute", "공개 상세", "private/path", "Private Counterpart", "requester@example.com"} {
		if strings.Contains(serialized, privateValue) {
			t.Fatalf("snapshot exposed private evidence %q: %s", privateValue, serialized)
		}
	}
}

func TestCompanyShareNarrativesAreNormalizedAndPublished(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	if errorValue := service.writeCompanyInfoFile(companyInfo{Name: localizedText{"ko": "테스트 회사", "en": "Test Company"}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	settings, errorValue := applyCompanyShareSettingsUpdate(defaultCompanyShareSettings(), companyShareSettingsUpdate{
		SessionHours: 24,
		Languages:    []string{"en", "ko"},
		Narratives: map[string]companyShareNarrative{
			"ko": {
				Highlights:        []string{"  전년 대비 매출 42% 성장  ", ""},
				BusinessModel:     "  연간 구독 모델  ",
				MarketOpportunity: "  아시아 운영 시장  ",
				FundingStage:      "  Seed  ",
			},
			"en": {Highlights: []string{"42% year-over-year revenue growth"}},
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	snapshot, errorValue := service.buildCompanyShareSnapshot(t.Context(), settings, time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if snapshot.Narratives["ko"].BusinessModel != "연간 구독 모델" || len(snapshot.Narratives["ko"].Highlights) != 1 {
		t.Fatalf("unexpected Korean narrative: %#v", snapshot.Narratives["ko"])
	}
	if snapshot.Narratives["en"].Highlights[0] != "42% year-over-year revenue growth" {
		t.Fatalf("unexpected English narrative: %#v", snapshot.Narratives["en"])
	}
}

func TestCompanyShareNarrativesRejectMoreThanThreeHighlights(t *testing.T) {
	_, errorValue := normalizeCompanyShareNarratives(map[string]companyShareNarrative{
		"ko": {Highlights: []string{"1", "2", "3", "4"}},
	}, []string{"en", "ko"})
	if errorValue == nil {
		t.Fatal("expected highlight limit error")
	}
}

func TestCompanyShareMetricContextsPreserveMeaningWithoutAssumingDirection(t *testing.T) {
	settings, errorValue := applyCompanyShareSettingsUpdate(defaultCompanyShareSettings(), companyShareSettingsUpdate{
		SessionHours:  24,
		MetricNames:   []string{"burnRate", "retention"},
		PrimaryMetric: "retention",
		MetricContexts: map[string]companyShareMetricContext{
			"burnRate": {
				Labels:             map[string]string{"ko": "월 소진액", "en": "Monthly burn"},
				Descriptions:       map[string]string{"ko": "월별 순현금 지출"},
				FavorableDirection: "decrease",
				EvidenceRole:       "efficiency",
				ShowSource:         true,
			},
			"retention": {FavorableDirection: "increase"},
			"unused":    {FavorableDirection: "increase"},
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if settings.PrimaryMetric != "retention" || len(settings.MetricContexts) != 2 {
		t.Fatalf("unexpected metric presentation: %#v", settings)
	}
	if settings.MetricContexts["burnRate"].FavorableDirection != "decrease" {
		t.Fatalf("unexpected burn direction: %#v", settings.MetricContexts["burnRate"])
	}
	if settings.MetricContexts["burnRate"].EvidenceRole != "efficiency" || !settings.MetricContexts["burnRate"].ShowSource {
		t.Fatalf("unexpected burn evidence context: %#v", settings.MetricContexts["burnRate"])
	}

	_, errorValue = applyCompanyShareSettingsUpdate(defaultCompanyShareSettings(), companyShareSettingsUpdate{
		SessionHours: 24, MetricNames: []string{"retention"}, PrimaryMetric: "not-published",
	})
	if errorValue == nil {
		t.Fatal("expected invalid primary metric error")
	}
}

func TestCompanySharePasswordRotationInvalidatesSession(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	settings := saveCompanyShareTestSettings(t, service, "first-password")
	snapshot := companyShareSnapshot{Revision: 1, PublishedAt: time.Now().UTC().Format(time.RFC3339), Profiles: map[string]companyShareProfile{"ko": {Name: "테스트 회사"}}}
	if errorValue := service.writeCompanyShareSnapshotFile(snapshot); errorValue != nil {
		t.Fatal(errorValue)
	}
	settings.PublishedAt = snapshot.PublishedAt
	settings.PublicationRevision = 1
	if errorValue := service.writeCompanyShareSettingsFile(settings); errorValue != nil {
		t.Fatal(errorValue)
	}

	unlockRequest := httptest.NewRequest(http.MethodPost, "/company/api/unlock", strings.NewReader(`{"password":"first-password"}`))
	unlockRequest.RemoteAddr = "198.51.100.5:443"
	unlockResponse := httptest.NewRecorder()
	service.unlockCompanyShare(unlockResponse, unlockRequest)
	if unlockResponse.Code != http.StatusOK {
		t.Fatalf("unlock status = %d, body = %s", unlockResponse.Code, unlockResponse.Body.String())
	}
	cookie := unlockResponse.Result().Cookies()[0]
	contentRequest := httptest.NewRequest(http.MethodGet, "/company/api/content", nil)
	contentRequest.AddCookie(cookie)
	contentResponse := httptest.NewRecorder()
	service.writeCompanyShareContent(contentResponse, contentRequest)
	if contentResponse.Code != http.StatusOK {
		t.Fatalf("content status = %d, body = %s", contentResponse.Code, contentResponse.Body.String())
	}

	rotated := saveCompanyShareTestSettings(t, service, "second-password")
	rotated.PublishedAt = snapshot.PublishedAt
	rotated.PublicationRevision = 1
	if errorValue := service.writeCompanyShareSettingsFile(rotated); errorValue != nil {
		t.Fatal(errorValue)
	}
	invalidatedResponse := httptest.NewRecorder()
	service.writeCompanyShareContent(invalidatedResponse, contentRequest)
	if invalidatedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("rotated session status = %d, body = %s", invalidatedResponse.Code, invalidatedResponse.Body.String())
	}
}

func TestCompanyShareUnlockRateLimit(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	settings := saveCompanyShareTestSettings(t, service, "correct-password")
	settings.PublishedAt = time.Now().UTC().Format(time.RFC3339)
	if errorValue := service.writeCompanyShareSettingsFile(settings); errorValue != nil {
		t.Fatal(errorValue)
	}
	for attempt := 0; attempt < companyShareAttemptLimit; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/company/api/unlock", bytes.NewBufferString(`{"password":"wrong-password"}`))
		request.RemoteAddr = "203.0.113.8:443"
		response := httptest.NewRecorder()
		service.unlockCompanyShare(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", attempt, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/company/api/unlock", bytes.NewBufferString(`{"password":"correct-password"}`))
	request.RemoteAddr = "203.0.113.8:443"
	response := httptest.NewRecorder()
	service.unlockCompanyShare(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestCompanyShareTeamActivityPublishesLimitedIdentityAndTaskTitles(t *testing.T) {
	temporaryDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:         temporaryDirectory,
		FlowDatabasePath:       temporaryDirectory + "/flow.sqlite",
		AttendanceDatabasePath: temporaryDirectory + "/attendance.sqlite",
	})
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{TimeZone: "Asia/Seoul", Language: workspaceLanguageEnglish}); errorValue != nil {
		t.Fatal(errorValue)
	}
	insertCompanyShareActivityTestData(t, service)

	activity, errorValue := service.buildCompanyShareTeamActivity(t.Context(), time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if activity.AttendanceTotal != 2 || activity.WorkTotal != 2 || len(activity.Members) != 2 {
		t.Fatalf("unexpected activity aggregate: %#v", activity)
	}
	if activity.Days[len(activity.Days)-2].WorkMinutes != 60 || activity.Days[len(activity.Days)-1].WorkMinutes != 300 {
		t.Fatalf("overnight work minutes were not split by date: %#v", activity.Days[len(activity.Days)-2:])
	}
	if len(activity.RecentWork) != 2 || activity.RecentWork[0].MemberSeed == "" || activity.RecentWork[0].Title == "" {
		t.Fatalf("unexpected recent work: %#v", activity.RecentWork)
	}
	if activity.WorkStatuses[0].Status != "completed" || activity.WorkStatuses[len(activity.WorkStatuses)-1].Status != "inProgress" {
		t.Fatalf("unexpected work status order: %#v", activity.WorkStatuses)
	}
	document, errorValue := json.Marshal(activity)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	serialized := string(document)
	for _, publicValue := range []string{"김", "이", "고객 대시보드 개선", "/company/api/team/"} {
		if !strings.Contains(serialized, publicValue) {
			t.Fatalf("activity omitted %q: %s", publicValue, serialized)
		}
	}
	for _, privateValue := range []string{"member@example.com", "second@example.com", "김철수", "이영희", "09:00"} {
		if strings.Contains(serialized, privateValue) {
			t.Fatalf("activity exposed %q: %s", privateValue, serialized)
		}
	}
}

func TestPublicCompanyShareSurname(t *testing.T) {
	testCases := map[string]string{
		"김철수":        "김",
		"남궁민":        "남궁",
		"Suhyun Lee": "Lee",
	}
	for name, expected := range testCases {
		if actual := publicCompanyShareSurname(name); actual != expected {
			t.Fatalf("publicCompanyShareSurname(%q) = %q, want %q", name, actual, expected)
		}
	}
}

func TestCompanyShareWorkStatusesPlacesPlannedLast(t *testing.T) {
	statuses := companyShareWorkStatuses(map[string]int{"completed": 3, "inProgress": 2, "planned": 1})
	if statuses[0].Status != "completed" || statuses[1].Status != "inProgress" || statuses[2].Status != "planned" {
		t.Fatalf("unexpected work status order: %#v", statuses)
	}
}

func saveCompanyShareTestSettings(t *testing.T, service *Service, password string) companyShareSettings {
	t.Helper()
	current, errorValue := service.readCompanyShareSettings()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	settings, errorValue := applyCompanyShareSettingsUpdate(current, companyShareSettingsUpdate{
		Enabled: true, Password: password, SessionHours: 24, ProfileFields: defaultCompanyShareFields,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeCompanyShareSettingsFile(settings); errorValue != nil {
		t.Fatal(errorValue)
	}
	return settings
}

func insertCompanyShareTestData(t *testing.T, service *Service) {
	t.Helper()
	database, errorValue := service.openCompanyDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(t.Context(), `
INSERT INTO company_metrics (metric, year, quarter, month, value, currency, value_usd, unit, note, updated_at) VALUES
('annualRevenue', 2025, 0, 0, 1200000000, 'KRW', 870000, '', 'internal note', '2026-01-01T00:00:00Z'),
('mau', 2025, 0, 12, 9000, '', NULL, '명', 'do not publish', '2026-01-01T00:00:00Z');
INSERT INTO company_records (id, category, record_date, title, detail, attributes, updated_at) VALUES
('public-record', 'milestone', '2025-12-01', '공개 이력', '공개 상세', '{"round":"Seed","secret":"secret-attribute"}', '2026-01-01T00:00:00Z'),
('private-record', 'funding', '2025-11-01', '비공개 이력', 'private-record', '{}', '2026-01-01T00:00:00Z');
INSERT INTO company_documents (id, document_number, kind, document_type, title, counterpart, language, file_path, summary, summary_embedding, requester_email, issued_at, updated_at) VALUES
('public-document', 'AWD-2025-001', 'received', 'award-certificate', '수상 확인서', 'Private Counterpart', 'ko', 'private/path', '선정 근거 요약', '', 'requester@example.com', '2025-12-02T00:00:00Z', '2025-12-02T00:00:00Z')`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func insertCompanyShareActivityTestData(t *testing.T, service *Service) {
	t.Helper()
	attendanceDatabase, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = attendanceDatabase.ExecContext(t.Context(), `
INSERT INTO attendance_events (
	id, mattermost_user_id, mattermost_username, email, display_name, kind, occurred_at, local_date, local_time,
	time_zone_at_event, source, team_id, channel_id, action_post_id, result_post_id, location_id, location_name,
	canceled_at, cancel_reason, repeated_click_at
) VALUES
('attendance-1', 'member-1', 'member', 'member@example.com', '김철수', 'clock_in', '2026-07-13T14:00:00Z', '2026-07-13', '23:00', 'Asia/Seoul', 'test', '', '', '', '', '', '', '', '', ''),
('attendance-1-out', 'member-1', 'member', 'member@example.com', '김철수', 'clock_out', '2026-07-13T17:00:00Z', '2026-07-13', '02:00', 'Asia/Seoul', 'test', '', '', '', '', '', '', '', '', ''),
('attendance-2', 'member-2', 'second', 'second@example.com', '이영희', 'clock_in', '2026-07-14T00:00:00Z', '2026-07-14', '09:00', 'Asia/Seoul', 'test', '', '', '', '', '', '', '', '', ''),
('attendance-2-out', 'member-2', 'second', 'second@example.com', '이영희', 'clock_out', '2026-07-14T03:00:00Z', '2026-07-14', '12:00', 'Asia/Seoul', 'test', '', '', '', '', '', '', '', '', '')`)
	attendanceDatabase.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	flowDatabase, errorValue := service.openFlowDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer flowDatabase.Close()
	_, errorValue = flowDatabase.ExecContext(t.Context(), `
INSERT INTO flow_tasks (
	id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size,
	status, status_rank, start_date, end_date, flag, request_reason, decision_reason, created_at, updated_at
) VALUES
('task-1', '2026-W29', 'member@example.com', '김철수', '[]', '[]', 'secret', 'secret', '온보딩 흐름 정리', '', 'S', '진행', 1, '2026-07-13', '', 0, '', '', '2026-07-13T00:00:00Z', '2026-07-13T10:00:00Z'),
('task-2', '2026-W29', 'second@example.com', '이영희', '[]', '[]', 'secret', 'secret', '고객 대시보드 개선', '', 'S', '완료', 2, '2026-07-12', '2026-07-14', 0, '', '', '2026-07-12T00:00:00Z', '2026-07-14T10:00:00Z')`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}
