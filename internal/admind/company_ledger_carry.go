package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

const companyLedgerCarryRecoveryAction = "company-ledger-carry-into-the-record"
const companyLedgerCarryTimeout = 30 * time.Second

// What a device recorded about the company before the company did. These drop
// only once the record holds all of them.
var carriedCompanyLedgerTables = []string{
	"company_metrics",
	"company_records",
	"company_documents",
	"company_ledger_carried_rows",
}

// The store is not created any more, only opened to be let go of. A device that
// never used the company tools has nothing here and nothing is made for it.
func (service *Service) openCompanyLedgerDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openStateDatabase(ctx, "company", ensureTheCompanyLedgerIsReadable, sqliteDatabaseOptions{})
}

// The oldest stores have neither currency column, and a row cannot be carried
// out of a column that is not there.
func ensureTheCompanyLedgerIsReadable(ctx context.Context, database *sql.DB) error {
	if _, held := countRowsInCompanyLedgerTable(ctx, database, "company_metrics"); !held {
		return nil
	}
	if errorValue := ensureCompanyMetricColumn(ctx, database, "currency", companyMetricCurrencyColumn); errorValue != nil {
		return errorValue
	}
	return ensureCompanyMetricColumn(ctx, database, "value_usd", "REAL")
}

const companyMetricCurrencyColumn = "TEXT NOT NULL DEFAULT '' CHECK(currency IN ('', 'USD', 'KRW', 'EUR', 'JPY', 'GBP', 'CNY', 'HKD', 'SGD', 'AUD', 'CAD', 'CHF', 'INR'))"

