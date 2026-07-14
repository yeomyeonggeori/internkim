package admind

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	companyShareCookieName       = "internkim_company_share"
	companyShareSecretName       = "company-share-session-secret"
	companyShareSettingsFileName = "company-share-settings.json"
	companyShareSnapshotFileName = "company-share-snapshot.json"
	companyShareAttemptWindow    = 15 * time.Minute
	companyShareAttemptLimit     = 5
)

var allowedCompanyShareFields = map[string]bool{
	"brandName":           true,
	"slogan":              true,
	"description":         true,
	"website":             true,
	"foundedDate":         true,
	"employeeCount":       true,
	"jurisdiction":        true,
	"representative":      true,
	"representativeTitle": true,
	"capital":             true,
	"fiscalYearEnd":       true,
	"email":               true,
}

var defaultCompanyShareFields = []string{
	"brandName", "slogan", "description", "website", "foundedDate", "employeeCount", "jurisdiction",
}

type companyShareSettings struct {
	Enabled             bool                                 `json:"enabled"`
	PasswordHash        string                               `json:"passwordHash,omitempty"`
	AccessVersion       int                                  `json:"accessVersion"`
	SessionHours        int                                  `json:"sessionHours"`
	ProfileFields       []string                             `json:"profileFields"`
	MetricNames         []string                             `json:"metricNames"`
	PrimaryMetric       string                               `json:"primaryMetric,omitempty"`
	MetricContexts      map[string]companyShareMetricContext `json:"metricContexts"`
	RecordIDs           []string                             `json:"recordIDs"`
	RecordContexts      map[string]companyShareRecordContext `json:"recordContexts"`
	DocumentIDs         []string                             `json:"documentIDs"`
	ContactEmail        string                               `json:"contactEmail,omitempty"`
	ShowTeamActivity    bool                                 `json:"showTeamActivity"`
	Narratives          map[string]companyShareNarrative     `json:"narratives"`
	PublishedAt         string                               `json:"publishedAt,omitempty"`
	PublicationRevision int                                  `json:"publicationRevision"`
}

type companyShareSettingsResponse struct {
	Enabled             bool                                 `json:"enabled"`
	HasPassword         bool                                 `json:"hasPassword"`
	SessionHours        int                                  `json:"sessionHours"`
	ProfileFields       []string                             `json:"profileFields"`
	MetricNames         []string                             `json:"metricNames"`
	PrimaryMetric       string                               `json:"primaryMetric,omitempty"`
	MetricContexts      map[string]companyShareMetricContext `json:"metricContexts"`
	RecordIDs           []string                             `json:"recordIDs"`
	RecordContexts      map[string]companyShareRecordContext `json:"recordContexts"`
	DocumentIDs         []string                             `json:"documentIDs"`
	ContactEmail        string                               `json:"contactEmail,omitempty"`
	ShowTeamActivity    bool                                 `json:"showTeamActivity"`
	Narratives          map[string]companyShareNarrative     `json:"narratives"`
	PublishedAt         string                               `json:"publishedAt,omitempty"`
	PublicationRevision int                                  `json:"publicationRevision"`
}

type companyShareSettingsUpdate struct {
	Enabled          bool                                 `json:"enabled"`
	Password         string                               `json:"password"`
	SessionHours     int                                  `json:"sessionHours"`
	ProfileFields    []string                             `json:"profileFields"`
	MetricNames      []string                             `json:"metricNames"`
	PrimaryMetric    string                               `json:"primaryMetric"`
	MetricContexts   map[string]companyShareMetricContext `json:"metricContexts"`
	RecordIDs        []string                             `json:"recordIDs"`
	RecordContexts   map[string]companyShareRecordContext `json:"recordContexts"`
	DocumentIDs      []string                             `json:"documentIDs"`
	ContactEmail     string                               `json:"contactEmail"`
	ShowTeamActivity bool                                 `json:"showTeamActivity"`
	Narratives       map[string]companyShareNarrative     `json:"narratives"`
}

type companyShareMetricContext struct {
	Labels             map[string]string `json:"labels"`
	Descriptions       map[string]string `json:"descriptions"`
	FavorableDirection string            `json:"favorableDirection"`
	EvidenceRole       string            `json:"evidenceRole"`
	ShowSource         bool              `json:"showSource"`
}

type companyShareRecordContext struct {
	Titles        map[string]string `json:"titles"`
	Descriptions  map[string]string `json:"descriptions"`
	AttributeKeys []string          `json:"attributeKeys"`
}

