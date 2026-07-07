package admind

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (service *Service) openCompanyDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openSQLiteDatabase(ctx, service.companyDatabasePath(), ensureCompanySchema)
}

func (service *Service) companyDatabasePath() string {
	return filepath.Join(service.Configuration.StateDirectory, "company.sqlite")
}

func ensureCompanySchema(ctx context.Context, database *sql.DB) error {
	statements := []string{`
CREATE TABLE IF NOT EXISTS company_metrics (
	metric TEXT NOT NULL,
	year INTEGER NOT NULL CHECK(year >= 1900),
	quarter INTEGER NOT NULL DEFAULT 0 CHECK(quarter BETWEEN 0 AND 4),
	month INTEGER NOT NULL DEFAULT 0 CHECK(month BETWEEN 0 AND 12),
	value REAL NOT NULL,
	unit TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	PRIMARY KEY (metric, year, quarter, month),
	CHECK (quarter = 0 OR month = 0)
)`, `
CREATE TABLE IF NOT EXISTS company_records (
	id TEXT PRIMARY KEY,
	category TEXT NOT NULL,
	record_date TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL,
	detail TEXT NOT NULL DEFAULT '',
	attributes TEXT NOT NULL DEFAULT '{}',
	updated_at TEXT NOT NULL
)`, `
CREATE TABLE IF NOT EXISTS company_documents (
	id TEXT PRIMARY KEY,
	document_number TEXT NOT NULL DEFAULT '',
	kind TEXT NOT NULL DEFAULT 'issued' CHECK(kind IN ('issued', 'received', 'internal')),
	document_type TEXT NOT NULL,
	title TEXT NOT NULL,
	counterpart TEXT NOT NULL DEFAULT '',
	language TEXT NOT NULL DEFAULT '',
	file_path TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	summary_embedding TEXT NOT NULL DEFAULT '',
	requester_email TEXT NOT NULL DEFAULT '',
	issued_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`}
	for _, statement := range statements {
		if _, errorValue := database.ExecContext(ctx, statement); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

type companyMetric struct {
	Metric    string  `json:"metric"`
	Year      int     `json:"year"`
	Quarter   int     `json:"quarter,omitempty"`
	Month     int     `json:"month,omitempty"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit,omitempty"`
	Note      string  `json:"note,omitempty"`
	UpdatedAt string  `json:"updatedAt,omitempty"`
}

func (service *Service) recordCompanyMetric(responseWriter http.ResponseWriter, request *http.Request) {
	var metric companyMetric
	if errorValue := json.NewDecoder(request.Body).Decode(&metric); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	metric.Metric = strings.TrimSpace(metric.Metric)
	if metric.Metric == "" || metric.Year < 1900 {
		http.Error(responseWriter, "metric and a four-digit year are required", http.StatusBadRequest)
		return
	}
	if metric.Quarter != 0 && metric.Month != 0 {
		http.Error(responseWriter, "provide quarter or month, not both — a metric row is annual (neither), quarterly (quarter only), or monthly (month only)", http.StatusBadRequest)
		return
	}
	if metric.Quarter < 0 || metric.Quarter > 4 || metric.Month < 0 || metric.Month > 12 {
		http.Error(responseWriter, "quarter must be 1-4 and month must be 1-12", http.StatusBadRequest)
		return
	}
	database, errorValue := service.openCompanyDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	metric.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, errorValue = database.ExecContext(request.Context(), `
INSERT INTO company_metrics (metric, year, quarter, month, value, unit, note, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (metric, year, quarter, month)
DO UPDATE SET value = excluded.value, unit = excluded.unit, note = excluded.note, updated_at = excluded.updated_at`,
		metric.Metric, metric.Year, metric.Quarter, metric.Month, metric.Value, strings.TrimSpace(metric.Unit), strings.TrimSpace(metric.Note), metric.UpdatedAt)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, metric)
}

func (service *Service) listCompanyMetrics(responseWriter http.ResponseWriter, request *http.Request) {
	database, errorValue := service.openCompanyDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	query := `SELECT metric, year, quarter, month, value, unit, note, updated_at FROM company_metrics WHERE 1=1`
	arguments := []any{}
	if metricName := strings.TrimSpace(request.URL.Query().Get("metric")); metricName != "" {
		query += " AND metric = ?"
		arguments = append(arguments, metricName)
	}
	if fromYear, errorValue := strconv.Atoi(request.URL.Query().Get("fromYear")); errorValue == nil {
		query += " AND year >= ?"
		arguments = append(arguments, fromYear)
	}
	if toYear, errorValue := strconv.Atoi(request.URL.Query().Get("toYear")); errorValue == nil {
		query += " AND year <= ?"
		arguments = append(arguments, toYear)
	}
	query += " ORDER BY metric, year, quarter, month"
	rows, errorValue := database.QueryContext(request.Context(), query, arguments...)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	metrics := []companyMetric{}
	for rows.Next() {
		var metric companyMetric
		if errorValue := rows.Scan(&metric.Metric, &metric.Year, &metric.Quarter, &metric.Month, &metric.Value, &metric.Unit, &metric.Note, &metric.UpdatedAt); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
		metrics = append(metrics, metric)
	}
	service.writeJSON(responseWriter, map[string]any{"metrics": metrics})
}

type companyRecord struct {
	ID         string          `json:"id"`
	Category   string          `json:"category"`
	Date       string          `json:"date,omitempty"`
	Title      string          `json:"title"`
	Detail     string          `json:"detail,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
	UpdatedAt  string          `json:"updatedAt,omitempty"`
}

func (service *Service) addCompanyRecord(responseWriter http.ResponseWriter, request *http.Request) {
	var record companyRecord
	if errorValue := json.NewDecoder(request.Body).Decode(&record); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	record.Category = strings.TrimSpace(record.Category)
	record.Title = strings.TrimSpace(record.Title)
	if record.Category == "" || record.Title == "" {
		http.Error(responseWriter, "category and title are required", http.StatusBadRequest)
		return
	}
	record.ID = randomCompanyID()
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	database, errorValue := service.openCompanyDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	_, errorValue = database.ExecContext(request.Context(), `
INSERT INTO company_records (id, category, record_date, title, detail, attributes, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.Category, strings.TrimSpace(record.Date), record.Title, strings.TrimSpace(record.Detail), normalizeAttributesJSON(record.Attributes), record.UpdatedAt)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, record)
}

