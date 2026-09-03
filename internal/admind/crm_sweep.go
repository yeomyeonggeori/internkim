package admind

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// A link between a CRM row and a task, a calendar event, a message or a file.
// The record has no place for one, so a device holding any keeps its store.
var crmTablesWithNowhereToGo = map[string]string{
	"resource_link": "https://github.com/yeomyeonggeori/internkim/issues/1331",
}

func crmTablesTheRecordCannotTake(ctx context.Context, database *sql.DB) []string {
	pinned := []string{}
	for tableName, why := range crmTablesWithNowhereToGo {
		rowCount, held := countRowsInTable(ctx, database, tableName)
		if held && rowCount > 0 {
			pinned = append(pinned, tableName+" ("+why+")")
		}
	}
	return pinned
}

// Every customer row this device holds that the record has not taken. A carried
// row is remembered by its device id, so asking again after a carry answers
// zero.
func crmTheRecordDoesNotHold(ctx context.Context, database *sql.DB) (int, error) {
	carried, errorValue := carriedCRMRowIDs(ctx, database)
	if errorValue != nil {
		return 0, errorValue
	}
	uncarried := 0
	for _, tableName := range []string{"account", "contact", "opportunity", "activity"} {
		ids, errorValue := crmRowIDsOf(ctx, database, tableName)
		if errorValue != nil {
			return 0, errorValue
		}
		for _, id := range ids {
			if _, taken := carried[id]; !taken {
				uncarried++
			}
		}
	}
	return uncarried, nil
}

func crmRowIDsOf(ctx context.Context, database *sql.DB, tableName string) ([]string, error) {
	if _, held := countRowsInTable(ctx, database, tableName); !held {
		return nil, nil
	}
	rows, errorValue := database.QueryContext(ctx, "SELECT id FROM "+tableName)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if errorValue := rows.Scan(&id); errorValue != nil {
			return nil, errorValue
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// A device that names no company covers nothing, so it keeps everything it
// holds. Reading this the other way round is how a company's customers get
// dropped by a device the record has never heard of.
func (service *Service) sweepTheCRMTheRecordNowHolds(ctx context.Context) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return
	}
	defer database.Close()

	uncovered, errorValue := crmTheRecordDoesNotHold(ctx, database)
	if errorValue != nil {
		slog.WarnContext(ctx, "the CRM this device still holds could not be counted, so none of it was let go",
			"error", errorValue)
		return
	}
	if uncovered > 0 && service.centralPlane() == nil {
		slog.WarnContext(ctx, "this device names no company, so it keeps the CRM nobody else holds",
			"uncovered", uncovered, "recovery_action", crmCarryRecoveryAction)
		return
	}
	if uncovered > 0 {
		slog.WarnContext(ctx, "this device holds a CRM the record does not, so its store stays",
			"uncovered", uncovered, "recovery_action", crmCarryRecoveryAction)
		return
	}
	pinned := crmTablesTheRecordCannotTake(ctx, database)
	if len(pinned) > 0 {
		slog.WarnContext(ctx, "this device holds CRM rows the record has no place for, so its store stays",
			"tables", strings.Join(pinned, ","), "recovery_action", crmCarryRecoveryAction)
		return
	}
	if errorValue := dropCRMTables(ctx, database); errorValue != nil {
		slog.WarnContext(ctx, "a CRM table the record now holds could not be dropped", "error", errorValue)
	}
}

func dropCRMTables(ctx context.Context, database *sql.DB) error {
	for _, tableName := range carriedCRMTables {
		rowCount, held := countRowsInTable(ctx, database, tableName)
		if !held {
			continue
		}
		if _, errorValue := database.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName); errorValue != nil {
			return errorValue
		}
		slog.InfoContext(ctx, "dropped a CRM table the record now holds",
			"table", tableName, "rows", rowCount)
	}
	return nil
}

type crmRecordCoverage struct {
	Uncovered     int      `json:"uncovered"`
	Pinned        []string `json:"pinned"`
	Organizations int      `json:"organizations"`
	Contacts      int      `json:"contacts"`
	Opportunities int      `json:"opportunities"`
	Activities    int      `json:"activities"`
	Dropped       []string `json:"dropped"`
	Refused       []string `json:"refused"`
}

func (service *Service) handleCRMRecordCoverage(responseWriter http.ResponseWriter, request *http.Request) {
	coverage, errorValue := service.crmCoverageOfTheRecord(request)
	if errorValue != nil {
		http.Error(responseWriter, "crm_coverage_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, coverage)
}

func (service *Service) crmCoverageOfTheRecord(request *http.Request) (crmRecordCoverage, error) {
	coverage := crmRecordCoverage{Pinned: []string{}, Dropped: []string{}, Refused: []string{}}
	uncovered, pinned, errorValue := service.crmStillHeldHere(request.Context())
	if errorValue != nil {
		return coverage, errorValue
	}
	coverage.Uncovered = uncovered
	coverage.Pinned = pinned
	if request.URL.Query().Get("carry") != "true" {
		return coverage, nil
	}
	report, errorValue := service.carryTheCRMIntoTheRecord(request.Context())
	if errorValue != nil {
		return coverage, errorValue
	}
	coverage.Organizations = report.Organizations
	coverage.Contacts = report.Contacts
	coverage.Opportunities = report.Opportunities
	coverage.Activities = report.Activities
	coverage.Dropped = report.Dropped
	coverage.Refused = report.Refused
	service.sweepTheCRMTheRecordNowHolds(request.Context())
	return coverage, nil
}

func (service *Service) crmStillHeldHere(ctx context.Context) (int, []string, error) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return 0, nil, errorValue
	}
	defer database.Close()
	uncovered, errorValue := crmTheRecordDoesNotHold(ctx, database)
	if errorValue != nil {
		return 0, nil, fmt.Errorf("the CRM this device still holds could not be counted: %w", errorValue)
	}
	return uncovered, crmTablesTheRecordCannotTake(ctx, database), nil
}

func (service *Service) startCRMSweep(ctx context.Context) {
	go service.sweepTheCRMTheRecordNowHolds(ctx)
}
