package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const crmCarryRecoveryAction = "crm-carry-into-the-record"
const crmCarryTimeout = 30 * time.Second

// What a device recorded about its customers before the record held it. These
// drop only once the record holds all of them.
var carriedCRMTables = []string{
	"activity",
	"opportunity_contact",
	"opportunity",
	"contact",
	"account",
	"crm_carried_rows",
}

// A device seeded with the standard pipelines has exactly these stages.
var crmStagesTheRecordKeeps = map[string]bool{
	"waiting":     true,
	"in_progress": true,
	"review":      true,
	"done":        true,
	"on_hold":     true,
	"lost":        true,
}

// What the record took, kept against the device id so a second sweep neither
// counts a carried row as missing nor loses the identity its links were
// rewritten to. The table is retired with the store it describes.
func carriedCRMRowIDs(ctx context.Context, database *sql.DB) (map[string]string, error) {
	carried := map[string]string{}
	if _, held := countRowsInTable(ctx, database, "crm_carried_rows"); !held {
		return carried, nil
	}
	rows, errorValue := database.QueryContext(ctx, "SELECT device_id, record_id FROM crm_carried_rows")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var deviceID, recordID string
		if errorValue := rows.Scan(&deviceID, &recordID); errorValue != nil {
			return nil, errorValue
		}
		carried[deviceID] = recordID
	}
	return carried, rows.Err()
}

func rememberCarriedCRMRow(ctx context.Context, database *sql.DB, deviceID string, recordID string) {
	if _, errorValue := database.ExecContext(ctx,
		"CREATE TABLE IF NOT EXISTS crm_carried_rows (device_id TEXT PRIMARY KEY, record_id TEXT NOT NULL)"); errorValue != nil {
		return
	}
	database.ExecContext(ctx,
		"INSERT OR IGNORE INTO crm_carried_rows (device_id, record_id) VALUES (?, ?)", deviceID, recordID)
}

type crmCarryReport struct {
	Organizations int      `json:"organizations"`
	Contacts      int      `json:"contacts"`
	Opportunities int      `json:"opportunities"`
	Activities    int      `json:"activities"`
	Dropped       []string `json:"dropped"`
	Refused       []string `json:"refused"`
}

type crmCarry struct {
	client             *centralplane.Client
	database           *sql.DB
	administratorEmail string
	companyID          string
	members            map[string]bool
	recordIDByDeviceID map[string]string
	report             *crmCarryReport
}

// A row the record refuses is reported, not reshaped: it stays here, it is
// named, and the tables stay with it.
func (service *Service) carryTheCRMIntoTheRecord(ctx context.Context) (crmCarryReport, error) {
	report := crmCarryReport{Dropped: []string{}, Refused: []string{}}
	client := service.centralPlane()
	if client == nil {
		return report, fmt.Errorf("this device names no company to carry its CRM into")
	}
	administratorEmail := service.claimedAdminEmail()
	if administratorEmail == "" {
		return report, fmt.Errorf("no administrator is claimed here, and a carried customer record is an administrator's to write")
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return report, errorValue
	}
	defer database.Close()

	companyID, errorValue := client.CompanyRowID(ctx, administratorEmail)
	if errorValue != nil {
		return report, errorValue
	}
	carried, errorValue := carriedCRMRowIDs(ctx, database)
	if errorValue != nil {
		return report, errorValue
	}
	members, errorValue := membersTheCompanyKnows(ctx, client)
	if errorValue != nil {
		return report, errorValue
	}

	carry := &crmCarry{
		client:             client,
		database:           database,
		administratorEmail: administratorEmail,
		companyID:          companyID,
		members:            members,
		recordIDByDeviceID: carried,
		report:             &report,
	}
	if errorValue := carry.organizations(ctx); errorValue != nil {
		return report, errorValue
	}
	if errorValue := carry.contacts(ctx); errorValue != nil {
		return report, errorValue
	}
	if errorValue := carry.opportunities(ctx); errorValue != nil {
		return report, errorValue
	}
	return report, carry.activities(ctx)
}

