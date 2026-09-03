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
	service := newCompanyShareProfileService(t, map[string]string{
		"ko": `{"name":"테스트 회사","description":"한국어 소개","bankAccount":"민감한 계좌","legalAttributes":[{"label":"사업자번호","value":"000-00-00000"}],"email":"company@example.com"}`,
		"en": `{"name":"Test Company","description":"English profile","email":"company@example.com"}`,
	})
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
	service := newCompanyShareProfileService(t, map[string]string{"ko": `{"name":"테스트 회사"}`, "en": `{"name":"Test Company"}`})
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
	for _, privateValue := range []string{"secret-attribute", "공개 상세", "private/path", "Private Counterpart", "member-private"} {
		if strings.Contains(serialized, privateValue) {
			t.Fatalf("snapshot exposed private evidence %q: %s", privateValue, serialized)
		}
	}
}

func TestCompanyShareNarrativesAreNormalizedAndPublished(t *testing.T) {
	service := newCompanyShareProfileService(t, map[string]string{"ko": `{"name":"테스트 회사"}`, "en": `{"name":"Test Company"}`})
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

func TestCompanyShareTeamActivityPublishesTaskTitlesAndNoAddresses(t *testing.T) {
	service := newCompanyShareCompanyService(t)
	useCompanyForTest(service, startCompanyHoldingTwoTasks(t).URL)
	writeClaimedAdminEmailForTest(t, service, "admin@example.com")

	activity, errorValue := service.buildCompanyShareTeamActivity(t.Context(), time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if activity.WorkTotal != 2 || len(activity.Members) != 2 {
		t.Fatalf("unexpected activity aggregate: %#v", activity)
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
	if !strings.Contains(serialized, "고객 대시보드 개선") {
		t.Fatalf("activity omitted a task title: %s", serialized)
	}
	for _, privateValue := range []string{"member@example.com", "second@example.com"} {
		if strings.Contains(serialized, privateValue) {
			t.Fatalf("activity exposed %q: %s", privateValue, serialized)
		}
	}
}

func startCompanyHoldingTwoTasks(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/api/agent/session" {
			responseWriter.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":4102444800}`))
			return
		}
		if !strings.HasPrefix(request.URL.Path, "/rest/v1/task") {
			responseWriter.Write([]byte(`[]`))
			return
		}
		responseWriter.Write([]byte(`[` +
			`{"id":"11111111-1111-4111-8111-111111111111","title":"온보딩 흐름 정리","status":"in_progress",` +
			`"business":"secret","type":"secret","size":"S","starts_at":"2026-07-13T00:00:00+00:00",` +
			`"created_at":"2026-07-13T00:00:00+00:00","updated_at":"2026-07-13T10:00:00+00:00",` +
			`"task_participant":[{"member":{"email":"member@example.com"}}]},` +
			`{"id":"22222222-2222-4222-8222-222222222222","title":"고객 대시보드 개선","status":"completed",` +
			`"business":"secret","type":"secret","size":"S","starts_at":"2026-07-12T00:00:00+00:00",` +
			`"ends_at":"2026-07-14T00:00:00+00:00","created_at":"2026-07-12T00:00:00+00:00",` +
			`"updated_at":"2026-07-14T10:00:00+00:00",` +
			`"task_participant":[{"member":{"email":"second@example.com"}}]}]`))
	}))
	t.Cleanup(server.Close)
	return server
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

// The company is the only attendance store, so the clocks this panel counts
// come back from the record rather than from a table on the device.
func companyShareRecordHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	if strings.HasSuffix(request.URL.Path, "/api/agent/session") {
		writer.Write([]byte(`{"memberID":"admin","accessToken":"token","expiresAt":99999999999}`))
		return
	}
	if strings.Contains(request.URL.Path, "/rest/v1/attendance") {
		writer.Write([]byte(`[
			{"kind":"clock_in","occurred_at":"2026-07-13T14:00:00Z","member":{"email":"member@example.com","name":"김철수"}},
			{"kind":"clock_out","occurred_at":"2026-07-13T17:00:00Z","member":{"email":"member@example.com","name":"김철수"}},
			{"kind":"clock_in","occurred_at":"2026-07-14T00:00:00Z","member":{"email":"second@example.com","name":"이영희"}},
			{"kind":"clock_out","occurred_at":"2026-07-14T03:00:00Z","member":{"email":"second@example.com","name":"이영희"}}
		]`))
		return
	}
	writer.Write([]byte(`[]`))
}