func (service *Service) listCompanyRecords(responseWriter http.ResponseWriter, request *http.Request) {
	database, errorValue := service.openCompanyDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	query := `SELECT id, category, record_date, title, detail, attributes, updated_at FROM company_records WHERE 1=1`
	arguments := []any{}
	if category := strings.TrimSpace(request.URL.Query().Get("category")); category != "" {
		query += " AND category = ?"
		arguments = append(arguments, category)
	}
	if keyword := strings.TrimSpace(request.URL.Query().Get("query")); keyword != "" {
		query += " AND (title LIKE ? OR detail LIKE ? OR attributes LIKE ?)"
		pattern := "%" + keyword + "%"
		arguments = append(arguments, pattern, pattern, pattern)
	}
	query += " ORDER BY record_date DESC, updated_at DESC"
	rows, errorValue := database.QueryContext(request.Context(), query, arguments...)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	records := []companyRecord{}
	for rows.Next() {
		var record companyRecord
		var attributes string
		if errorValue := rows.Scan(&record.ID, &record.Category, &record.Date, &record.Title, &record.Detail, &attributes, &record.UpdatedAt); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
		record.Attributes = json.RawMessage(attributes)
		records = append(records, record)
	}
	service.writeJSON(responseWriter, map[string]any{"records": records})
}

func (service *Service) updateCompanyRecord(responseWriter http.ResponseWriter, request *http.Request) {
	var record companyRecord
	if errorValue := json.NewDecoder(request.Body).Decode(&record); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(record.ID) == "" {
		http.Error(responseWriter, "id is required", http.StatusBadRequest)
		return
	}
	database, errorValue := service.openCompanyDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	assignments := []string{"updated_at = ?"}
	arguments := []any{time.Now().UTC().Format(time.RFC3339)}
	appendAssignment := func(column string, value string) {
		if strings.TrimSpace(value) != "" {
			assignments = append(assignments, column+" = ?")
			arguments = append(arguments, strings.TrimSpace(value))
		}
	}
	appendAssignment("category", record.Category)
	appendAssignment("record_date", record.Date)
	appendAssignment("title", record.Title)
	appendAssignment("detail", record.Detail)
	if len(record.Attributes) > 0 {
		assignments = append(assignments, "attributes = ?")
		arguments = append(arguments, normalizeAttributesJSON(record.Attributes))
	}
	arguments = append(arguments, strings.TrimSpace(record.ID))
	result, errorValue := database.ExecContext(request.Context(), "UPDATE company_records SET "+strings.Join(assignments, ", ")+" WHERE id = ?", arguments...)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		http.Error(responseWriter, "record not found", http.StatusNotFound)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"id": record.ID, "updated": true})
}