func ensureCompanyMetricColumn(ctx context.Context, database *sql.DB, columnName string, definition string) error {
	rows, errorValue := database.QueryContext(ctx, "PRAGMA table_info(company_metrics)")
	if errorValue != nil {
		return errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var columnIndex int
		var name, columnType string
		var isNotNull, primaryKey int
		var defaultValue any
		if errorValue := rows.Scan(&columnIndex, &name, &columnType, &isNotNull, &defaultValue, &primaryKey); errorValue != nil {
			return errorValue
		}
		if name == columnName {
			return rows.Err()
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, "ALTER TABLE company_metrics ADD COLUMN "+columnName+" "+definition)
	return errorValue
}

type heldCompanyMetric struct {
	Key          string
	Metric       string
	Year         int
	Quarter      int
	Month        int
	Value        float64
	CurrencyCode string
	ValueUSD     *float64
	Unit         string
	Note         string
	UpdatedAt    string
}

type heldCompanyRecord struct {
	ID         string
	Category   string
	RecordDate string
	Title      string
	Detail     string
	Attributes string
	UpdatedAt  string
}

type heldCompanyDocument struct {
	ID             string
	DocumentNumber string
	Kind           string
	DocumentType   string
	Title          string
	Counterpart    string
	Language       string
	FilePath       string
	Summary        string
	RequesterEmail string
	IssuedAt       string
}

func (metric heldCompanyMetric) periodKey() string {
	return strings.Join([]string{
		metric.Metric,
		strconv.Itoa(metric.Year),
		strconv.Itoa(metric.Quarter),
		strconv.Itoa(metric.Month),
	}, "|")
}

func uncarriedCompanyMetrics(ctx context.Context, database *sql.DB) ([]heldCompanyMetric, error) {
	carried, errorValue := carriedCompanyLedgerRowIDs(ctx, database)
	if errorValue != nil {
		return nil, errorValue
	}
	if _, held := countRowsInCompanyLedgerTable(ctx, database, "company_metrics"); !held {
		return nil, nil
	}
	rows, errorValue := database.QueryContext(ctx, `
SELECT metric, year, quarter, month, value, currency, value_usd, unit, note, updated_at
FROM company_metrics ORDER BY metric, year, quarter, month`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	metrics := []heldCompanyMetric{}
	for rows.Next() {
		var metric heldCompanyMetric
		if errorValue := rows.Scan(&metric.Metric, &metric.Year, &metric.Quarter, &metric.Month,
			&metric.Value, &metric.CurrencyCode, &metric.ValueUSD, &metric.Unit, &metric.Note,
			&metric.UpdatedAt); errorValue != nil {
			return nil, errorValue
		}
		metric.Key = metric.periodKey()
		if _, taken := carried[metric.Key]; taken {
			continue
		}
		metrics = append(metrics, metric)
	}
	return metrics, rows.Err()
}

func uncarriedCompanyRecords(ctx context.Context, database *sql.DB) ([]heldCompanyRecord, error) {
	carried, errorValue := carriedCompanyLedgerRowIDs(ctx, database)
	if errorValue != nil {
		return nil, errorValue
	}
	if _, held := countRowsInCompanyLedgerTable(ctx, database, "company_records"); !held {
		return nil, nil
	}
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, category, record_date, title, detail, attributes, updated_at
FROM company_records ORDER BY record_date, id`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	records := []heldCompanyRecord{}
	for rows.Next() {
		var record heldCompanyRecord
		if errorValue := rows.Scan(&record.ID, &record.Category, &record.RecordDate, &record.Title,
			&record.Detail, &record.Attributes, &record.UpdatedAt); errorValue != nil {
			return nil, errorValue
		}
		if _, taken := carried[record.ID]; taken {
			continue
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func uncarriedCompanyDocuments(ctx context.Context, database *sql.DB) ([]heldCompanyDocument, error) {
	carried, errorValue := carriedCompanyLedgerRowIDs(ctx, database)
	if errorValue != nil {
		return nil, errorValue
	}
	if _, held := countRowsInCompanyLedgerTable(ctx, database, "company_documents"); !held {
		return nil, nil
	}
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, document_number, kind, document_type, title, counterpart, language, file_path,
	summary, requester_email, issued_at
FROM company_documents ORDER BY issued_at, id`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	documents := []heldCompanyDocument{}
	for rows.Next() {
		var document heldCompanyDocument
		if errorValue := rows.Scan(&document.ID, &document.DocumentNumber, &document.Kind,
			&document.DocumentType, &document.Title, &document.Counterpart, &document.Language,
			&document.FilePath, &document.Summary, &document.RequesterEmail,
			&document.IssuedAt); errorValue != nil {
			return nil, errorValue
		}
		if _, taken := carried[document.ID]; taken {
			continue
		}
		documents = append(documents, document)
	}
	return documents, rows.Err()
}

func carriedCompanyMetricOf(metric heldCompanyMetric) (centralplane.CarriedCompanyMetric, error) {
	writtenAt, errorValue := time.Parse(time.RFC3339, metric.UpdatedAt)
	if errorValue != nil {
		return centralplane.CarriedCompanyMetric{}, fmt.Errorf("updated at %q is not a moment", metric.UpdatedAt)
	}
	currencyCode := strings.TrimSpace(metric.CurrencyCode)
	if currencyCode != "" && metric.ValueUSD == nil {
		return centralplane.CarriedCompanyMetric{}, fmt.Errorf("%s is in %s and names no USD equivalent, which the record keeps beside every amount", metric.Metric, currencyCode)
	}
	return centralplane.CarriedCompanyMetric{
		Metric:       metric.Metric,
		Year:         metric.Year,
		Quarter:      metric.Quarter,
		Month:        metric.Month,
		Value:        metric.Value,
		CurrencyCode: currencyCode,
		ValueUSD:     metric.ValueUSD,
		Unit:         metric.Unit,
		Note:         metric.Note,
		UpdatedAt:    writtenAt,
	}, nil
}

// The device let a record be dated by year and month alone, and the record keeps
// a date, so a half date is refused rather than guessed into a day.
func carriedCompanyRecordOf(record heldCompanyRecord) (centralplane.CarriedCompanyRecord, error) {
	writtenAt, errorValue := time.Parse(time.RFC3339, record.UpdatedAt)
	if errorValue != nil {
		return centralplane.CarriedCompanyRecord{}, fmt.Errorf("updated at %q is not a moment", record.UpdatedAt)
	}
	recordDate := strings.TrimSpace(record.RecordDate)
	if recordDate != "" {
		if _, errorValue := time.Parse(time.DateOnly, recordDate); errorValue != nil {
			return centralplane.CarriedCompanyRecord{}, fmt.Errorf("date %q is not a day", recordDate)
		}
	}
	attributes, errorValue := carriedAttributesOf(record.Attributes)
	if errorValue != nil {
		return centralplane.CarriedCompanyRecord{}, errorValue
	}
	return centralplane.CarriedCompanyRecord{
		Category:   record.Category,
		RecordDate: recordDate,
		Title:      record.Title,
		Detail:     record.Detail,
		Attributes: attributes,
		UpdatedAt:  writtenAt,
	}, nil
}

func carriedAttributesOf(written string) (map[string]string, error) {
	document := strings.TrimSpace(written)
	if document == "" {
		return map[string]string{}, nil
	}
	var values map[string]any
	if json.Unmarshal([]byte(document), &values) != nil {
		return nil, fmt.Errorf("attributes %q are not a JSON object", document)
	}
	attributes := map[string]string{}
	for label, value := range values {
		switch typed := value.(type) {
		case string:
			attributes[label] = typed
		case float64:
			attributes[label] = strconv.FormatFloat(typed, 'f', -1, 64)
		case bool:
			attributes[label] = strconv.FormatBool(typed)
		default:
			return nil, fmt.Errorf("attribute %q is neither text nor a number", label)
		}
	}
	return attributes, nil
}

func carriedCompanyDocumentOf(document heldCompanyDocument) (centralplane.CarriedCompanyDocument, error) {
	issuedAt, errorValue := time.Parse(time.RFC3339, document.IssuedAt)
	if errorValue != nil {
		return centralplane.CarriedCompanyDocument{}, fmt.Errorf("issued at %q is not a moment", document.IssuedAt)
	}
	requesterEmail := strings.ToLower(strings.TrimSpace(document.RequesterEmail))
	if requesterEmail == "" {
		return centralplane.CarriedCompanyDocument{}, fmt.Errorf("%q names nobody who asked for it, and a document is written as the person who did", document.Title)
	}
	return centralplane.CarriedCompanyDocument{
		RequesterEmail: requesterEmail,
		DocumentNumber: document.DocumentNumber,
		Kind:           document.Kind,
		DocumentType:   document.DocumentType,
		Title:          document.Title,
		Counterpart:    document.Counterpart,
		Language:       document.Language,
		FilePath:       document.FilePath,
		Summary:        document.Summary,
		IssuedAt:       issuedAt,
	}, nil
}

type companyLedgerCarryReport struct {
	Metrics   int      `json:"metrics"`
	Records   int      `json:"records"`
	Documents int      `json:"documents"`
	Refused   []string `json:"refused"`
}

// A row the record refuses is reported, not reshaped: it stays here, it is
// named, and the tables stay with it.
func (service *Service) carryTheCompanyLedgerIntoTheRecord(ctx context.Context) (companyLedgerCarryReport, error) {
	report := companyLedgerCarryReport{Refused: []string{}}
	client := service.centralPlane()
	if client == nil {
		return report, fmt.Errorf("this device names no company to carry its ledger into")
	}
	administratorEmail := service.claimedAdminEmail()
	if administratorEmail == "" {
		return report, fmt.Errorf("no administrator is claimed here, and what a company states about itself is an administrator's to record")
	}
	database, errorValue := service.openCompanyLedgerDatabase(ctx)
	if errorValue != nil {
		return report, errorValue
	}
	defer database.Close()

	companyID, errorValue := client.CompanyRowID(ctx, administratorEmail)
	if errorValue != nil {
		return report, errorValue
	}
	if errorValue := carryTheMetrics(ctx, client, database, administratorEmail, companyID, &report); errorValue != nil {
		return report, errorValue
	}
	if errorValue := carryTheRecords(ctx, client, database, administratorEmail, companyID, &report); errorValue != nil {
		return report, errorValue
	}
	return report, carryTheDocuments(ctx, client, database, companyID, &report)
}

func carryTheMetrics(
	ctx context.Context,
	client *centralplane.Client,
	database *sql.DB,
	administratorEmail string,
	companyID string,
	report *companyLedgerCarryReport,
) error {
	metrics, errorValue := uncarriedCompanyMetrics(ctx, database)
	if errorValue != nil {
		return errorValue
	}
	for _, metric := range metrics {
		carried, errorValue := carriedCompanyMetricOf(metric)
		if errorValue == nil {
			errorValue = withinTheCarryBudget(ctx, func(carryContext context.Context) error {
				return client.CarryCompanyMetric(carryContext, administratorEmail, companyID, carried)
			})
		}
		if errorValue != nil {
			report.Refused = append(report.Refused, metric.Key+": "+errorValue.Error())
			continue
		}
		rememberCarriedCompanyLedgerRow(ctx, database, metric.Key)
		report.Metrics++
	}
	return nil
}

func carryTheRecords(
	ctx context.Context,
	client *centralplane.Client,
	database *sql.DB,
	administratorEmail string,
	companyID string,
	report *companyLedgerCarryReport,
) error {
	records, errorValue := uncarriedCompanyRecords(ctx, database)
	if errorValue != nil {
		return errorValue
	}
	for _, record := range records {
		carried, errorValue := carriedCompanyRecordOf(record)
		if errorValue == nil {
			errorValue = withinTheCarryBudget(ctx, func(carryContext context.Context) error {
				return client.CarryCompanyRecord(carryContext, administratorEmail, companyID, carried)
			})
		}
		if errorValue != nil {
			report.Refused = append(report.Refused, record.ID+": "+errorValue.Error())
			continue
		}
		rememberCarriedCompanyLedgerRow(ctx, database, record.ID)
		report.Records++
	}
	return nil
}

func carryTheDocuments(
	ctx context.Context,
	client *centralplane.Client,
	database *sql.DB,
	companyID string,
	report *companyLedgerCarryReport,
) error {
	documents, errorValue := uncarriedCompanyDocuments(ctx, database)
	if errorValue != nil {
		return errorValue
	}
	for _, document := range documents {
		carried, errorValue := carriedCompanyDocumentOf(document)
		if errorValue == nil {
			errorValue = withinTheCarryBudget(ctx, func(carryContext context.Context) error {
				return client.CarryCompanyDocument(carryContext, companyID, carried)
			})
		}
		if errorValue != nil {
			report.Refused = append(report.Refused, document.ID+": "+document.RequesterEmail+": "+errorValue.Error())
			continue
		}
		rememberCarriedCompanyLedgerRow(ctx, database, document.ID)
		report.Documents++
	}
	return nil
}

func withinTheCarryBudget(ctx context.Context, carry func(context.Context) error) error {
	carryContext, cancel := context.WithTimeout(ctx, companyLedgerCarryTimeout)
	defer cancel()
	return carry(carryContext)
}

// What the record took, kept against the local key so a second sweep does not
// count a carried row as still missing. The table is retired with the store it
// describes, so it never outlives what it is about.
func carriedCompanyLedgerRowIDs(ctx context.Context, database *sql.DB) (map[string]struct{}, error) {
	carried := map[string]struct{}{}
	if _, held := countRowsInCompanyLedgerTable(ctx, database, "company_ledger_carried_rows"); !held {
		return carried, nil
	}
	rows, errorValue := database.QueryContext(ctx, "SELECT id FROM company_ledger_carried_rows")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if errorValue := rows.Scan(&id); errorValue != nil {
			return nil, errorValue
		}
		carried[id] = struct{}{}
	}
	return carried, rows.Err()
}

func rememberCarriedCompanyLedgerRow(ctx context.Context, database *sql.DB, id string) {
	if _, errorValue := database.ExecContext(ctx,
		"CREATE TABLE IF NOT EXISTS company_ledger_carried_rows (id TEXT PRIMARY KEY)"); errorValue != nil {
		return
	}
	database.ExecContext(ctx, "INSERT OR IGNORE INTO company_ledger_carried_rows (id) VALUES (?)", id)
}

func countRowsInCompanyLedgerTable(ctx context.Context, database *sql.DB, tableName string) (int, bool) {
	var rowCount int
	if database.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tableName).Scan(&rowCount) != nil {
		return 0, false
	}
	return rowCount, true
}

