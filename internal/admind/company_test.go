package admind

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newCompanyTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(Configuration{StateDirectory: t.TempDir()})
}

func performCompanyRequest(t *testing.T, handler func(http.ResponseWriter, *http.Request), method string, target string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *bytes.Reader
	if payload != nil {
		document, errorValue := json.Marshal(payload)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		body = bytes.NewReader(document)
	} else {
		body = bytes.NewReader(nil)
	}
	request := httptest.NewRequest(method, target, body)
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder
}

func decodeCompanyResponse(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), target); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestCompanyInfoPartialUpdateAndLanguageFallback(t *testing.T) {
	service := newCompanyTestService(t)

	var koView companyInfoView
	decodeCompanyResponse(t, performCompanyRequest(t, service.updateCompanyInfo, http.MethodPut, "/admin/api/company-info", map[string]any{
		"language":       "ko",
		"name":           "주식회사 여명거리",
		"representative": "김여명",
		"address":        "서울특별시 강남구",
		"bankAccount":    "신한은행 110-123",
		"phone":          "02-1234-5678",
		"email":          "contact@example.com",
		"legalAttributes": map[string]string{
			"사업자등록번호": "123-45-67890",
		},
	}), &koView)
	if len(koView.MissingFields) != 0 {
		t.Fatalf("ko missingFields = %v, want empty", koView.MissingFields)
	}
	if koView.LegalAttributes["사업자등록번호"] != "123-45-67890" {
		t.Fatalf("legalAttributes = %v", koView.LegalAttributes)
	}
	if koView.RepresentativeTitle != "대표이사" {
		t.Fatalf("representativeTitle = %q", koView.RepresentativeTitle)
	}

	var partialView companyInfoView
	decodeCompanyResponse(t, performCompanyRequest(t, service.updateCompanyInfo, http.MethodPut, "/admin/api/company-info", map[string]any{
		"language": "ko",
		"address":  "부산광역시 해운대구",
	}), &partialView)
	if partialView.Name != "주식회사 여명거리" || partialView.Address != "부산광역시 해운대구" {
		t.Fatalf("partial update broke other fields: name=%q address=%q", partialView.Name, partialView.Address)
	}

	var enView companyInfoView
	decodeCompanyResponse(t, performCompanyRequest(t, service.writeCompanyInfo, http.MethodGet, "/admin/api/company-info?language=en", nil), &enView)
	if enView.Name != "주식회사 여명거리" {
		t.Fatalf("en fallback name = %q", enView.Name)
	}
	missing := strings.Join(enView.MissingFields, ",")
	for _, field := range []string{"name", "representative", "address", "bankAccount"} {
		if !strings.Contains(missing, field) {
			t.Fatalf("en missingFields = %v, want %s reported", enView.MissingFields, field)
		}
	}
	if strings.Contains(missing, "phone") || strings.Contains(missing, "email") {
		t.Fatalf("neutral fields wrongly missing: %v", enView.MissingFields)
	}
	if enView.RepresentativeTitle != "CEO" {
		t.Fatalf("en representativeTitle = %q", enView.RepresentativeTitle)
	}
}