func (service *Service) deleteCompanyRecord(responseWriter http.ResponseWriter, request *http.Request) {
	recordID := strings.TrimSpace(request.URL.Query().Get("id"))
	if recordID == "" {
		http.Error(responseWriter, "id is required", http.StatusBadRequest)
		return
	}
	database, errorValue := service.openCompanyDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	result, errorValue := database.ExecContext(request.Context(), "DELETE FROM company_records WHERE id = ?", recordID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		http.Error(responseWriter, "record not found", http.StatusNotFound)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"id": recordID, "deleted": true})
}

type companyDocument struct {
	ID               string    `json:"id"`
	DocumentNumber   string    `json:"documentNumber,omitempty"`
	Kind             string    `json:"kind"`
	DocumentType     string    `json:"documentType"`
	Title            string    `json:"title"`
	Counterpart      string    `json:"counterpart,omitempty"`
	Language         string    `json:"language,omitempty"`
	FilePath         string    `json:"filePath,omitempty"`
	Summary          string    `json:"summary,omitempty"`
	SummaryEmbedding []float64 `json:"summaryEmbedding,omitempty"`
	RequesterEmail   string    `json:"requesterEmail,omitempty"`
	IssuedAt         string    `json:"issuedAt,omitempty"`
	UpdatedAt        string    `json:"updatedAt,omitempty"`
	StorageDirectory string    `json:"storageDirectory,omitempty"`
	Score            float64   `json:"score,omitempty"`
}

var companyDocumentNumberPrefixes = map[string]string{
	"quote":                  "Q",
	"invoice":                "INV",
	"purchase-order":         "PO",
	"transaction-statement":  "TS",
	"approval-request":       "APR",
	"expense-approval":       "EXP",
	"meeting-minutes":        "MIN",
	"weekly-report":          "WKR",
	"business-trip-report":   "BTR",
	"employment-certificate": "CERT",
	"career-certificate":     "CRT",
	"leave-request":          "LV",
	"power-of-attorney":      "POA",
	"offer-letter":           "OFR",
	"employment-contract":    "EMP",
	"nda":                    "NDA",
	"mou":                    "MOU",
	"service-agreement":      "SVC",
}

func companyDocumentNumberPrefix(documentType string) string {
	if prefix, isKnown := companyDocumentNumberPrefixes[documentType]; isKnown {
		return prefix
	}
	initials := ""
	for _, segment := range strings.Split(documentType, "-") {
		if segment != "" {
			initials += strings.ToUpper(segment[:1])
		}
	}
	if initials == "" {
		return "DOC"
	}
	return initials
}