func companyLedgerTheRecordDoesNotHold(ctx context.Context, database *sql.DB) (int, error) {
	metrics, errorValue := uncarriedCompanyMetrics(ctx, database)
	if errorValue != nil {
		return 0, errorValue
	}
	records, errorValue := uncarriedCompanyRecords(ctx, database)
	if errorValue != nil {
		return 0, errorValue
	}
	documents, errorValue := uncarriedCompanyDocuments(ctx, database)
	if errorValue != nil {
		return 0, errorValue
	}
	return len(metrics) + len(records) + len(documents), nil
}

// A device that names no company covers nothing, so it keeps everything it
// holds. Reading this the other way round is how a company's own history gets
// dropped by a device the record has never heard of.
func (service *Service) sweepTheCompanyLedgerTheRecordNowHolds(ctx context.Context) {
	database, errorValue := service.openCompanyLedgerDatabase(ctx)
	if errorValue != nil {
		return
	}
	defer database.Close()

	uncovered, errorValue := companyLedgerTheRecordDoesNotHold(ctx, database)
	if errorValue != nil {
		slog.WarnContext(ctx, "the company ledger this device still holds could not be counted, so none of it was let go",
			"error", errorValue)
		return
	}
	if uncovered > 0 && service.centralPlane() == nil {
		slog.WarnContext(ctx, "this device names no company, so it keeps the ledger nobody else holds",
			"uncovered", uncovered, "recovery_action", companyLedgerCarryRecoveryAction)
		return
	}
	if uncovered > 0 {
		slog.WarnContext(ctx, "this device holds a company ledger the record does not, so its store stays",
			"uncovered", uncovered, "recovery_action", companyLedgerCarryRecoveryAction)
		return
	}
	if errorValue := dropCompanyLedgerTables(ctx, database); errorValue != nil {
		slog.WarnContext(ctx, "a company ledger table the record now holds could not be dropped", "error", errorValue)
	}
}