func TestCompanyShareCountsTheClocksTheCompanyHolds(t *testing.T) {
	temporaryDirectory := t.TempDir()
	plane := httptest.NewServer(http.HandlerFunc(companyShareRecordHandler))
	defer plane.Close()
	service := NewService(Configuration{
		StateDirectory:             temporaryDirectory,
		TaskDatabasePath:           temporaryDirectory + "/flow.sqlite",
		ClaimedAdminEmailPath:      writeTestFile(t, "admin@example.com"),
		CentralPlaneAppURL:         plane.URL,
		CentralPlaneProjectURL:     plane.URL,
		CentralPlanePublishableKey: "publishable",
		CentralPlaneAgentKeyPath:   writeAgentKeyForTest(t, "agent-key"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageEnglish)

	activity, errorValue := service.buildCompanyShareTeamActivity(t.Context(), time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if activity.AttendanceTotal != 2 {
		t.Fatalf("the record holds two days of clocks: %#v", activity)
	}
	if activity.Days[len(activity.Days)-2].WorkMinutes != 60 || activity.Days[len(activity.Days)-1].WorkMinutes != 300 {
		t.Fatalf("overnight work minutes were not split by date: %#v", activity.Days[len(activity.Days)-2:])
	}
	document, errorValue := json.Marshal(activity)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, privateValue := range []string{"member@example.com", "second@example.com", "김철수", "이영희"} {
		if strings.Contains(string(document), privateValue) {
			t.Fatalf("the panel exposed %q", privateValue)
		}
	}
}

const (
	companyShareLedgerMetrics = `{"count":2,"metrics":[` +
		`{"metricID":"metric-revenue","metric":"annualRevenue","year":2025,"quarter":0,"month":0,"value":1200000000,` +
		`"currency":"KRW","valueUSD":870000,"unit":null,"note":"internal note","updatedAt":"2026-01-01T00:00:00Z"},` +
		`{"metricID":"metric-mau","metric":"mau","year":2025,"quarter":0,"month":12,"value":9000,` +
		`"currency":null,"valueUSD":null,"unit":"\uba85","note":"do not publish","updatedAt":"2026-01-01T00:00:00Z"}]}`
	companyShareLedgerRecords = `{"count":2,"records":[` +
		`{"recordID":"public-record","category":"milestone","date":"2025-12-01","title":"\uacf5\uac1c \uc774\ub825",` +
		`"detail":"\uacf5\uac1c \uc0c1\uc138","attributes":[{"label":"round","value":"Seed"},{"label":"secret","value":"secret-attribute"}],"updatedAt":"2026-01-01T00:00:00Z"},` +
		`{"recordID":"private-record","category":"funding","date":"2025-11-01","title":"\ube44\uacf5\uac1c \uc774\ub825",` +
		`"detail":"private-record","attributes":[],"updatedAt":"2026-01-01T00:00:00Z"}]}`
	companyShareLedgerDocuments = `{"count":1,"documents":[` +
		`{"documentID":"public-document","documentNumber":"AWD-2025-001","kind":"received","documentType":"award-certificate",` +
		`"title":"\uc218\uc0c1 \ud655\uc778\uc11c","counterpart":"Private Counterpart","language":"ko","filePath":"private/path",` +
		`"summary":"\uc120\uc815 \uadfc\uac70 \uc694\uc57d","requesterID":"member-private","issuedAt":"2025-12-02T00:00:00Z"}]}`
)

func newCompanyShareProfileService(t *testing.T, profilesByLanguage map[string]string) *Service {
	t.Helper()
	company := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/agent/session":
			_, _ = responseWriter.Write([]byte(`{"memberID":"member-admin","accessToken":"token","expiresAt":4102444800}`))
		case "/api/v1/tools/company_info_get/invoke":
			var payload struct {
				Input struct {
					Language string `json:"language"`
				} `json:"input"`
			}
			_ = json.NewDecoder(request.Body).Decode(&payload)
			profile, isAnswered := profilesByLanguage[payload.Input.Language]
			if !isAnswered {
				profile = `{}`
			}
			_, _ = responseWriter.Write([]byte(`{"tool":"company_info_get","result":` + profile + `}`))
		case "/api/v1/tools/company_metric_list/invoke":
			_, _ = responseWriter.Write([]byte(`{"tool":"company_metric_list","result":` + companyShareLedgerMetrics + `}`))
		case "/api/v1/tools/company_record_list/invoke":
			_, _ = responseWriter.Write([]byte(`{"tool":"company_record_list","result":` + companyShareLedgerRecords + `}`))
		case "/api/v1/tools/company_document_list/invoke":
			_, _ = responseWriter.Write([]byte(`{"tool":"company_document_list","result":` + companyShareLedgerDocuments + `}`))
		default:
			responseWriter.WriteHeader(http.StatusNotFound)
			_, _ = responseWriter.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(company.Close)
	service := NewService(Configuration{
		StateDirectory:             t.TempDir(),
		ClaimedAdminEmailPath:      writeTestFile(t, "admin@example.com"),
		CentralPlaneAppURL:         company.URL,
		CentralPlaneProjectURL:     company.URL,
		CentralPlanePublishableKey: "publishable",
		CentralPlaneAgentKeyPath:   writeAgentKeyForTest(t, "agent-key"),
	})
	return service
}