func (service *Service) registerCompanyDocument(responseWriter http.ResponseWriter, request *http.Request) {
	var document companyDocument
	if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	document.DocumentType = strings.TrimSpace(document.DocumentType)
	document.Title = strings.TrimSpace(document.Title)
	if document.DocumentType == "" || document.Title == "" {
		http.Error(responseWriter, "documentType and title are required", http.StatusBadRequest)
		return
	}
	document.Kind = normalizeDocumentKind(document.Kind)
	document.ID = randomCompanyID()
	now := time.Now().UTC()
	document.IssuedAt = now.Format(time.RFC3339)
	document.UpdatedAt = document.IssuedAt
	document.RequesterEmail = strings.ToLower(strings.TrimSpace(request.Header.Get("CF-Access-Authenticated-User-Email")))
	database, errorValue := service.openCompanyDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(request.Context(), nil)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if document.Kind == "issued" {
		number, errorValue := nextCompanyDocumentNumber(request.Context(), transaction, document.DocumentType, now.Year())
		if errorValue != nil {
			_ = transaction.Rollback()
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
		document.DocumentNumber = number
	}
	_, errorValue = transaction.ExecContext(request.Context(), `
INSERT INTO company_documents (id, document_number, kind, document_type, title, counterpart, language, file_path, summary, summary_embedding, requester_email, issued_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		document.ID, document.DocumentNumber, document.Kind, document.DocumentType, document.Title,
		strings.TrimSpace(document.Counterpart), strings.TrimSpace(document.Language), strings.TrimSpace(document.FilePath),
		strings.TrimSpace(document.Summary), encodeEmbedding(document.SummaryEmbedding), document.RequesterEmail,
		document.IssuedAt, document.UpdatedAt)
	if errorValue != nil {
		_ = transaction.Rollback()
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	document.SummaryEmbedding = nil
	document.StorageDirectory = "/workspace/circles/staff/documents/" + document.DocumentType
	service.writeJSON(responseWriter, document)
}

func nextCompanyDocumentNumber(ctx context.Context, transaction *sql.Tx, documentType string, year int) (string, error) {
	prefix := fmt.Sprintf("%s-%d-", companyDocumentNumberPrefix(documentType), year)
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT document_number FROM company_documents WHERE document_number LIKE ?`, prefix+"%")
	if errorValue != nil {
		return "", errorValue
	}
	defer rows.Close()
	highest := 0
	for rows.Next() {
		var number string
		if errorValue := rows.Scan(&number); errorValue != nil {
			return "", errorValue
		}
		sequence, errorValue := strconv.Atoi(strings.TrimPrefix(number, prefix))
		if errorValue == nil && sequence > highest {
			highest = sequence
		}
	}
	return fmt.Sprintf("%s%03d", prefix, highest+1), nil
}

func normalizeDocumentKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "received" || kind == "internal" {
		return kind
	}
	return "issued"
}

func (service *Service) listCompanyDocuments(responseWriter http.ResponseWriter, request *http.Request) {
	database, errorValue := service.openCompanyDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	query := `SELECT id, document_number, kind, document_type, title, counterpart, language, file_path, summary, requester_email, issued_at, updated_at
FROM company_documents WHERE 1=1`
	arguments := []any{}
	if documentType := strings.TrimSpace(request.URL.Query().Get("type")); documentType != "" {
		query += " AND document_type = ?"
		arguments = append(arguments, documentType)
	}
	if counterpart := strings.TrimSpace(request.URL.Query().Get("counterpart")); counterpart != "" {
		query += " AND counterpart LIKE ?"
		arguments = append(arguments, "%"+counterpart+"%")
	}
	if keyword := strings.TrimSpace(request.URL.Query().Get("query")); keyword != "" {
		query += " AND (title LIKE ? OR summary LIKE ? OR counterpart LIKE ?)"
		pattern := "%" + keyword + "%"
		arguments = append(arguments, pattern, pattern, pattern)
	}
	query += " ORDER BY issued_at DESC"
	documents, errorValue := scanCompanyDocuments(request.Context(), database, query, arguments)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"documents": documents})
}

type companyDocumentSearchRequest struct {
	Query          string    `json:"query"`
	QueryEmbedding []float64 `json:"queryEmbedding"`
	Limit          int       `json:"limit"`
}

func (service *Service) searchCompanyDocuments(responseWriter http.ResponseWriter, request *http.Request) {
	var searchRequest companyDocumentSearchRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&searchRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	limit := searchRequest.Limit
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	database, errorValue := service.openCompanyDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	if len(searchRequest.QueryEmbedding) > 0 {
		documents, errorValue := service.searchCompanyDocumentsByEmbedding(request.Context(), database, searchRequest.QueryEmbedding, limit)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
		if len(documents) > 0 {
			service.writeJSON(responseWriter, map[string]any{"documents": documents, "matchKind": "semantic"})
			return
		}
	}
	query := `SELECT id, document_number, kind, document_type, title, counterpart, language, file_path, summary, requester_email, issued_at, updated_at
FROM company_documents WHERE (title LIKE ? OR summary LIKE ? OR counterpart LIKE ?) ORDER BY issued_at DESC LIMIT ?`
	pattern := "%" + strings.TrimSpace(searchRequest.Query) + "%"
	documents, errorValue := scanCompanyDocuments(request.Context(), database, query, []any{pattern, pattern, pattern, limit})
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"documents": documents, "matchKind": "text"})
}