type companyShareNarrative struct {
	Highlights           []string `json:"highlights"`
	BusinessModel        string   `json:"businessModel,omitempty"`
	CustomerEvidence     string   `json:"customerEvidence,omitempty"`
	MarketOpportunity    string   `json:"marketOpportunity,omitempty"`
	CompetitiveAdvantage string   `json:"competitiveAdvantage,omitempty"`
	Roadmap              string   `json:"roadmap,omitempty"`
	FundingStage         string   `json:"fundingStage,omitempty"`
	FundingTarget        string   `json:"fundingTarget,omitempty"`
	UseOfFunds           string   `json:"useOfFunds,omitempty"`
}

type companyShareProfile struct {
	Name                string `json:"name"`
	BrandName           string `json:"brandName,omitempty"`
	Slogan              string `json:"slogan,omitempty"`
	Description         string `json:"description,omitempty"`
	Website             string `json:"website,omitempty"`
	FoundedDate         string `json:"foundedDate,omitempty"`
	EmployeeCount       int    `json:"employeeCount,omitempty"`
	Jurisdiction        string `json:"jurisdiction,omitempty"`
	Representative      string `json:"representative,omitempty"`
	RepresentativeTitle string `json:"representativeTitle,omitempty"`
	Capital             string `json:"capital,omitempty"`
	FiscalYearEnd       string `json:"fiscalYearEnd,omitempty"`
	Email               string `json:"email,omitempty"`
}

type companyShareMetric struct {
	Metric   string                `json:"metric"`
	Year     int                   `json:"year"`
	Quarter  int                   `json:"quarter,omitempty"`
	Month    int                   `json:"month,omitempty"`
	Value    float64               `json:"value"`
	Currency companyMetricCurrency `json:"currency,omitempty"`
	ValueUSD *float64              `json:"valueUSD,omitempty"`
	Unit     string                `json:"unit,omitempty"`
	Source   string                `json:"source,omitempty"`
}