func TestCompanyMetricUpsertAndGranularityRejection(t *testing.T) {
	service := newCompanyTestService(t)

	record := func(payload map[string]any) *httptest.ResponseRecorder {
		return performCompanyRequest(t, service.recordCompanyMetric, http.MethodPost, "/admin/api/company-metrics", payload)
	}
	var recorded companyMetric
	decodeCompanyResponse(t, record(map[string]any{"metric": "annualRevenue", "year": 2025, "value": 1000000000, "currency": "KRW", "valueUSD": 720000}), &recorded)
	decodeCompanyResponse(t, record(map[string]any{"metric": "annualRevenue", "year": 2025, "value": 1200000000, "currency": "KRW", "valueUSD": 870000}), &recorded)

	rejected := record(map[string]any{"metric": "mau", "year": 2025, "quarter": 2, "month": 6, "value": 10})
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("quarter+month status = %d, want 400", rejected.Code)
	}

	var listing struct {
		Metrics []companyMetric `json:"metrics"`
	}
	decodeCompanyResponse(t, performCompanyRequest(t, service.listCompanyMetrics, http.MethodGet, "/admin/api/company-metrics?metric=annualRevenue", nil), &listing)
	if len(listing.Metrics) != 1 || listing.Metrics[0].Value != 1200000000 || listing.Metrics[0].Currency != companyMetricCurrencyKRW {
		t.Fatalf("metrics = %+v, want single upserted row", listing.Metrics)
	}
	if listing.Metrics[0].ValueUSD == nil || *listing.Metrics[0].ValueUSD != 870000 {
		t.Fatalf("valueUSD = %v, want 870000", listing.Metrics[0].ValueUSD)
	}
}