func (service *Service) searchCompanyDocumentsByEmbedding(ctx context.Context, database *sql.DB, queryEmbedding []float64, limit int) ([]companyDocument, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, document_number, kind, document_type, title, counterpart, language, file_path, summary, summary_embedding, requester_email, issued_at, updated_at
FROM company_documents WHERE summary_embedding != ''`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	scored := []companyDocument{}
	for rows.Next() {
		var document companyDocument
		var embeddingText string
		if errorValue := rows.Scan(&document.ID, &document.DocumentNumber, &document.Kind, &document.DocumentType, &document.Title,
			&document.Counterpart, &document.Language, &document.FilePath, &document.Summary, &embeddingText,
			&document.RequesterEmail, &document.IssuedAt, &document.UpdatedAt); errorValue != nil {
			return nil, errorValue
		}
		embedding := decodeEmbedding(embeddingText)
		similarity, isComparable := cosineSimilarity(queryEmbedding, embedding)
		if !isComparable {
			continue
		}
		document.Score = similarity
		scored = append(scored, document)
	}
	sort.Slice(scored, func(left, right int) bool { return scored[left].Score > scored[right].Score })
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

func (service *Service) updateCompanyDocument(responseWriter http.ResponseWriter, request *http.Request) {
	var document companyDocument
	if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(document.ID) == "" {
		http.Error(responseWriter, "id is required", http.StatusBadRequest)
		return
	}
	database, errorValue := service.openCompanyDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	assignments := []string{"updated_at = ?"}
	arguments := []any{time.Now().UTC().Format(time.RFC3339)}
	appendAssignment := func(column string, value string) {
		if strings.TrimSpace(value) != "" {
			assignments = append(assignments, column+" = ?")
			arguments = append(arguments, strings.TrimSpace(value))
		}
	}
	appendAssignment("title", document.Title)
	appendAssignment("counterpart", document.Counterpart)
	appendAssignment("file_path", document.FilePath)
	appendAssignment("summary", document.Summary)
	if len(document.SummaryEmbedding) > 0 {
		assignments = append(assignments, "summary_embedding = ?")
		arguments = append(arguments, encodeEmbedding(document.SummaryEmbedding))
	}
	arguments = append(arguments, strings.TrimSpace(document.ID))
	result, errorValue := database.ExecContext(request.Context(), "UPDATE company_documents SET "+strings.Join(assignments, ", ")+" WHERE id = ?", arguments...)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		http.Error(responseWriter, "document not found", http.StatusNotFound)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"id": document.ID, "updated": true})
}

func scanCompanyDocuments(ctx context.Context, database *sql.DB, query string, arguments []any) ([]companyDocument, error) {
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	documents := []companyDocument{}
	for rows.Next() {
		var document companyDocument
		if errorValue := rows.Scan(&document.ID, &document.DocumentNumber, &document.Kind, &document.DocumentType, &document.Title,
			&document.Counterpart, &document.Language, &document.FilePath, &document.Summary,
			&document.RequesterEmail, &document.IssuedAt, &document.UpdatedAt); errorValue != nil {
			return nil, errorValue
		}
		documents = append(documents, document)
	}
	return documents, nil
}

func normalizeAttributesJSON(attributes json.RawMessage) string {
	if len(attributes) == 0 {
		return "{}"
	}
	var values map[string]any
	if errorValue := json.Unmarshal(attributes, &values); errorValue != nil {
		return "{}"
	}
	document, errorValue := json.Marshal(values)
	if errorValue != nil {
		return "{}"
	}
	return string(document)
}

func encodeEmbedding(embedding []float64) string {
	if len(embedding) == 0 {
		return ""
	}
	document, errorValue := json.Marshal(embedding)
	if errorValue != nil {
		return ""
	}
	return string(document)
}

func decodeEmbedding(text string) []float64 {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var embedding []float64
	if errorValue := json.Unmarshal([]byte(text), &embedding); errorValue != nil {
		return nil
	}
	return embedding
}

func cosineSimilarity(left []float64, right []float64) (float64, bool) {
	if len(left) == 0 || len(left) != len(right) {
		return 0, false
	}
	dotProduct, leftNorm, rightNorm := 0.0, 0.0, 0.0
	for index := range left {
		dotProduct += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0, false
	}
	return dotProduct / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm)), true
}

func randomCompanyID() string {
	buffer := make([]byte, 12)
	if _, errorValue := rand.Read(buffer); errorValue != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))[:24]
	}
	return hex.EncodeToString(buffer)
}