type companyShareRecord struct {
	Category     string            `json:"category"`
	Date         string            `json:"date,omitempty"`
	Title        string            `json:"title"`
	Titles       map[string]string `json:"titles,omitempty"`
	Descriptions map[string]string `json:"descriptions,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
}

type companyShareDocument struct {
	DocumentType string `json:"documentType"`
	Title        string `json:"title"`
	Language     string `json:"language,omitempty"`
	Summary      string `json:"summary,omitempty"`
	IssuedAt     string `json:"issuedAt,omitempty"`
}

type companyShareSnapshot struct {
	Revision       int                                  `json:"revision"`
	PublishedAt    string                               `json:"publishedAt"`
	Profiles       map[string]companyShareProfile       `json:"profiles"`
	Metrics        []companyShareMetric                 `json:"metrics"`
	PrimaryMetric  string                               `json:"primaryMetric,omitempty"`
	MetricContexts map[string]companyShareMetricContext `json:"metricContexts"`
	Records        []companyShareRecord                 `json:"records"`
	Documents      []companyShareDocument               `json:"documents"`
	ContactEmail   string                               `json:"contactEmail,omitempty"`
	TeamActivity   *companyShareTeamActivity            `json:"teamActivity,omitempty"`
	Narratives     map[string]companyShareNarrative     `json:"narratives"`
}

type companyShareSessionPayload struct {
	IssuedAt      int64 `json:"issuedAt"`
	ExpiresAt     int64 `json:"expiresAt"`
	AccessVersion int   `json:"accessVersion"`
}

type companyShareAttempt struct {
	Failures    int
	WindowStart time.Time
}

func (service *Service) writeCompanyShareSettings(responseWriter http.ResponseWriter) {
	settings, errorValue := service.readCompanyShareSettings()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, companyShareSettingsView(settings))
}

func (service *Service) updateCompanyShareSettings(responseWriter http.ResponseWriter, request *http.Request) {
	var update companyShareSettingsUpdate
	if errorValue := json.NewDecoder(request.Body).Decode(&update); errorValue != nil {
		http.Error(responseWriter, "설정 내용을 확인해 주세요.", http.StatusBadRequest)
		return
	}
	settings, errorValue := service.readCompanyShareSettings()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	updated, errorValue := applyCompanyShareSettingsUpdate(settings, update)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.writeCompanyShareSettingsFile(updated); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, companyShareSettingsView(updated))
}

func applyCompanyShareSettingsUpdate(settings companyShareSettings, update companyShareSettingsUpdate) (companyShareSettings, error) {
	password := strings.TrimSpace(update.Password)
	if password != "" && len([]rune(password)) < 8 {
		return settings, errors.New("비밀번호는 8자 이상 입력해 주세요.")
	}
	if len([]byte(password)) > 72 {
		return settings, errors.New("비밀번호는 72바이트 이하로 입력해 주세요.")
	}
	if update.SessionHours < 1 || update.SessionHours > 168 {
		return settings, errors.New("접근 유지 시간은 1시간에서 168시간 사이여야 해요.")
	}
	if password != "" {
		passwordHash, errorValue := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if errorValue != nil {
			return settings, errorValue
		}
		settings.PasswordHash = string(passwordHash)
		settings.AccessVersion++
	}
	if update.Enabled && settings.PasswordHash == "" {
		return settings, errors.New("페이지를 공개하려면 먼저 비밀번호를 설정해 주세요.")
	}
	settings.Enabled = update.Enabled
	settings.SessionHours = update.SessionHours
	settings.ProfileFields = filterCompanyShareFields(update.ProfileFields)
	settings.MetricNames = uniqueTrimmedValues(update.MetricNames)
	primaryMetric := strings.TrimSpace(update.PrimaryMetric)
	if primaryMetric != "" && !stringSet(settings.MetricNames)[primaryMetric] {
		return settings, errors.New("대표 지표는 공개할 지표 중에서 선택해 주세요.")
	}
	metricContexts := update.MetricContexts
	if metricContexts == nil {
		metricContexts = settings.MetricContexts
	}
	normalizedMetricContexts, errorValue := normalizeCompanyShareMetricContexts(metricContexts, settings.MetricNames)
	if errorValue != nil {
		return settings, errorValue
	}
	settings.PrimaryMetric = primaryMetric
	settings.MetricContexts = normalizedMetricContexts
	settings.RecordIDs = uniqueTrimmedValues(update.RecordIDs)
	recordContexts := update.RecordContexts
	if recordContexts == nil {
		recordContexts = settings.RecordContexts
	}
	normalizedRecordContexts, errorValue := normalizeCompanyShareRecordContexts(recordContexts, settings.RecordIDs)
	if errorValue != nil {
		return settings, errorValue
	}
	settings.RecordContexts = normalizedRecordContexts
	settings.DocumentIDs = uniqueTrimmedValues(update.DocumentIDs)
	contactEmail := strings.TrimSpace(update.ContactEmail)
	if contactEmail != "" && validCompanyShareEmail(contactEmail) == "" {
		return settings, errors.New("문의 이메일 주소를 확인해 주세요.")
	}
	settings.ContactEmail = contactEmail
	settings.ShowTeamActivity = update.ShowTeamActivity
	if update.Narratives != nil {
		narratives, errorValue := normalizeCompanyShareNarratives(update.Narratives)
		if errorValue != nil {
			return settings, errorValue
		}
		settings.Narratives = narratives
	}
	return settings, nil
}

func (service *Service) publishCompanyShareSnapshot(responseWriter http.ResponseWriter, request *http.Request) {
	settings, errorValue := service.readCompanyShareSettings()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !settings.Enabled || settings.PasswordHash == "" {
		http.Error(responseWriter, "페이지를 활성화하고 비밀번호를 설정한 뒤 게시해 주세요.", http.StatusBadRequest)
		return
	}
	snapshot, errorValue := service.buildCompanyShareSnapshot(request.Context(), settings, time.Now().UTC())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue := service.writeCompanyShareSnapshotFile(snapshot); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	settings.PublishedAt = snapshot.PublishedAt
	settings.PublicationRevision = snapshot.Revision
	if errorValue := service.writeCompanyShareSettingsFile(settings); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, companyShareSettingsView(settings))
}

func (service *Service) buildCompanyShareSnapshot(ctx context.Context, settings companyShareSettings, now time.Time) (companyShareSnapshot, error) {
	info, errorValue := service.readCompanyInfo()
	if errorValue != nil {
		return companyShareSnapshot{}, errorValue
	}
	metrics, errorValue := service.readCompanyShareMetrics(ctx, settings.MetricNames, settings.MetricContexts)
	if errorValue != nil {
		return companyShareSnapshot{}, errorValue
	}
	records, errorValue := service.readCompanyShareRecords(ctx, settings.RecordIDs, settings.RecordContexts)
	if errorValue != nil {
		return companyShareSnapshot{}, errorValue
	}
	documents, errorValue := service.readCompanyShareDocuments(ctx, settings.DocumentIDs)
	if errorValue != nil {
		return companyShareSnapshot{}, errorValue
	}
	var teamActivity *companyShareTeamActivity
	if settings.ShowTeamActivity {
		activity, activityError := service.buildCompanyShareTeamActivity(ctx, now)
		if activityError != nil {
			return companyShareSnapshot{}, activityError
		}
		teamActivity = &activity
	}
	return companyShareSnapshot{
		Revision:       settings.PublicationRevision + 1,
		PublishedAt:    now.Format(time.RFC3339),
		Profiles:       buildCompanyShareProfiles(info, settings.ProfileFields),
		Metrics:        metrics,
		PrimaryMetric:  settings.PrimaryMetric,
		MetricContexts: settings.MetricContexts,
		Records:        records,
		Documents:      documents,
		ContactEmail:   settings.ContactEmail,
		TeamActivity:   teamActivity,
		Narratives:     settings.Narratives,
	}, nil
}

func buildCompanyShareProfiles(info companyInfo, fields []string) map[string]companyShareProfile {
	included := stringSet(fields)
	profiles := map[string]companyShareProfile{}
	for _, language := range []string{"ko", "en"} {
		profile := companyShareProfile{Name: resolveAnyLocalized(info.Name, language)}
		if included["brandName"] {
			profile.BrandName = resolveAnyLocalized(info.BrandName, language)
		}
		if included["slogan"] {
			profile.Slogan = resolveAnyLocalized(info.Slogan, language)
		}
		if included["description"] {
			profile.Description = resolveAnyLocalized(info.Description, language)
		}
		if included["website"] {
			profile.Website = validCompanyShareWebsite(info.Website)
		}
		if included["foundedDate"] {
			profile.FoundedDate = strings.TrimSpace(info.FoundedDate)
		}
		if included["employeeCount"] {
			profile.EmployeeCount = info.EmployeeCount
		}
		if included["jurisdiction"] {
			profile.Jurisdiction = resolveAnyLocalized(info.Jurisdiction, language)
		}
		if included["representative"] {
			profile.Representative = resolveAnyLocalized(info.Representative, language)
		}
		if included["representativeTitle"] {
			profile.RepresentativeTitle = resolveAnyLocalized(info.RepresentativeTitle, language)
		}
		if included["capital"] {
			profile.Capital = strings.TrimSpace(info.Capital)
		}
		if included["fiscalYearEnd"] {
			profile.FiscalYearEnd = strings.TrimSpace(info.FiscalYearEnd)
		}
		if included["email"] {
			profile.Email = validCompanyShareEmail(info.Email)
		}
		profiles[language] = profile
	}
	return profiles
}

func (service *Service) readCompanyShareMetrics(ctx context.Context, metricNames []string, contexts map[string]companyShareMetricContext) ([]companyShareMetric, error) {
	if len(metricNames) == 0 {
		return []companyShareMetric{}, nil
	}
	database, errorValue := service.openCompanyDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `SELECT metric, year, quarter, month, value, currency, value_usd, unit, note FROM company_metrics ORDER BY metric, year, quarter, month`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	included := stringSet(metricNames)
	metrics := []companyShareMetric{}
	for rows.Next() {
		var metric companyShareMetric
		var source string
		if errorValue := rows.Scan(&metric.Metric, &metric.Year, &metric.Quarter, &metric.Month, &metric.Value, &metric.Currency, &metric.ValueUSD, &metric.Unit, &source); errorValue != nil {
			return nil, errorValue
		}
		if included[metric.Metric] {
			if contexts[metric.Metric].ShowSource {
				metric.Source = strings.TrimSpace(source)
			}
			metrics = append(metrics, metric)
		}
	}
	return metrics, rows.Err()
}

func (service *Service) readCompanyShareRecords(ctx context.Context, recordIDs []string, contexts map[string]companyShareRecordContext) ([]companyShareRecord, error) {
	if len(recordIDs) == 0 {
		return []companyShareRecord{}, nil
	}
	database, errorValue := service.openCompanyDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `SELECT id, category, record_date, title, attributes FROM company_records ORDER BY record_date DESC, updated_at DESC`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	included := stringSet(recordIDs)
	records := []companyShareRecord{}
	for rows.Next() {
		var recordID string
		var record companyShareRecord
		var attributesJSON string
		if errorValue := rows.Scan(&recordID, &record.Category, &record.Date, &record.Title, &attributesJSON); errorValue != nil {
			return nil, errorValue
		}
		if included[recordID] {
			recordContext := contexts[recordID]
			record.Titles = recordContext.Titles
			record.Descriptions = recordContext.Descriptions
			record.Attributes = publicCompanyShareAttributes(attributesJSON, recordContext.AttributeKeys)
			records = append(records, record)
		}
	}
	return records, rows.Err()
}

func (service *Service) readCompanyShareDocuments(ctx context.Context, documentIDs []string) ([]companyShareDocument, error) {
	if len(documentIDs) == 0 {
		return []companyShareDocument{}, nil
	}
	database, errorValue := service.openCompanyDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `SELECT id, document_type, title, language, summary, issued_at FROM company_documents ORDER BY issued_at DESC`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	included := stringSet(documentIDs)
	documents := []companyShareDocument{}
	for rows.Next() {
		var documentID string
		var document companyShareDocument
		if errorValue := rows.Scan(&documentID, &document.DocumentType, &document.Title, &document.Language, &document.Summary, &document.IssuedAt); errorValue != nil {
			return nil, errorValue
		}
		if included[documentID] {
			documents = append(documents, document)
		}
	}
	return documents, rows.Err()
}

func (service *Service) handleCompanyShare(responseWriter http.ResponseWriter, request *http.Request) {
	setCompanyShareResponseHeaders(responseWriter)
	path := strings.TrimPrefix(request.URL.Path, "/company/api")
	switch {
	case request.Method == http.MethodGet && path == "/session":
		service.writeCompanyShareSession(responseWriter, request)
	case request.Method == http.MethodPost && path == "/unlock":
		service.unlockCompanyShare(responseWriter, request)
	case request.Method == http.MethodGet && path == "/content":
		service.writeCompanyShareContent(responseWriter, request)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/team/") && strings.HasSuffix(path, "/image"):
		seed := strings.TrimSuffix(strings.TrimPrefix(path, "/team/"), "/image")
		service.serveCompanyShareMemberImage(responseWriter, request, seed)
	case request.Method == http.MethodPost && path == "/logout":
		http.SetCookie(responseWriter, expiredCompanyShareCookie())
		service.writeJSON(responseWriter, map[string]bool{"ok": true})
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) writeCompanyShareSession(responseWriter http.ResponseWriter, request *http.Request) {
	settings, errorValue := service.readCompanyShareSettings()
	if errorValue != nil {
		http.Error(responseWriter, "공유 페이지 상태를 불러오지 못했어요.", http.StatusInternalServerError)
		return
	}
	available := settings.Enabled && settings.PasswordHash != "" && settings.PublishedAt != ""
	service.writeJSON(responseWriter, map[string]any{
		"available":     available,
		"authenticated": available && service.hasValidCompanyShareSession(request, settings),
		"publishedAt":   settings.PublishedAt,
	})
}

func (service *Service) unlockCompanyShare(responseWriter http.ResponseWriter, request *http.Request) {
	settings, errorValue := service.readCompanyShareSettings()
	if errorValue != nil || !settings.Enabled || settings.PasswordHash == "" || settings.PublishedAt == "" {
		http.Error(responseWriter, "현재 공유 중인 회사 페이지가 없어요.", http.StatusNotFound)
		return
	}
	clientAddress := companyShareClientAddress(request)
	if service.isCompanyShareRateLimited(clientAddress, time.Now().UTC()) {
		http.Error(responseWriter, "잠시 후 다시 시도해 주세요.", http.StatusTooManyRequests)
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil || bcrypt.CompareHashAndPassword([]byte(settings.PasswordHash), []byte(body.Password)) != nil {
		service.recordCompanyShareFailure(clientAddress, time.Now().UTC())
		logAuditEvent("company share unlock denied")
		http.Error(responseWriter, "비밀번호를 확인해 주세요.", http.StatusUnauthorized)
		return
	}
	service.clearCompanyShareFailures(clientAddress)
	if errorValue := service.issueCompanyShareCookie(responseWriter, request, settings); errorValue != nil {
		http.Error(responseWriter, "접근 세션을 만들지 못했어요.", http.StatusInternalServerError)
		return
	}
	logAuditEvent("company share unlock success")
	service.writeJSON(responseWriter, map[string]bool{"authenticated": true})
}

func (service *Service) writeCompanyShareContent(responseWriter http.ResponseWriter, request *http.Request) {
	settings, errorValue := service.readCompanyShareSettings()
	if errorValue != nil || !settings.Enabled || !service.hasValidCompanyShareSession(request, settings) {
		http.Error(responseWriter, "비밀번호 인증이 필요해요.", http.StatusUnauthorized)
		return
	}
	snapshot, errorValue := service.readCompanyShareSnapshot()
	if errorValue != nil {
		http.Error(responseWriter, "공유 페이지를 불러오지 못했어요.", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, snapshot)
}

func (service *Service) issueCompanyShareCookie(responseWriter http.ResponseWriter, request *http.Request, settings companyShareSettings) error {
	now := time.Now().UTC()
	duration := time.Duration(settings.SessionHours) * time.Hour
	payload := companyShareSessionPayload{IssuedAt: now.Unix(), ExpiresAt: now.Add(duration).Unix(), AccessVersion: settings.AccessVersion}
	value, errorValue := service.signCompanySharePayload(payload)
	if errorValue != nil {
		return errorValue
	}
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     companyShareCookieName,
		Value:    value,
		Path:     "/company",
		Expires:  time.Unix(payload.ExpiresAt, 0).UTC(),
		MaxAge:   int(duration.Seconds()),
		HttpOnly: true,
		Secure:   !isLocalRequest(request),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (service *Service) hasValidCompanyShareSession(request *http.Request, settings companyShareSettings) bool {
	cookie, errorValue := request.Cookie(companyShareCookieName)
	if errorValue != nil {
		return false
	}
	payload, errorValue := service.verifyCompanySharePayload(cookie.Value, time.Now().UTC())
	return errorValue == nil && payload.AccessVersion == settings.AccessVersion
}

func expiredCompanyShareCookie() *http.Cookie {
	return &http.Cookie{
		Name:     companyShareCookieName,
		Path:     "/company",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

func (service *Service) signCompanySharePayload(payload companyShareSessionPayload) (string, error) {
	key, errorValue := service.companyShareSigningKey()
	if errorValue != nil {
		return "", errorValue
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return "", errorValue
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(document)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(encodedPayload))
	return encodedPayload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (service *Service) verifyCompanySharePayload(value string, now time.Time) (companyShareSessionPayload, error) {
	key, errorValue := service.companyShareSigningKey()
	if errorValue != nil {
		return companyShareSessionPayload{}, errorValue
	}
	encodedPayload, encodedSignature, found := strings.Cut(strings.TrimSpace(value), ".")
	if !found {
		return companyShareSessionPayload{}, errors.New("malformed")
	}
	signature, errorValue := base64.RawURLEncoding.DecodeString(encodedSignature)
	if errorValue != nil {
		return companyShareSessionPayload{}, errors.New("bad signature")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(encodedPayload))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return companyShareSessionPayload{}, errors.New("bad signature")
	}
	document, errorValue := base64.RawURLEncoding.DecodeString(encodedPayload)
	if errorValue != nil {
		return companyShareSessionPayload{}, errors.New("bad payload")
	}
	var payload companyShareSessionPayload
	if errorValue := json.Unmarshal(document, &payload); errorValue != nil || payload.ExpiresAt <= now.Unix() {
		return companyShareSessionPayload{}, errors.New("expired")
	}
	return payload, nil
}

func (service *Service) companyShareSigningKey() ([]byte, error) {
	path := filepath.Join(service.Configuration.StateDirectory, companyShareSecretName)
	document, errorValue := os.ReadFile(path)
	if errorValue == nil {
		key, decodeError := hex.DecodeString(strings.TrimSpace(string(document)))
		if decodeError != nil || len(key) != 32 {
			return nil, errors.New("company share session secret is invalid")
		}
		return key, nil
	}
	if !errors.Is(errorValue, os.ErrNotExist) {
		return nil, errorValue
	}
	key := make([]byte, 32)
	if _, errorValue := rand.Read(key); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := writeFileAtomically(path, []byte(hex.EncodeToString(key)), 0o600); errorValue != nil {
		return nil, errorValue
	}
	return key, nil
}

func (service *Service) readCompanyShareSettings() (companyShareSettings, error) {
	document, errorValue := os.ReadFile(service.companyShareSettingsPath())
	if errors.Is(errorValue, os.ErrNotExist) {
		return defaultCompanyShareSettings(), nil
	}
	if errorValue != nil {
		return companyShareSettings{}, errorValue
	}
	var settings companyShareSettings
	if errorValue := json.Unmarshal(document, &settings); errorValue != nil {
		return companyShareSettings{}, errorValue
	}
	if settings.SessionHours == 0 {
		settings.SessionHours = 24
	}
	if settings.AccessVersion == 0 {
		settings.AccessVersion = 1
	}
	if settings.Narratives == nil {
		settings.Narratives = defaultCompanyShareNarratives()
	}
	if settings.MetricContexts == nil {
		settings.MetricContexts = map[string]companyShareMetricContext{}
	}
	if settings.RecordContexts == nil {
		settings.RecordContexts = map[string]companyShareRecordContext{}
	}
	return settings, nil
}

func defaultCompanyShareSettings() companyShareSettings {
	return companyShareSettings{
		AccessVersion:  1,
		SessionHours:   24,
		ProfileFields:  append([]string{}, defaultCompanyShareFields...),
		MetricNames:    []string{},
		MetricContexts: map[string]companyShareMetricContext{},
		RecordIDs:      []string{},
		RecordContexts: map[string]companyShareRecordContext{},
		DocumentIDs:    []string{},
		Narratives:     defaultCompanyShareNarratives(),
	}
}

func defaultCompanyShareNarratives() map[string]companyShareNarrative {
	return map[string]companyShareNarrative{
		"ko": {Highlights: []string{}},
		"en": {Highlights: []string{}},
	}
}

func (service *Service) writeCompanyShareSettingsFile(settings companyShareSettings) error {
	return writeCompanyShareJSON(service.companyShareSettingsPath(), settings)
}

func (service *Service) readCompanyShareSnapshot() (companyShareSnapshot, error) {
	document, errorValue := os.ReadFile(service.companyShareSnapshotPath())
	if errorValue != nil {
		return companyShareSnapshot{}, errorValue
	}
	var snapshot companyShareSnapshot
	if errorValue := json.Unmarshal(document, &snapshot); errorValue != nil {
		return companyShareSnapshot{}, errorValue
	}
	return snapshot, nil
}

func (service *Service) writeCompanyShareSnapshotFile(snapshot companyShareSnapshot) error {
	return writeCompanyShareJSON(service.companyShareSnapshotPath(), snapshot)
}

func writeCompanyShareJSON(path string, value any) error {
	document, errorValue := json.MarshalIndent(value, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(path, append(document, '\n'), 0o600)
}

func (service *Service) companyShareSettingsPath() string {
	return filepath.Join(service.Configuration.StateDirectory, companyShareSettingsFileName)
}
func (service *Service) companyShareSnapshotPath() string {
	return filepath.Join(service.Configuration.StateDirectory, companyShareSnapshotFileName)
}

func companyShareSettingsView(settings companyShareSettings) companyShareSettingsResponse {
	return companyShareSettingsResponse{
		Enabled:             settings.Enabled,
		HasPassword:         settings.PasswordHash != "",
		SessionHours:        settings.SessionHours,
		ProfileFields:       settings.ProfileFields,
		MetricNames:         settings.MetricNames,
		PrimaryMetric:       settings.PrimaryMetric,
		MetricContexts:      settings.MetricContexts,
		RecordIDs:           settings.RecordIDs,
		RecordContexts:      settings.RecordContexts,
		DocumentIDs:         settings.DocumentIDs,
		ContactEmail:        settings.ContactEmail,
		ShowTeamActivity:    settings.ShowTeamActivity,
		Narratives:          settings.Narratives,
		PublishedAt:         settings.PublishedAt,
		PublicationRevision: settings.PublicationRevision,
	}
}

func normalizeCompanyShareMetricContexts(values map[string]companyShareMetricContext, metricNames []string) (map[string]companyShareMetricContext, error) {
	contexts := map[string]companyShareMetricContext{}
	for _, metricName := range metricNames {
		contextValue, errorValue := normalizeCompanyShareMetricContext(values[metricName])
		if errorValue != nil {
			return nil, errorValue
		}
		contexts[metricName] = contextValue
	}
	return contexts, nil
}

func normalizeCompanyShareMetricContext(value companyShareMetricContext) (companyShareMetricContext, error) {
	labels, errorValue := normalizeCompanyShareLocalizedValues(value.Labels, 120)
	if errorValue != nil {
		return companyShareMetricContext{}, errorValue
	}
	descriptions, errorValue := normalizeCompanyShareLocalizedValues(value.Descriptions, 500)
	if errorValue != nil {
		return companyShareMetricContext{}, errorValue
	}
	direction := strings.TrimSpace(value.FavorableDirection)
	if direction != "increase" && direction != "decrease" {
		direction = "neutral"
	}
	role := strings.TrimSpace(value.EvidenceRole)
	if !companyShareEvidenceRoles[role] {
		role = ""
	}
	return companyShareMetricContext{
		Labels: labels, Descriptions: descriptions, FavorableDirection: direction,
		EvidenceRole: role, ShowSource: value.ShowSource,
	}, nil
}

var companyShareEvidenceRoles = map[string]bool{
	"growth": true, "efficiency": true, "scale": true, "quality": true, "reach": true, "capital": true,
}

func normalizeCompanyShareRecordContexts(values map[string]companyShareRecordContext, recordIDs []string) (map[string]companyShareRecordContext, error) {
	contexts := map[string]companyShareRecordContext{}
	for _, recordID := range recordIDs {
		contextValue, errorValue := normalizeCompanyShareRecordContext(values[recordID])
		if errorValue != nil {
			return nil, errorValue
		}
		contexts[recordID] = contextValue
	}
	return contexts, nil
}

func normalizeCompanyShareRecordContext(value companyShareRecordContext) (companyShareRecordContext, error) {
	titles, errorValue := normalizeCompanyShareLocalizedValues(value.Titles, 160)
	if errorValue != nil {
		return companyShareRecordContext{}, errorValue
	}
	descriptions, errorValue := normalizeCompanyShareLocalizedValues(value.Descriptions, 800)
	if errorValue != nil {
		return companyShareRecordContext{}, errorValue
	}
	attributeKeys := uniqueTrimmedValues(value.AttributeKeys)
	if len(attributeKeys) > 8 {
		return companyShareRecordContext{}, errors.New("이력마다 공개 속성은 8개 이하로 선택해 주세요.")
	}
	return companyShareRecordContext{Titles: titles, Descriptions: descriptions, AttributeKeys: attributeKeys}, nil
}

func publicCompanyShareAttributes(document string, attributeKeys []string) map[string]string {
	if len(attributeKeys) == 0 {
		return nil
	}
	var values map[string]any
	if json.Unmarshal([]byte(document), &values) != nil {
		return nil
	}
	attributes := map[string]string{}
	for _, key := range attributeKeys {
		if value, isPublic := publicCompanyShareAttributeValue(values[key]); isPublic {
			attributes[key] = value
		}
	}
	return attributes
}

func publicCompanyShareAttributeValue(value any) (string, bool) {
	switch typedValue := value.(type) {
	case string:
		return strings.TrimSpace(typedValue), strings.TrimSpace(typedValue) != ""
	case float64:
		return strconv.FormatFloat(typedValue, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typedValue), true
	default:
		return "", false
	}
}

func normalizeCompanyShareLocalizedValues(values map[string]string, maximumLength int) (map[string]string, error) {
	result := map[string]string{}
	for _, language := range []string{"ko", "en"} {
		value := strings.TrimSpace(values[language])
		if len([]rune(value)) > maximumLength {
			return nil, errors.New("지표 설명 길이를 확인해 주세요.")
		}
		result[language] = value
	}
	return result, nil
}

func normalizeCompanyShareNarratives(values map[string]companyShareNarrative) (map[string]companyShareNarrative, error) {
	narratives := defaultCompanyShareNarratives()
	for _, language := range []string{"ko", "en"} {
		narrative, errorValue := normalizeCompanyShareNarrative(values[language])
		if errorValue != nil {
			return nil, errorValue
		}
		narratives[language] = narrative
	}
	return narratives, nil
}

func normalizeCompanyShareNarrative(value companyShareNarrative) (companyShareNarrative, error) {
	highlights, errorValue := normalizeCompanyShareHighlights(value.Highlights)
	if errorValue != nil {
		return companyShareNarrative{}, errorValue
	}
	fields := []*string{
		&value.BusinessModel, &value.CustomerEvidence, &value.MarketOpportunity,
		&value.CompetitiveAdvantage, &value.Roadmap, &value.FundingStage,
		&value.FundingTarget, &value.UseOfFunds,
	}
	for _, field := range fields {
		*field = strings.TrimSpace(*field)
		if len([]rune(*field)) > 2000 {
			return companyShareNarrative{}, errors.New("공개 원고는 항목마다 2,000자 이하로 입력해 주세요.")
		}
	}
	value.Highlights = highlights
	return value, nil
}

func normalizeCompanyShareHighlights(values []string) ([]string, error) {
	highlights := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len([]rune(value)) > 240 {
			return nil, errors.New("핵심 하이라이트는 항목마다 240자 이하로 입력해 주세요.")
		}
		highlights = append(highlights, value)
	}
	if len(highlights) > 3 {
		return nil, errors.New("핵심 하이라이트는 최대 3개까지 입력해 주세요.")
	}
	return highlights, nil
}

func filterCompanyShareFields(values []string) []string {
	filtered := []string{}
	for _, value := range uniqueTrimmedValues(values) {
		if allowedCompanyShareFields[value] {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func uniqueTrimmedValues(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func validCompanyShareWebsite(value string) string {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(value))
	if errorValue != nil || parsedURL.Host == "" || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") {
		return ""
	}
	return parsedURL.String()
}

func validCompanyShareEmail(value string) string {
	address, errorValue := mail.ParseAddress(strings.TrimSpace(value))
	if errorValue != nil || address.Address != strings.TrimSpace(value) {
		return ""
	}
	return address.Address
}

func companyShareClientAddress(request *http.Request) string {
	if value := strings.TrimSpace(request.Header.Get("CF-Connecting-IP")); value != "" {
		return value
	}
	host, _, errorValue := net.SplitHostPort(request.RemoteAddr)
	if errorValue == nil {
		return host
	}
	return request.RemoteAddr
}

func (service *Service) isCompanyShareRateLimited(address string, now time.Time) bool {
	service.companyShareMutex.Lock()
	defer service.companyShareMutex.Unlock()
	attempt, exists := service.companyShareAttempts[address]
	if !exists || now.Sub(attempt.WindowStart) >= companyShareAttemptWindow {
		return false
	}
	return attempt.Failures >= companyShareAttemptLimit
}

func (service *Service) recordCompanyShareFailure(address string, now time.Time) {
	service.companyShareMutex.Lock()
	defer service.companyShareMutex.Unlock()
	attempt := service.companyShareAttempts[address]
	if attempt.WindowStart.IsZero() || now.Sub(attempt.WindowStart) >= companyShareAttemptWindow {
		attempt = companyShareAttempt{WindowStart: now}
	}
	attempt.Failures++
	service.companyShareAttempts[address] = attempt
}

func (service *Service) clearCompanyShareFailures(address string) {
	service.companyShareMutex.Lock()
	defer service.companyShareMutex.Unlock()
	delete(service.companyShareAttempts, address)
}

func setCompanyShareResponseHeaders(responseWriter http.ResponseWriter) {
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	responseWriter.Header().Set("X-Robots-Tag", "noindex, nofollow")
}