func dropCompanyLedgerTables(ctx context.Context, database *sql.DB) error {
	for _, tableName := range carriedCompanyLedgerTables {
		rowCount, held := countRowsInCompanyLedgerTable(ctx, database, tableName)
		if !held {
			continue
		}
		if _, errorValue := database.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName); errorValue != nil {
			return errorValue
		}
		slog.InfoContext(ctx, "dropped a company ledger table the record now holds",
			"table", tableName, "rows", rowCount)
	}
	return nil
}

func (service *Service) startCompanyLedgerSweep(ctx context.Context) {
	go service.sweepTheCompanyLedgerTheRecordNowHolds(ctx)
}

type companyLedgerCoverage struct {
	Metrics   int      `json:"metrics"`
	Records   int      `json:"records"`
	Documents int      `json:"documents"`
	Carried   int      `json:"carried"`
	Refused   []string `json:"refused"`
}

func (service *Service) handleCompanyLedgerCoverage(responseWriter http.ResponseWriter, request *http.Request) {
	coverage, errorValue := service.companyLedgerCoverageOfTheRecord(request)
	if errorValue != nil {
		http.Error(responseWriter, "company_ledger_coverage_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, coverage)
}

func (service *Service) companyLedgerCoverageOfTheRecord(request *http.Request) (companyLedgerCoverage, error) {
	coverage, errorValue := service.companyLedgerStillHeldHere(request.Context())
	if errorValue != nil || request.URL.Query().Get("carry") != "true" {
		return coverage, errorValue
	}
	report, errorValue := service.carryTheCompanyLedgerIntoTheRecord(request.Context())
	if errorValue != nil {
		return coverage, errorValue
	}
	coverage.Carried = report.Metrics + report.Records + report.Documents
	coverage.Refused = report.Refused
	service.sweepTheCompanyLedgerTheRecordNowHolds(request.Context())
	return coverage, nil
}

func (service *Service) companyLedgerStillHeldHere(ctx context.Context) (companyLedgerCoverage, error) {
	coverage := companyLedgerCoverage{Refused: []string{}}
	database, errorValue := service.openCompanyLedgerDatabase(ctx)
	if errorValue != nil {
		return coverage, errorValue
	}
	defer database.Close()
	metrics, errorValue := uncarriedCompanyMetrics(ctx, database)
	if errorValue != nil {
		return coverage, errorValue
	}
	records, errorValue := uncarriedCompanyRecords(ctx, database)
	if errorValue != nil {
		return coverage, errorValue
	}
	documents, errorValue := uncarriedCompanyDocuments(ctx, database)
	if errorValue != nil {
		return coverage, errorValue
	}
	coverage.Metrics = len(metrics)
	coverage.Records = len(records)
	coverage.Documents = len(documents)
	return coverage, nil
}