func TestCompanyMetricMoneyValidation(t *testing.T) {
	service := newCompanyTestService(t)
	record := func(payload map[string]any) *httptest.ResponseRecorder {
		return performCompanyRequest(t, service.recordCompanyMetric, http.MethodPost, "/admin/api/company-metrics", payload)
	}

	for name, payload := range map[string]map[string]any{
		"missing USD value":    {"metric": "revenue", "year": 2025, "value": 1, "currency": "KRW"},
		"unsupported currency": {"metric": "revenue", "year": 2025, "value": 1, "currency": "BTC", "valueUSD": 1},
		"currency and unit":    {"metric": "revenue", "year": 2025, "value": 1, "currency": "USD", "unit": "dollars"},
	} {
		t.Run(name, func(t *testing.T) {
			response := record(payload)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}

	var recorded companyMetric
	decodeCompanyResponse(t, record(map[string]any{"metric": "revenue", "year": 2025, "value": 42, "currency": "usd"}), &recorded)
	if recorded.Currency != companyMetricCurrencyUSD || recorded.ValueUSD == nil || *recorded.ValueUSD != 42 {
		t.Fatalf("USD normalization = %+v", recorded)
	}
}

func TestCompanyMetricSchemaMigratesLegacyDatabase(t *testing.T) {
	service := newCompanyTestService(t)
	legacyDatabase, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(service.companyDatabasePath()))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = legacyDatabase.Exec(`
CREATE TABLE company_metrics (
	metric TEXT NOT NULL,
	year INTEGER NOT NULL,
	quarter INTEGER NOT NULL DEFAULT 0,
	month INTEGER NOT NULL DEFAULT 0,
	value REAL NOT NULL,
	unit TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	PRIMARY KEY (metric, year, quarter, month)
);
INSERT INTO company_metrics (metric, year, value, unit, updated_at)
VALUES ('sites', 2025, 63, '곳', '2026-01-01T00:00:00Z')`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := legacyDatabase.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	database, errorValue := service.openCompanyDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var metric companyMetric
	errorValue = database.QueryRowContext(t.Context(), `SELECT metric, year, value, currency, value_usd, unit FROM company_metrics`).Scan(
		&metric.Metric, &metric.Year, &metric.Value, &metric.Currency, &metric.ValueUSD, &metric.Unit,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if metric.Metric != "sites" || metric.Value != 63 || metric.Currency != "" || metric.ValueUSD != nil || metric.Unit != "곳" {
		t.Fatalf("migrated metric = %+v", metric)
	}
}

func TestCompanyDocumentNumberingPerTypeAndYear(t *testing.T) {
	service := newCompanyTestService(t)

	register := func(documentType string, kind string) companyDocument {
		var document companyDocument
		decodeCompanyResponse(t, performCompanyRequest(t, service.registerCompanyDocument, http.MethodPost, "/admin/api/company-documents", map[string]any{
			"documentType": documentType,
			"title":        documentType + " 테스트",
			"kind":         kind,
			"summary":      "테스트 요약",
		}), &document)
		return document
	}

	first := register("quote", "issued")
	second := register("quote", "issued")
	other := register("service-agreement", "issued")
	received := register("quote", "received")

	if !strings.HasPrefix(first.DocumentNumber, "Q-") || !strings.HasSuffix(first.DocumentNumber, "-001") {
		t.Fatalf("first number = %q", first.DocumentNumber)
	}
	if !strings.HasSuffix(second.DocumentNumber, "-002") {
		t.Fatalf("second number = %q", second.DocumentNumber)
	}
	if !strings.HasPrefix(other.DocumentNumber, "SVC-") || !strings.HasSuffix(other.DocumentNumber, "-001") {
		t.Fatalf("other type number = %q", other.DocumentNumber)
	}
	if received.DocumentNumber != "" {
		t.Fatalf("received document got number %q", received.DocumentNumber)
	}
	if first.StorageDirectory != "/workspace/circles/staff/documents/quote" {
		t.Fatalf("storageDirectory = %q", first.StorageDirectory)
	}
}

func TestCompanyRecordLifecycleAndDocumentTextSearch(t *testing.T) {
	service := newCompanyTestService(t)

	var record companyRecord
	decodeCompanyResponse(t, performCompanyRequest(t, service.addCompanyRecord, http.MethodPost, "/admin/api/company-records", map[string]any{
		"category":   "funding",
		"date":       "2025-11-01",
		"title":      "시드 투자 유치",
		"attributes": map[string]string{"amount": "20억 원", "round": "Seed"},
	}), &record)

	decodeCompanyResponse(t, performCompanyRequest(t, service.updateCompanyRecord, http.MethodPut, "/admin/api/company-records", map[string]any{
		"id":    record.ID,
		"title": "시드 라운드 투자 유치",
	}), &map[string]any{})

	var listing struct {
		Records []companyRecord `json:"records"`
	}
	decodeCompanyResponse(t, performCompanyRequest(t, service.listCompanyRecords, http.MethodGet, "/admin/api/company-records?category=funding", nil), &listing)
	if len(listing.Records) != 1 || listing.Records[0].Title != "시드 라운드 투자 유치" {
		t.Fatalf("records = %+v", listing.Records)
	}

	var registered companyDocument
	decodeCompanyResponse(t, performCompanyRequest(t, service.registerCompanyDocument, http.MethodPost, "/admin/api/company-documents", map[string]any{
		"documentType": "service-agreement",
		"title":        "ABC물산 용역계약서",
		"counterpart":  "ABC물산",
		"summary":      "월 500만 원, 3개월 컨설팅 용역 계약",
	}), &registered)

	var found struct {
		Documents []companyDocument `json:"documents"`
		MatchKind string            `json:"matchKind"`
	}
	decodeCompanyResponse(t, performCompanyRequest(t, service.searchCompanyDocuments, http.MethodPost, "/admin/api/company-documents/search", map[string]any{
		"query": "용역",
	}), &found)
	if found.MatchKind != "text" || len(found.Documents) != 1 {
		t.Fatalf("search result = %+v", found)
	}

	decodeCompanyResponse(t, performCompanyRequest(t, service.updateCompanyDocument, http.MethodPut, "/admin/api/company-documents", map[string]any{
		"id":       registered.ID,
		"filePath": "/workspace/circles/staff/documents/service-agreement/renamed.docx",
	}), &map[string]any{})

	var documents struct {
		Documents []companyDocument `json:"documents"`
	}
	decodeCompanyResponse(t, performCompanyRequest(t, service.listCompanyDocuments, http.MethodGet, "/admin/api/company-documents?counterpart=ABC", nil), &documents)
	if len(documents.Documents) != 1 || documents.Documents[0].FilePath != "/workspace/circles/staff/documents/service-agreement/renamed.docx" {
		t.Fatalf("documents = %+v", documents.Documents)
	}
}
