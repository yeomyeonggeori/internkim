package admind

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const organizationCarryRecoveryAction = "organization-carry-into-the-record"
const organizationCarryTimeout = 30 * time.Second

// A device that names no company covers nothing, so it counts everything it
// holds as uncovered and keeps its store.
func (service *Service) sweepTheOrganizationTheCompanyNowHolds(ctx context.Context) {
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return
	}
	defer database.Close()

	if errorValue := dropOrganizationTables(ctx, database, derivedOrganizationTables); errorValue != nil {
		slog.WarnContext(ctx, "a derived organization table could not be dropped", "error", errorValue)
		return
	}

	uncovered, errorValue := service.organizationProfilesTheRecordDoesNotHold(ctx, database)
	if errorValue != nil {
		slog.WarnContext(ctx, "the organization profiles this device still holds could not be counted, so none of them were let go",
			"error", errorValue)
		return
	}
	if uncovered > 0 {
		slog.WarnContext(ctx, "this device holds organization profiles the record does not, so its store stays",
			"uncovered", uncovered, "recovery_action", organizationCarryRecoveryAction)
		return
	}
	if errorValue := dropOrganizationTables(ctx, database, carriedOrganizationTables); errorValue != nil {
		slog.WarnContext(ctx, "an organization table the company now holds could not be dropped", "error", errorValue)
	}
}

func (service *Service) organizationProfilesTheRecordDoesNotHold(ctx context.Context, database *sql.DB) (int, error) {
	if service.centralPlane() == nil {
		rowCount, held := countRowsInOrganizationTable(ctx, database, "organization_profiles")
		if !held {
			return 0, nil
		}
		return rowCount, nil
	}
	profiles, errorValue := uncarriedOrganizationProfiles(ctx, database)
	if errorValue != nil {
		return 0, errorValue
	}
	return len(profiles), nil
}

const deviceOrganizationProfileQuery = `
SELECT profile_key, user_id, email, job_title, group_id, phone_number, hire_date, supervisor_id, status
FROM organization_profiles
ORDER BY email ASC`