func membersTheCompanyKnows(ctx context.Context, client *centralplane.Client) (map[string]bool, error) {
	held, errorValue := client.Members(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	members := map[string]bool{}
	for _, member := range held {
		members[member.MemberID] = true
	}
	return members, nil
}

func (carry *crmCarry) ownerOf(deviceOwnerID string, whose string) (string, error) {
	owner := strings.TrimSpace(deviceOwnerID)
	if owner == "" {
		return "", nil
	}
	if !carry.members[owner] {
		return "", fmt.Errorf("%s is owned by somebody this company does not know", whose)
	}
	return owner, nil
}

func (carry *crmCarry) refuse(deviceID string, errorValue error) {
	carry.report.Refused = append(carry.report.Refused, deviceID+": "+errorValue.Error())
}

func (carry *crmCarry) drop(what string) {
	carry.report.Dropped = append(carry.report.Dropped, what)
}

func (carry *crmCarry) remember(ctx context.Context, deviceID string, recordID string) {
	carry.recordIDByDeviceID[deviceID] = recordID
	rememberCarriedCRMRow(ctx, carry.database, deviceID, recordID)
}

func (carry *crmCarry) write(
	ctx context.Context,
	call func(context.Context) (string, error),
) (string, error) {
	carryContext, cancel := context.WithTimeout(ctx, crmCarryTimeout)
	defer cancel()
	return call(carryContext)
}

type heldOrganization struct {
	ID          string
	Name        string
	Status      string
	TypesJSON   string
	TagsJSON    string
	Importance  string
	OwnerID     string
	Address     string
	Description string
	CreatedAt   string
	UpdatedAt   string
	ArchivedAt  string
}

// The store allows one connection, so every row is read and the cursor closed
// before anything is written back through it.
func readHeldOrganizations(ctx context.Context, database *sql.DB) ([]heldOrganization, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, name, status, COALESCE(types, ''), tags, importance, owner_person_id,
	COALESCE(address, ''), COALESCE(description, ''), created_at, updated_at, COALESCE(archived_at, '')
FROM account ORDER BY created_at`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	held := []heldOrganization{}
	for rows.Next() {
		var one heldOrganization
		if errorValue := rows.Scan(&one.ID, &one.Name, &one.Status, &one.TypesJSON, &one.TagsJSON,
			&one.Importance, &one.OwnerID, &one.Address, &one.Description, &one.CreatedAt,
			&one.UpdatedAt, &one.ArchivedAt); errorValue != nil {
			return nil, errorValue
		}
		held = append(held, one)
	}
	return held, rows.Err()
}

func (carry *crmCarry) organizations(ctx context.Context) error {
	held, errorValue := readHeldOrganizations(ctx, carry.database)
	if errorValue != nil {
		return errorValue
	}
	for _, one := range held {
		if _, taken := carry.recordIDByDeviceID[one.ID]; taken {
			continue
		}
		written, errorValue := momentsOf(one.CreatedAt, one.UpdatedAt)
		if errorValue != nil {
			carry.refuse(one.ID, errorValue)
			continue
		}
		owner, errorValue := carry.ownerOf(one.OwnerID, one.Name)
		if errorValue != nil {
			carry.refuse(one.ID, errorValue)
			continue
		}
		recordID, errorValue := carry.write(ctx, func(callContext context.Context) (string, error) {
			return carry.client.CarryCRMOrganization(callContext, carry.administratorEmail, carry.companyID,
				centralplane.CarriedCRMOrganization{
					Name:        one.Name,
					Status:      one.Status,
					Types:       stringsOfJSONArray(one.TypesJSON),
					Tags:        stringsOfJSONArray(one.TagsJSON),
					Importance:  one.Importance,
					OwnerID:     owner,
					Address:     one.Address,
					Description: one.Description,
					CreatedAt:   written.createdAt,
					UpdatedAt:   written.updatedAt,
					ArchivedAt:  one.ArchivedAt,
				})
		})
		if errorValue != nil {
			carry.refuse(one.ID, errorValue)
			continue
		}
		carry.remember(ctx, one.ID, recordID)
		carry.report.Organizations++
	}
	return nil
}

type carriedMoments struct {
	createdAt time.Time
	updatedAt time.Time
}

func momentsOf(createdAt string, updatedAt string) (carriedMoments, error) {
	created, errorValue := time.Parse(time.RFC3339, createdAt)
	if errorValue != nil {
		return carriedMoments{}, fmt.Errorf("created at %q is not a moment", createdAt)
	}
	updated, errorValue := time.Parse(time.RFC3339, updatedAt)
	if errorValue != nil {
		return carriedMoments{}, fmt.Errorf("updated at %q is not a moment", updatedAt)
	}
	return carriedMoments{createdAt: created, updatedAt: updated}, nil
}

func stringsOfJSONArray(document string) []string {
	written := strings.TrimSpace(document)
	if written == "" {
		return []string{}
	}
	var values []string
	if json.Unmarshal([]byte(written), &values) != nil {
		return []string{}
	}
	return values
}

type heldContact struct {
	ID          string
	AccountID   string
	Name        string
	Email       string
	Phone       string
	Title       string
	Department  string
	Description string
	CreatedAt   string
	UpdatedAt   string
	ArchivedAt  string
}

func readHeldContacts(ctx context.Context, database *sql.DB) ([]heldContact, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, COALESCE(account_id, ''), name, COALESCE(email, ''), COALESCE(phone, ''),
	COALESCE(title, ''), COALESCE(department, ''), COALESCE(description, ''),
	created_at, updated_at, COALESCE(archived_at, '')
FROM contact ORDER BY created_at`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	held := []heldContact{}
	for rows.Next() {
		var one heldContact
		if errorValue := rows.Scan(&one.ID, &one.AccountID, &one.Name, &one.Email, &one.Phone,
			&one.Title, &one.Department, &one.Description, &one.CreatedAt, &one.UpdatedAt,
			&one.ArchivedAt); errorValue != nil {
			return nil, errorValue
		}
		held = append(held, one)
	}
	return held, rows.Err()
}

func (carry *crmCarry) contacts(ctx context.Context) error {
	held, errorValue := readHeldContacts(ctx, carry.database)
	if errorValue != nil {
		return errorValue
	}
	for _, one := range held {
		if _, taken := carry.recordIDByDeviceID[one.ID]; taken {
			continue
		}
		written, errorValue := momentsOf(one.CreatedAt, one.UpdatedAt)
		if errorValue != nil {
			carry.refuse(one.ID, errorValue)
			continue
		}
		organizationID := ""
		if one.AccountID != "" {
			mapped, known := carry.recordIDByDeviceID[one.AccountID]
			if !known {
				carry.refuse(one.ID, fmt.Errorf("%s belongs to a relationship the record did not take", one.Name))
				continue
			}
			organizationID = mapped
		}
		recordID, errorValue := carry.write(ctx, func(callContext context.Context) (string, error) {
			return carry.client.CarryCRMContact(callContext, carry.administratorEmail, carry.companyID,
				centralplane.CarriedCRMContact{
					OrganizationID: organizationID,
					Name:           one.Name,
					Email:          one.Email,
					Phone:          one.Phone,
					Title:          one.Title,
					Department:     one.Department,
					Description:    one.Description,
					CreatedAt:      written.createdAt,
					UpdatedAt:      written.updatedAt,
					ArchivedAt:     one.ArchivedAt,
				})
		})
		if errorValue != nil {
			carry.refuse(one.ID, errorValue)
			continue
		}
		carry.remember(ctx, one.ID, recordID)
		carry.report.Contacts++
	}
	return nil
}

type heldOpportunity struct {
	ID               string
	AccountID        string
	Business         string
	Name             string
	Pipeline         string
	Stage            string
	StagePosition    float64
	StageChangedAt   string
	OwnerPersonID    string
	AmountMinor      *int64
	CurrencyCode     string
	BaseAmountMinor  *int64
	BaseCurrencyCode string
	Importance       string
	DueAt            string
	DueTimeZone      string
	LostReason       string
	Description      string
	CreatedAt        string
	UpdatedAt        string
	ArchivedAt       string
}

type heldOpportunityContact struct {
	ContactID string
	IsPrimary bool
}

func readHeldOpportunities(ctx context.Context, database *sql.DB) ([]heldOpportunity, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, COALESCE(account_id, ''), COALESCE(business, ''), name, pipeline, stage, stage_position,
	stage_changed_at, owner_person_id, amount_minor, currency_code, base_amount_minor,
	COALESCE(base_currency_code, ''), importance, COALESCE(due_at, ''), COALESCE(due_time_zone, ''),
	COALESCE(lost_reason, ''), COALESCE(description, ''), created_at, updated_at,
	COALESCE(archived_at, '')
FROM opportunity ORDER BY created_at`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	held := []heldOpportunity{}
	for rows.Next() {
		var one heldOpportunity
		if errorValue := rows.Scan(&one.ID, &one.AccountID, &one.Business, &one.Name, &one.Pipeline,
			&one.Stage, &one.StagePosition, &one.StageChangedAt, &one.OwnerPersonID, &one.AmountMinor,
			&one.CurrencyCode, &one.BaseAmountMinor, &one.BaseCurrencyCode, &one.Importance,
			&one.DueAt, &one.DueTimeZone, &one.LostReason, &one.Description, &one.CreatedAt,
			&one.UpdatedAt, &one.ArchivedAt); errorValue != nil {
			return nil, errorValue
		}
		held = append(held, one)
	}
	return held, rows.Err()
}

func readHeldOpportunityContacts(ctx context.Context, database *sql.DB) (map[string][]heldOpportunityContact, error) {
	if _, held := countRowsInTable(ctx, database, "opportunity_contact"); !held {
		return map[string][]heldOpportunityContact{}, nil
	}
	rows, errorValue := database.QueryContext(ctx, `
SELECT opportunity_id, contact_id, is_primary FROM opportunity_contact
ORDER BY opportunity_id, is_primary DESC, contact_id`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	linked := map[string][]heldOpportunityContact{}
	for rows.Next() {
		var opportunityID string
		var one heldOpportunityContact
		if errorValue := rows.Scan(&opportunityID, &one.ContactID, &one.IsPrimary); errorValue != nil {
			return nil, errorValue
		}
		linked[opportunityID] = append(linked[opportunityID], one)
	}
	return linked, rows.Err()
}

func (carry *crmCarry) opportunities(ctx context.Context) error {
	held, errorValue := readHeldOpportunities(ctx, carry.database)
	if errorValue != nil {
		return errorValue
	}
	linked, errorValue := readHeldOpportunityContacts(ctx, carry.database)
	if errorValue != nil {
		return errorValue
	}
	for _, one := range held {
		if _, taken := carry.recordIDByDeviceID[one.ID]; taken {
			continue
		}
		carried, errorValue := carry.carriedOpportunityOf(one, linked[one.ID])
		if errorValue != nil {
			carry.refuse(one.ID, errorValue)
			continue
		}
		recordID, errorValue := carry.write(ctx, func(callContext context.Context) (string, error) {
			return carry.client.CarryCRMOpportunity(callContext, carry.administratorEmail, carry.companyID, carried)
		})
		if errorValue != nil {
			carry.refuse(one.ID, errorValue)
			continue
		}
		carry.remember(ctx, one.ID, recordID)
		carry.report.Opportunities++
	}
	return nil
}

func (carry *crmCarry) carriedOpportunityOf(
	held heldOpportunity,
	linked []heldOpportunityContact,
) (centralplane.CarriedCRMOpportunity, error) {
	written, errorValue := momentsOf(held.CreatedAt, held.UpdatedAt)
	if errorValue != nil {
		return centralplane.CarriedCRMOpportunity{}, errorValue
	}
	movedAt, errorValue := time.Parse(time.RFC3339, held.StageChangedAt)
	if errorValue != nil {
		return centralplane.CarriedCRMOpportunity{}, fmt.Errorf("stage changed at %q is not a moment", held.StageChangedAt)
	}
	if !crmStagesTheRecordKeeps[held.Stage] {
		return centralplane.CarriedCRMOpportunity{}, fmt.Errorf("stage %q is not one the record keeps", held.Stage)
	}
	if strings.TrimSpace(held.AccountID) == "" {
		return centralplane.CarriedCRMOpportunity{}, fmt.Errorf("%s is against a contact rather than a relationship, and the record keeps every deal under one", held.Name)
	}
	organizationID, known := carry.recordIDByDeviceID[held.AccountID]
	if !known {
		return centralplane.CarriedCRMOpportunity{}, fmt.Errorf("%s belongs to a relationship the record did not take", held.Name)
	}
	owner, errorValue := carry.ownerOf(held.OwnerPersonID, held.Name)
	if errorValue != nil {
		return centralplane.CarriedCRMOpportunity{}, errorValue
	}
	return centralplane.CarriedCRMOpportunity{
		OrganizationID:   organizationID,
		ContactID:        carry.contactTheDealIsWith(held.Name, linked),
		Name:             held.Name,
		Business:         held.Business,
		PipelineID:       held.Pipeline,
		StageID:          held.Stage,
		StagePosition:    int(math.Round(held.StagePosition)),
		StageChangedAt:   movedAt,
		OwnerID:          owner,
		AmountMinor:      held.AmountMinor,
		CurrencyCode:     held.CurrencyCode,
		BaseAmountMinor:  held.BaseAmountMinor,
		BaseCurrencyCode: held.BaseCurrencyCode,
		Importance:       held.Importance,
		DueAt:            held.DueAt,
		DueTimeZone:      held.DueTimeZone,
		LostReasonID:     held.LostReason,
		Description:      held.Description,
		CreatedAt:        written.createdAt,
		UpdatedAt:        written.updatedAt,
		ArchivedAt:       held.ArchivedAt,
	}, nil
}

// opportunity_contact lets a device deal name several contacts; the record
// keeps one.
func (carry *crmCarry) contactTheDealIsWith(name string, linked []heldOpportunityContact) string {
	settled := ""
	for _, one := range linked {
		mapped, known := carry.recordIDByDeviceID[one.ContactID]
		if !known {
			carry.drop(name + ": " + one.ContactID + " was linked to it and the record did not take that contact")
			continue
		}
		if settled == "" {
			settled = mapped
			continue
		}
		carry.drop(name + ": " + one.ContactID + " was linked to it beside the contact it is with")
	}
	return settled
}

type heldActivity struct {
	ID            string
	AccountID     string
	ContactID     string
	OpportunityID string
	Business      string
	Kind          string
	Title         string
	OccurredAt    string
	Content       string
	CreatedAt     string
	UpdatedAt     string
}

func readHeldActivities(ctx context.Context, database *sql.DB) ([]heldActivity, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, COALESCE(account_id, ''), COALESCE(contact_id, ''), COALESCE(opportunity_id, ''),
	COALESCE(business, ''), kind, title, occurred_at, COALESCE(content, ''), created_at, updated_at
FROM activity ORDER BY occurred_at`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	held := []heldActivity{}
	for rows.Next() {
		var one heldActivity
		if errorValue := rows.Scan(&one.ID, &one.AccountID, &one.ContactID, &one.OpportunityID,
			&one.Business, &one.Kind, &one.Title, &one.OccurredAt, &one.Content, &one.CreatedAt,
			&one.UpdatedAt); errorValue != nil {
			return nil, errorValue
		}
		held = append(held, one)
	}
	return held, rows.Err()
}

func (carry *crmCarry) activities(ctx context.Context) error {
	held, errorValue := readHeldActivities(ctx, carry.database)
	if errorValue != nil {
		return errorValue
	}
	deals, errorValue := readHeldOpportunities(ctx, carry.database)
	if errorValue != nil {
		return errorValue
	}
	nameOfDeal := map[string]string{}
	for _, deal := range deals {
		nameOfDeal[deal.ID] = deal.Name
	}
	for _, one := range held {
		if _, taken := carry.recordIDByDeviceID[one.ID]; taken {
			continue
		}
		written, errorValue := momentsOf(one.CreatedAt, one.UpdatedAt)
		if errorValue != nil {
			carry.refuse(one.ID, errorValue)
			continue
		}
		carried := centralplane.CarriedCRMActivity{
			OrganizationID: carry.recordIDByDeviceID[one.AccountID],
			OpportunityID:  carry.recordIDByDeviceID[one.OpportunityID],
			ContactID:      carry.recordIDByDeviceID[one.ContactID],
			Title:          one.Title,
			Note:           one.Content,
			Business:       one.Business,
			Kind:           one.Kind,
			Status:         "completed",
			OccurredAt:     one.OccurredAt,
			CreatedAt:      written.createdAt,
			UpdatedAt:      written.updatedAt,
		}
		// The device wrote the move into the title; the record's own trigger
		// writes the deal's name there and the move underneath.
		if one.Kind == "stage_change" && nameOfDeal[one.OpportunityID] != "" {
			carried.Title = nameOfDeal[one.OpportunityID]
			carried.Note = one.Title
		}
		recordID, errorValue := carry.write(ctx, func(callContext context.Context) (string, error) {
			return carry.client.CarryCRMActivity(callContext, carry.administratorEmail, carry.companyID, carried)
		})
		if errorValue != nil {
			carry.refuse(one.ID, errorValue)
			continue
		}
		carry.remember(ctx, one.ID, recordID)
		carry.report.Activities++
	}
	return nil
}