// Every profile this device holds that the record has not taken. A carried row
// is remembered by its key, so asking again after a carry answers zero.
func uncarriedOrganizationProfiles(ctx context.Context, database *sql.DB) ([]deviceOrganizationProfile, error) {
	alreadyCarried, errorValue := carriedOrganizationRowKeys(ctx, database)
	if errorValue != nil {
		return nil, errorValue
	}
	if _, held := countRowsInOrganizationTable(ctx, database, "organization_profiles"); !held {
		return nil, nil
	}
	rows, errorValue := database.QueryContext(ctx, deviceOrganizationProfileQuery)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	profiles := []deviceOrganizationProfile{}
	for rows.Next() {
		var profile deviceOrganizationProfile
		if errorValue := rows.Scan(&profile.Key, &profile.MemberID, &profile.Email, &profile.JobTitle,
			&profile.GroupID, &profile.PhoneNumber, &profile.HireDate, &profile.SupervisorID, &profile.Status); errorValue != nil {
			return nil, errorValue
		}
		if _, carried := alreadyCarried[profile.Key]; carried {
			continue
		}
		if !profile.describesSomebody() {
			continue
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

type deviceOrganizationProfile struct {
	Key          string
	MemberID     string
	Email        string
	JobTitle     string
	GroupID      string
	PhoneNumber  string
	HireDate     string
	SupervisorID string
	Status       string
}

// A row that says nothing about a person is not a profile the record is missing.
// The store wrote one for everybody the directory named, described or not.
func (profile deviceOrganizationProfile) describesSomebody() bool {
	return strings.TrimSpace(profile.JobTitle) != "" ||
		strings.TrimSpace(profile.PhoneNumber) != "" ||
		strings.TrimSpace(profile.HireDate) != "" ||
		strings.TrimSpace(profile.GroupID) != "" ||
		strings.TrimSpace(profile.SupervisorID) != ""
}

// What the record took, kept against the local key so a second sweep does not
// count a carried row as still missing. The table is retired with the store it
// describes, so it never outlives what it is about.
func carriedOrganizationRowKeys(ctx context.Context, database *sql.DB) (map[string]struct{}, error) {
	carried := map[string]struct{}{}
	if _, held := countRowsInOrganizationTable(ctx, database, "organization_carried_rows"); !held {
		return carried, nil
	}
	rows, errorValue := database.QueryContext(ctx, "SELECT profile_key FROM organization_carried_rows")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var profileKey string
		if errorValue := rows.Scan(&profileKey); errorValue != nil {
			return nil, errorValue
		}
		carried[profileKey] = struct{}{}
	}
	return carried, rows.Err()
}

func rememberCarriedOrganizationRow(ctx context.Context, database *sql.DB, profileKey string) {
	_, _ = database.ExecContext(ctx,
		"CREATE TABLE IF NOT EXISTS organization_carried_rows (profile_key TEXT PRIMARY KEY, carried_at TEXT NOT NULL)")
	_, _ = database.ExecContext(ctx,
		"INSERT OR REPLACE INTO organization_carried_rows (profile_key, carried_at) VALUES (?, ?)",
		profileKey, time.Now().UTC().Format(time.RFC3339))
}

type organizationCarryReport struct {
	Profiles int      `json:"profiles"`
	Refused  []string `json:"refused,omitempty"`
}

// The record's organization profile is an administrator's to write, and a
// background carry acts as nobody, so it acts as the administrator this device
// was claimed by.
func (service *Service) carryTheOrganizationIntoTheRecord(ctx context.Context) (organizationCarryReport, error) {
	report := organizationCarryReport{}
	client := service.centralPlane()
	if client == nil {
		return report, errNoCompanyDirectory
	}
	administratorEmail := service.claimedAdminEmail()
	if administratorEmail == "" {
		return report, fmt.Errorf("no administrator is claimed here, and an organization profile is an administrator's to record")
	}

	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return report, errorValue
	}
	defer database.Close()

	profiles, errorValue := uncarriedOrganizationProfiles(ctx, database)
	if errorValue != nil {
		return report, errorValue
	}
	if len(profiles) == 0 {
		return report, nil
	}

	groupNameByID, errorValue := deviceOrganizationGroupNames(ctx, database)
	if errorValue != nil {
		return report, errorValue
	}
	emailByMemberID := map[string]string{}
	for _, profile := range profiles {
		if profile.MemberID != "" && profile.Email != "" {
			emailByMemberID[profile.MemberID] = strings.ToLower(strings.TrimSpace(profile.Email))
		}
	}

	for _, profile := range profiles {
		email := strings.ToLower(strings.TrimSpace(profile.Email))
		if email == "" {
			report.Refused = append(report.Refused, profile.Key+": the row names no address, and the record knows a person by theirs")
			continue
		}
		carryContext, cancel := context.WithTimeout(ctx, organizationCarryTimeout)
		_, errorValue := client.WriteOrganizationProfiles(carryContext, []centralplane.OrganizationProfile{{
			Email:           email,
			JobTitle:        strings.TrimSpace(profile.JobTitle),
			PhoneNumber:     strings.TrimSpace(profile.PhoneNumber),
			HireDate:        strings.TrimSpace(profile.HireDate),
			SupervisorEmail: emailByMemberID[strings.TrimSpace(profile.SupervisorID)],
			TeamName:        groupNameByID[strings.TrimSpace(profile.GroupID)],
		}})
		cancel()
		if errorValue != nil {
			report.Refused = append(report.Refused, profile.Key+": "+errorValue.Error())
			continue
		}
		rememberCarriedOrganizationRow(ctx, database, profile.Key)
		report.Profiles++
	}
	return report, nil
}

func deviceOrganizationGroupNames(ctx context.Context, database *sql.DB) (map[string]string, error) {
	nameByID := map[string]string{}
	if _, held := countRowsInOrganizationTable(ctx, database, "organization_groups"); !held {
		return nameByID, nil
	}
	rows, errorValue := database.QueryContext(ctx, "SELECT id, name FROM organization_groups")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if errorValue := rows.Scan(&id, &name); errorValue != nil {
			return nil, errorValue
		}
		nameByID[strings.TrimSpace(id)] = strings.TrimSpace(name)
	}
	return nameByID, rows.Err()
}
