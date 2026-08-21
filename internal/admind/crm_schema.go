package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const crmOpportunityTableColumns = `
		id TEXT PRIMARY KEY CHECK(trim(id) <> ''),
		account_id TEXT,
		business TEXT,
		name TEXT NOT NULL CHECK(trim(name) <> ''),
		pipeline TEXT NOT NULL,
		stage TEXT NOT NULL,
		stage_position REAL NOT NULL,
		stage_changed_at TEXT NOT NULL CHECK(stage_changed_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		owner_person_id TEXT NOT NULL CHECK(trim(owner_person_id) <> ''),
		owner_circle_id TEXT,
		amount_minor INTEGER CHECK(amount_minor IS NULL OR amount_minor >= 0),
		currency_code TEXT NOT NULL CHECK(currency_code IN ('KRW', 'USD', 'JPY', 'EUR')),
		base_amount_minor INTEGER CHECK(base_amount_minor IS NULL OR base_amount_minor >= 0),
		base_currency_code TEXT CHECK(base_currency_code IS NULL OR base_currency_code IN ('KRW', 'USD', 'JPY', 'EUR')),
		importance TEXT NOT NULL CHECK(importance IN ('high', 'medium', 'low')),
		due_at TEXT CHECK(due_at IS NULL OR due_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		due_time_zone TEXT,
		lost_reason TEXT,
		description TEXT,
		created_at TEXT NOT NULL CHECK(created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		created_by_person_id TEXT NOT NULL CHECK(trim(created_by_person_id) <> ''),
		updated_at TEXT NOT NULL CHECK(updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		updated_by_person_id TEXT NOT NULL CHECK(trim(updated_by_person_id) <> ''),
		archived_at TEXT CHECK(archived_at IS NULL OR archived_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		archived_by_person_id TEXT,
		CHECK((due_at IS NULL) = (due_time_zone IS NULL)),
		CHECK((base_amount_minor IS NULL) = (base_currency_code IS NULL)),
		CHECK((archived_at IS NULL) = (archived_by_person_id IS NULL)),
		FOREIGN KEY (account_id) REFERENCES account(id) ON DELETE RESTRICT,
		FOREIGN KEY (pipeline, stage) REFERENCES pipeline_stage(pipeline, stage) ON DELETE RESTRICT
	`

var crmSchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS pipeline (
		pipeline TEXT PRIMARY KEY,
		label TEXT NOT NULL CHECK(trim(label) <> ''),
		direction TEXT NOT NULL CHECK(direction IN ('inbound', 'outbound', 'none')),
		is_active INTEGER NOT NULL CHECK(is_active IN (0, 1))
	)`,
	`CREATE TABLE IF NOT EXISTS pipeline_stage (
		pipeline TEXT NOT NULL,
		stage TEXT NOT NULL CHECK(trim(stage) <> ''),
		position INTEGER NOT NULL CHECK(position > 0),
		outcome TEXT NOT NULL CHECK(outcome IN ('open', 'on_hold', 'won', 'lost')),
		PRIMARY KEY (pipeline, stage),
		UNIQUE (pipeline, position),
		FOREIGN KEY (pipeline) REFERENCES pipeline(pipeline) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS lost_reason (
		reason TEXT PRIMARY KEY CHECK(trim(reason) <> ''),
		label TEXT NOT NULL CHECK(trim(label) <> ''),
		is_active INTEGER NOT NULL CHECK(is_active IN (0, 1))
	)`,
	`CREATE TABLE IF NOT EXISTS account (
		id TEXT PRIMARY KEY CHECK(trim(id) <> ''),
		name TEXT NOT NULL CHECK(trim(name) <> ''),
		status TEXT NOT NULL CHECK(status IN ('prospect', 'active', 'paused')),
		types TEXT CHECK(types IS NULL OR (json_valid(types) AND json_type(types) = 'array')),
		tags TEXT NOT NULL CHECK(json_valid(tags) AND json_type(tags) = 'array'),
		importance TEXT NOT NULL CHECK(importance IN ('high', 'medium', 'low')),
		owner_person_id TEXT NOT NULL CHECK(trim(owner_person_id) <> ''),
		owner_circle_id TEXT,
		address TEXT,
		description TEXT,
		created_at TEXT NOT NULL CHECK(created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		created_by_person_id TEXT NOT NULL CHECK(trim(created_by_person_id) <> ''),
		updated_at TEXT NOT NULL CHECK(updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		updated_by_person_id TEXT NOT NULL CHECK(trim(updated_by_person_id) <> ''),
		archived_at TEXT CHECK(archived_at IS NULL OR archived_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		archived_by_person_id TEXT,
		CHECK((archived_at IS NULL) = (archived_by_person_id IS NULL))
	)`,
	`CREATE TABLE IF NOT EXISTS contact (
		id TEXT PRIMARY KEY CHECK(trim(id) <> ''),
		account_id TEXT,
		name TEXT NOT NULL CHECK(trim(name) <> ''),
		email TEXT,
		phone TEXT,
		title TEXT,
		department TEXT,
		is_primary INTEGER NOT NULL CHECK(is_primary IN (0, 1)),
		owner_person_id TEXT NOT NULL CHECK(trim(owner_person_id) <> ''),
		owner_circle_id TEXT,
		description TEXT,
		created_at TEXT NOT NULL CHECK(created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		created_by_person_id TEXT NOT NULL CHECK(trim(created_by_person_id) <> ''),
		updated_at TEXT NOT NULL CHECK(updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		updated_by_person_id TEXT NOT NULL CHECK(trim(updated_by_person_id) <> ''),
		archived_at TEXT CHECK(archived_at IS NULL OR archived_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		archived_by_person_id TEXT,
		CHECK(trim(COALESCE(email, '')) <> '' OR trim(COALESCE(phone, '')) <> ''),
		CHECK((archived_at IS NULL) = (archived_by_person_id IS NULL)),
		FOREIGN KEY (account_id) REFERENCES account(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS opportunity (` + crmOpportunityTableColumns + `)`,
	`CREATE TABLE IF NOT EXISTS opportunity_contact (
		opportunity_id TEXT NOT NULL,
		contact_id TEXT NOT NULL,
		is_primary INTEGER NOT NULL CHECK(is_primary IN (0, 1)),
		PRIMARY KEY (opportunity_id, contact_id),
		FOREIGN KEY (opportunity_id) REFERENCES opportunity(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY (contact_id) REFERENCES contact(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS activity (
		id TEXT PRIMARY KEY CHECK(trim(id) <> ''),
		account_id TEXT,
		contact_id TEXT,
		opportunity_id TEXT,
		business TEXT,
		kind TEXT NOT NULL CHECK(kind IN ('note', 'email', 'meeting', 'call', 'task', 'file', 'event', 'stage_change')),
		title TEXT NOT NULL CHECK(trim(title) <> ''),
		occurred_at TEXT NOT NULL CHECK(occurred_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		content TEXT,
		created_at TEXT NOT NULL CHECK(created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		created_by_person_id TEXT NOT NULL CHECK(trim(created_by_person_id) <> ''),
		updated_at TEXT NOT NULL CHECK(updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		updated_by_person_id TEXT NOT NULL CHECK(trim(updated_by_person_id) <> ''),
		archived_at TEXT CHECK(archived_at IS NULL OR archived_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		archived_by_person_id TEXT,
		CHECK(account_id IS NOT NULL OR contact_id IS NOT NULL OR opportunity_id IS NOT NULL),
		CHECK((archived_at IS NULL) = (archived_by_person_id IS NULL)),
		FOREIGN KEY (account_id) REFERENCES account(id) ON DELETE RESTRICT,
		FOREIGN KEY (contact_id) REFERENCES contact(id) ON DELETE RESTRICT,
		FOREIGN KEY (opportunity_id) REFERENCES opportunity(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS resource_link (
		id TEXT PRIMARY KEY CHECK(trim(id) <> ''),
		entity_type TEXT NOT NULL CHECK(entity_type IN ('account', 'contact', 'opportunity', 'activity')),
		entity_id TEXT NOT NULL CHECK(trim(entity_id) <> ''),
		service TEXT NOT NULL CHECK(service IN ('flow', 'calendar', 'mail', 'files')),
		external_resource_type TEXT NOT NULL CHECK(trim(external_resource_type) <> ''),
		external_resource_id TEXT NOT NULL CHECK(trim(external_resource_id) <> ''),
		external_resource_url TEXT,
		created_at TEXT NOT NULL CHECK(created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		created_by_person_id TEXT NOT NULL CHECK(trim(created_by_person_id) <> ''),
		removed_at TEXT CHECK(removed_at IS NULL OR removed_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
		removed_by_person_id TEXT,
		CHECK((removed_at IS NULL) = (removed_by_person_id IS NULL))
	)`,
	`CREATE INDEX IF NOT EXISTS contact_email_lookup ON contact(lower(trim(email)))`,
	`CREATE INDEX IF NOT EXISTS contact_phone_lookup ON contact(replace(replace(replace(phone, '-', ''), ' ', ''), '+82', '0'))`,
	`CREATE UNIQUE INDEX IF NOT EXISTS contact_primary_account ON contact(account_id) WHERE account_id IS NOT NULL AND is_primary = 1 AND archived_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS opportunity_primary_contact ON opportunity_contact(opportunity_id) WHERE is_primary = 1`,
	`CREATE INDEX IF NOT EXISTS opportunity_stage_position ON opportunity(pipeline, stage, stage_position)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS resource_link_active_unique ON resource_link(entity_type, entity_id, service, external_resource_type, external_resource_id) WHERE removed_at IS NULL`,
	`CREATE TRIGGER IF NOT EXISTS account_types_validate_insert
	BEFORE INSERT ON account
	WHEN NEW.types IS NOT NULL AND EXISTS (
		SELECT 1 FROM json_each(NEW.types)
		WHERE type <> 'text' OR value NOT IN ('customer', 'partner', 'sponsor', 'vendor', 'investor', 'portfolio', 'other')
	)
	BEGIN
		SELECT RAISE(ABORT, 'invalid account type');
	END`,
	`CREATE TRIGGER IF NOT EXISTS account_types_validate_update
	BEFORE UPDATE OF types ON account
	WHEN NEW.types IS NOT NULL AND EXISTS (
		SELECT 1 FROM json_each(NEW.types)
		WHERE type <> 'text' OR value NOT IN ('customer', 'partner', 'sponsor', 'vendor', 'investor', 'portfolio', 'other')
	)
	BEGIN
		SELECT RAISE(ABORT, 'invalid account type');
	END`,
	`CREATE TRIGGER IF NOT EXISTS pipeline_stage_outcome_immutable
	BEFORE UPDATE OF outcome ON pipeline_stage
	WHEN OLD.outcome <> NEW.outcome AND EXISTS (
		SELECT 1 FROM opportunity WHERE pipeline = OLD.pipeline AND stage = OLD.stage
	)
	BEGIN
		SELECT RAISE(ABORT, 'cannot change outcome of a stage used by opportunities');
	END`,
	`CREATE TRIGGER IF NOT EXISTS opportunity_validate_insert
	BEFORE INSERT ON opportunity
	BEGIN
		SELECT CASE
			WHEN NEW.account_id IS NULL AND NOT EXISTS (
				SELECT 1 FROM opportunity_contact WHERE opportunity_id = NEW.id
			) THEN RAISE(ABORT, 'opportunity requires an account or contact')
			WHEN (SELECT outcome FROM pipeline_stage WHERE pipeline = NEW.pipeline AND stage = NEW.stage) = 'lost'
				AND NEW.lost_reason IS NULL THEN RAISE(ABORT, 'lost opportunity requires lost_reason')
			WHEN (SELECT outcome FROM pipeline_stage WHERE pipeline = NEW.pipeline AND stage = NEW.stage) <> 'lost'
				AND NEW.lost_reason IS NOT NULL THEN RAISE(ABORT, 'lost_reason is only valid for lost opportunities')
			WHEN (SELECT outcome FROM pipeline_stage WHERE pipeline = NEW.pipeline AND stage = NEW.stage) IN ('open', 'on_hold')
				AND (NEW.base_amount_minor IS NOT NULL OR NEW.base_currency_code IS NOT NULL) THEN RAISE(ABORT, 'unrealized opportunity cannot have base amount')
			WHEN (SELECT outcome FROM pipeline_stage WHERE pipeline = NEW.pipeline AND stage = NEW.stage) IN ('won', 'lost')
				AND NEW.amount_minor IS NOT NULL AND (NEW.base_amount_minor IS NULL OR NEW.base_currency_code IS NULL) THEN RAISE(ABORT, 'realized opportunity amount requires base amount')
		END;
	END`,
	`CREATE TRIGGER IF NOT EXISTS opportunity_validate_update
	BEFORE UPDATE ON opportunity
	BEGIN
		SELECT CASE
			WHEN NEW.account_id IS NULL AND NOT EXISTS (
				SELECT 1 FROM opportunity_contact WHERE opportunity_id = NEW.id
			) THEN RAISE(ABORT, 'opportunity requires an account or contact')
			WHEN (SELECT outcome FROM pipeline_stage WHERE pipeline = NEW.pipeline AND stage = NEW.stage) = 'lost'
				AND NEW.lost_reason IS NULL THEN RAISE(ABORT, 'lost opportunity requires lost_reason')
			WHEN (SELECT outcome FROM pipeline_stage WHERE pipeline = NEW.pipeline AND stage = NEW.stage) <> 'lost'
				AND NEW.lost_reason IS NOT NULL THEN RAISE(ABORT, 'lost_reason is only valid for lost opportunities')
			WHEN (SELECT outcome FROM pipeline_stage WHERE pipeline = NEW.pipeline AND stage = NEW.stage) IN ('open', 'on_hold')
				AND (NEW.base_amount_minor IS NOT NULL OR NEW.base_currency_code IS NOT NULL) THEN RAISE(ABORT, 'unrealized opportunity cannot have base amount')
			WHEN (SELECT outcome FROM pipeline_stage WHERE pipeline = NEW.pipeline AND stage = NEW.stage) IN ('won', 'lost')
				AND NEW.amount_minor IS NOT NULL AND (NEW.base_amount_minor IS NULL OR NEW.base_currency_code IS NULL) THEN RAISE(ABORT, 'realized opportunity amount requires base amount')
		END;
		END`,
	`CREATE TRIGGER IF NOT EXISTS opportunity_realized_amount_immutable
		BEFORE UPDATE OF amount_minor, currency_code, base_amount_minor, base_currency_code ON opportunity
		WHEN (SELECT outcome FROM pipeline_stage WHERE pipeline = OLD.pipeline AND stage = OLD.stage) IN ('won', 'lost')
			AND (
				NEW.amount_minor IS NOT OLD.amount_minor
				OR NEW.currency_code IS NOT OLD.currency_code
				OR NEW.base_amount_minor IS NOT OLD.base_amount_minor
				OR NEW.base_currency_code IS NOT OLD.base_currency_code
			)
		BEGIN
			SELECT RAISE(ABORT, 'realized opportunity amount is immutable');
		END`,
	`CREATE TRIGGER IF NOT EXISTS opportunity_contact_validate_insert
	BEFORE INSERT ON opportunity_contact
	WHEN EXISTS (SELECT 1 FROM opportunity WHERE id = NEW.opportunity_id)
	BEGIN
		SELECT CASE
			WHEN (SELECT account_id FROM opportunity WHERE id = NEW.opportunity_id) IS NOT NULL
				AND (SELECT account_id FROM contact WHERE id = NEW.contact_id) IS NOT (SELECT account_id FROM opportunity WHERE id = NEW.opportunity_id)
			THEN RAISE(ABORT, 'opportunity contact account mismatch')
		END;
	END`,
	`CREATE TRIGGER IF NOT EXISTS contact_validate_opportunities_after_account_update
	BEFORE UPDATE OF account_id ON contact
	WHEN EXISTS (
		SELECT 1
		FROM opportunity_contact oc
		JOIN opportunity o ON o.id = oc.opportunity_id
		WHERE oc.contact_id = OLD.id
			AND o.account_id IS NOT NULL
			AND o.account_id IS NOT NEW.account_id
	)
	BEGIN
		SELECT RAISE(ABORT, 'opportunity contact account mismatch');
	END`,
	`CREATE TRIGGER IF NOT EXISTS opportunity_validate_contacts_after_insert
	AFTER INSERT ON opportunity
	BEGIN
		SELECT CASE WHEN EXISTS (
			SELECT 1
			FROM opportunity_contact oc
			JOIN contact c ON c.id = oc.contact_id
			WHERE oc.opportunity_id = NEW.id
				AND NEW.account_id IS NOT NULL
				AND c.account_id IS NOT NEW.account_id
		) THEN RAISE(ABORT, 'opportunity contact account mismatch') END;
	END`,
	`CREATE TRIGGER IF NOT EXISTS opportunity_validate_contacts_after_update
	AFTER UPDATE OF account_id ON opportunity
	BEGIN
		SELECT CASE WHEN EXISTS (
			SELECT 1
			FROM opportunity_contact oc
			JOIN contact c ON c.id = oc.contact_id
			WHERE oc.opportunity_id = NEW.id
				AND NEW.account_id IS NOT NULL
				AND c.account_id IS NOT NEW.account_id
		) THEN RAISE(ABORT, 'opportunity contact account mismatch') END;
	END`,
	`CREATE TRIGGER IF NOT EXISTS opportunity_contact_prevent_orphan_delete
	BEFORE DELETE ON opportunity_contact
	WHEN (SELECT account_id FROM opportunity WHERE id = OLD.opportunity_id) IS NULL
		AND (SELECT COUNT(*) FROM opportunity_contact WHERE opportunity_id = OLD.opportunity_id) = 1
	BEGIN
		SELECT RAISE(ABORT, 'opportunity requires an account or contact');
	END`,
	`CREATE TRIGGER IF NOT EXISTS activity_validate_insert
	BEFORE INSERT ON activity
	WHEN NEW.opportunity_id IS NOT NULL
	BEGIN
		SELECT CASE
			WHEN NEW.account_id IS NOT (SELECT account_id FROM opportunity WHERE id = NEW.opportunity_id)
				OR NEW.business IS NOT (SELECT business FROM opportunity WHERE id = NEW.opportunity_id)
			THEN RAISE(ABORT, 'activity opportunity fields mismatch')
			WHEN NEW.contact_id IS NOT NULL AND NOT EXISTS (
				SELECT 1 FROM opportunity_contact WHERE opportunity_id = NEW.opportunity_id AND contact_id = NEW.contact_id
			) THEN RAISE(ABORT, 'activity contact is not linked to opportunity')
		END;
	END`,
	`CREATE TRIGGER IF NOT EXISTS activity_validate_update
	BEFORE UPDATE ON activity
	WHEN NEW.opportunity_id IS NOT NULL
	BEGIN
		SELECT CASE
			WHEN NEW.account_id IS NOT (SELECT account_id FROM opportunity WHERE id = NEW.opportunity_id)
				OR NEW.business IS NOT (SELECT business FROM opportunity WHERE id = NEW.opportunity_id)
			THEN RAISE(ABORT, 'activity opportunity fields mismatch')
			WHEN NEW.contact_id IS NOT NULL AND NOT EXISTS (
				SELECT 1 FROM opportunity_contact WHERE opportunity_id = NEW.opportunity_id AND contact_id = NEW.contact_id
			) THEN RAISE(ABORT, 'activity contact is not linked to opportunity')
			END;
		END`,
	`DROP TRIGGER IF EXISTS activity_stage_change_immutable_update`,
	`DROP TRIGGER IF EXISTS activity_stage_change_reentry_update`,
	`CREATE TRIGGER activity_stage_change_reentry_update
		BEFORE UPDATE OF kind ON activity
		WHEN NEW.kind = 'stage_change' AND OLD.kind != 'stage_change'
		BEGIN
			SELECT RAISE(ABORT, 'stage change activity is system generated');
		END`,
	`CREATE TRIGGER IF NOT EXISTS activity_stage_change_immutable_delete
		BEFORE DELETE ON activity
		WHEN OLD.kind = 'stage_change'
		BEGIN
			SELECT RAISE(ABORT, 'stage change activity is immutable');
		END`,
}

var crmPipelineSeeds = []struct {
	pipeline  string
	label     string
	direction string
}{
	{pipeline: "sales", label: "영업", direction: "inbound"},
	{pipeline: "fundraising", label: "투자 유치", direction: "inbound"},
	{pipeline: "sponsorship", label: "후원", direction: "inbound"},
	{pipeline: "investment", label: "투자 집행", direction: "outbound"},
	{pipeline: "procurement", label: "조달", direction: "outbound"},
	{pipeline: "partnership", label: "제휴", direction: "none"},
}

var crmPipelineStageSeeds = []struct {
	pipeline string
	stage    string
	position int
	outcome  string
}{
	{pipeline: "sales", stage: "waiting", position: 1, outcome: "open"},
	{pipeline: "sales", stage: "in_progress", position: 2, outcome: "open"},
	{pipeline: "sales", stage: "review", position: 3, outcome: "open"},
	{pipeline: "sales", stage: "done", position: 4, outcome: "won"},
	{pipeline: "sales", stage: "on_hold", position: 5, outcome: "on_hold"},
	{pipeline: "sales", stage: "lost", position: 6, outcome: "lost"},
	{pipeline: "fundraising", stage: "waiting", position: 1, outcome: "open"},
	{pipeline: "fundraising", stage: "in_progress", position: 2, outcome: "open"},
	{pipeline: "fundraising", stage: "review", position: 3, outcome: "open"},
	{pipeline: "fundraising", stage: "done", position: 4, outcome: "won"},
	{pipeline: "fundraising", stage: "on_hold", position: 5, outcome: "on_hold"},
	{pipeline: "fundraising", stage: "lost", position: 6, outcome: "lost"},
	{pipeline: "sponsorship", stage: "waiting", position: 1, outcome: "open"},
	{pipeline: "sponsorship", stage: "in_progress", position: 2, outcome: "open"},
	{pipeline: "sponsorship", stage: "review", position: 3, outcome: "open"},
	{pipeline: "sponsorship", stage: "done", position: 4, outcome: "won"},
	{pipeline: "sponsorship", stage: "on_hold", position: 5, outcome: "on_hold"},
	{pipeline: "sponsorship", stage: "lost", position: 6, outcome: "lost"},
	{pipeline: "investment", stage: "waiting", position: 1, outcome: "open"},
	{pipeline: "investment", stage: "in_progress", position: 2, outcome: "open"},
	{pipeline: "investment", stage: "review", position: 3, outcome: "open"},
	{pipeline: "investment", stage: "done", position: 4, outcome: "won"},
	{pipeline: "investment", stage: "on_hold", position: 5, outcome: "on_hold"},
	{pipeline: "investment", stage: "lost", position: 6, outcome: "lost"},
	{pipeline: "procurement", stage: "waiting", position: 1, outcome: "open"},
	{pipeline: "procurement", stage: "in_progress", position: 2, outcome: "open"},
	{pipeline: "procurement", stage: "review", position: 3, outcome: "open"},
	{pipeline: "procurement", stage: "done", position: 4, outcome: "won"},
	{pipeline: "procurement", stage: "on_hold", position: 5, outcome: "on_hold"},
	{pipeline: "procurement", stage: "lost", position: 6, outcome: "lost"},
	{pipeline: "partnership", stage: "waiting", position: 1, outcome: "open"},
	{pipeline: "partnership", stage: "in_progress", position: 2, outcome: "open"},
	{pipeline: "partnership", stage: "review", position: 3, outcome: "open"},
	{pipeline: "partnership", stage: "done", position: 4, outcome: "won"},
	{pipeline: "partnership", stage: "on_hold", position: 5, outcome: "on_hold"},
	{pipeline: "partnership", stage: "lost", position: 6, outcome: "lost"},
}

var crmUniformPipelineStages = []struct {
	stage    string
	position int
	outcome  string
}{
	{stage: "waiting", position: 1, outcome: "open"},
	{stage: "in_progress", position: 2, outcome: "open"},
	{stage: "review", position: 3, outcome: "open"},
	{stage: "done", position: 4, outcome: "won"},
	{stage: "on_hold", position: 5, outcome: "on_hold"},
	{stage: "lost", position: 6, outcome: "lost"},
}

var crmLostReasonSeeds = []struct {
	reason string
	label  string
}{
	{reason: "price", label: "가격"},
	{reason: "competitor", label: "경쟁사 선정"},
	{reason: "timing", label: "시기 부적합"},
	{reason: "no_budget", label: "예산 부재"},
	{reason: "no_fit", label: "요건 불일치"},
	{reason: "no_response", label: "응답 없음"},
	{reason: "internal", label: "내부 사정"},
}

func ensureCRMSchema(ctx context.Context, database *sql.DB) error {
	if errorValue := migrateCRMOpportunityAwayFromLostReasonForeignKey(ctx, database); errorValue != nil {
		return errorValue
	}
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return fmt.Errorf("begin CRM schema transaction: %w", errorValue)
	}
	for _, statement := range crmSchemaStatements {
		if _, errorValue := transaction.ExecContext(ctx, statement); errorValue != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("apply CRM schema: %w", errorValue)
		}
	}
	for _, seed := range crmPipelineSeeds {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO pipeline(pipeline, label, direction, is_active)
VALUES(?, ?, ?, 1)
ON CONFLICT(pipeline) DO NOTHING`, seed.pipeline, seed.label, seed.direction); errorValue != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("seed CRM pipeline %s: %w", seed.pipeline, errorValue)
		}
	}
	if errorValue := migrateCRMPipelineStagesToUniformSet(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	for _, seed := range crmPipelineStageSeeds {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO pipeline_stage(pipeline, stage, position, outcome)
VALUES(?, ?, ?, ?)
ON CONFLICT(pipeline, stage) DO NOTHING`, seed.pipeline, seed.stage, seed.position, seed.outcome); errorValue != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("seed CRM pipeline stage %s/%s: %w", seed.pipeline, seed.stage, errorValue)
		}
	}
	for _, seed := range crmLostReasonSeeds {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO lost_reason(reason, label, is_active)
VALUES(?, ?, 1)
ON CONFLICT(reason) DO NOTHING`, seed.reason, seed.label); errorValue != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("seed CRM lost reason %s: %w", seed.reason, errorValue)
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit CRM schema: %w", errorValue)
	}
	return nil
}

const crmUniformPipelineStagePositionOffset = 1000

func migrateCRMPipelineStagesToUniformSet(ctx context.Context, transaction *sql.Tx) error {
	quotedStages := make([]string, len(crmUniformPipelineStages))
	for index, stage := range crmUniformPipelineStages {
		quotedStages[index] = "'" + stage.stage + "'"
	}
	uniformStageList := strings.Join(quotedStages, ", ")

	var legacyStageCount int
	if errorValue := transaction.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pipeline_stage
WHERE stage NOT IN (`+uniformStageList+`)`).Scan(&legacyStageCount); errorValue != nil {
		return fmt.Errorf("count legacy CRM pipeline stages: %w", errorValue)
	}
	if legacyStageCount == 0 {
		return nil
	}

	pipelines, errorValue := crmDistinctPipelineIdentifiers(ctx, transaction)
	if errorValue != nil {
		return errorValue
	}
	for _, pipeline := range pipelines {
		for _, stage := range crmUniformPipelineStages {
			if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO pipeline_stage(pipeline, stage, position, outcome)
VALUES(?, ?, ?, ?)
ON CONFLICT(pipeline, stage) DO UPDATE SET position = excluded.position, outcome = excluded.outcome`,
				pipeline, stage.stage, stage.position+crmUniformPipelineStagePositionOffset, stage.outcome); errorValue != nil {
				return fmt.Errorf("seed CRM uniform pipeline stage %s/%s: %w", pipeline, stage.stage, errorValue)
			}
		}
	}
	if _, errorValue := transaction.ExecContext(ctx, `
UPDATE opportunity
SET stage = CASE (
		SELECT outcome FROM pipeline_stage
		WHERE pipeline_stage.pipeline = opportunity.pipeline AND pipeline_stage.stage = opportunity.stage
	)
		WHEN 'open' THEN 'in_progress'
		WHEN 'won' THEN 'done'
		WHEN 'lost' THEN 'lost'
		WHEN 'on_hold' THEN 'on_hold'
	END
WHERE stage NOT IN (`+uniformStageList+`)`); errorValue != nil {
		return fmt.Errorf("remap CRM opportunities off legacy pipeline stages: %w", errorValue)
	}
	if _, errorValue := transaction.ExecContext(ctx, `DELETE FROM pipeline_stage WHERE stage NOT IN (`+uniformStageList+`)`); errorValue != nil {
		return fmt.Errorf("delete legacy CRM pipeline stages: %w", errorValue)
	}
	if _, errorValue := transaction.ExecContext(ctx, `
UPDATE pipeline_stage
SET position = position - ?
WHERE position > ?`, crmUniformPipelineStagePositionOffset, crmUniformPipelineStagePositionOffset); errorValue != nil {
		return fmt.Errorf("finalize CRM uniform pipeline stage positions: %w", errorValue)
	}
	return nil
}

func crmDistinctPipelineIdentifiers(ctx context.Context, transaction *sql.Tx) ([]string, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT pipeline FROM pipeline
UNION
SELECT pipeline FROM pipeline_stage`)
	if errorValue != nil {
		return nil, fmt.Errorf("list CRM pipelines needing uniform stages: %w", errorValue)
	}
	defer rows.Close()
	pipelines := []string{}
	for rows.Next() {
		var pipeline string
		if errorValue := rows.Scan(&pipeline); errorValue != nil {
			return nil, errorValue
		}
		pipelines = append(pipelines, pipeline)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return pipelines, nil
}

func crmOpportunityHasLostReasonForeignKey(ctx context.Context, connection *sql.Conn) (bool, error) {
	var createStatement string
	errorValue := connection.QueryRowContext(ctx,
		"SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'opportunity'").Scan(&createStatement)
	if errorValue == sql.ErrNoRows {
		return false, nil
	}
	if errorValue != nil {
		return false, fmt.Errorf("read CRM opportunity table definition: %w", errorValue)
	}
	return strings.Contains(createStatement, "REFERENCES lost_reason(reason)"), nil
}

func rebuildCRMOpportunityKeepingLostReasonColumnFreeOfConstraint(ctx context.Context, connection *sql.Conn) error {
	transaction, errorValue := connection.BeginTx(ctx, nil)
	if errorValue != nil {
		return fmt.Errorf("begin CRM opportunity rebuild transaction: %w", errorValue)
	}
	dependentTriggers, errorValue := crmTriggerNamesReferencingOpportunity(ctx, transaction)
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	statements := make([]string, 0, len(dependentTriggers)+5)
	for _, triggerName := range dependentTriggers {
		statements = append(statements, "DROP TRIGGER "+triggerName)
	}
	statements = append(statements,
		"DROP TABLE IF EXISTS opportunity_rebuild",
		"CREATE TABLE opportunity_rebuild ("+crmOpportunityTableColumns+")",
		"INSERT INTO opportunity_rebuild SELECT * FROM opportunity",
		"DROP TABLE opportunity",
		"ALTER TABLE opportunity_rebuild RENAME TO opportunity",
	)
	for _, statement := range statements {
		if _, errorValue := transaction.ExecContext(ctx, statement); errorValue != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("rebuild CRM opportunity table: %w", errorValue)
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit CRM opportunity rebuild: %w", errorValue)
	}
	return nil
}

func crmTriggerNamesReferencingOpportunity(ctx context.Context, transaction *sql.Tx) ([]string, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT name FROM sqlite_master
WHERE type = 'trigger' AND sql LIKE '%opportunity%'
ORDER BY name`)
	if errorValue != nil {
		return nil, fmt.Errorf("list CRM triggers referencing opportunity: %w", errorValue)
	}
	defer rows.Close()
	triggerNames := []string{}
	for rows.Next() {
		var triggerName string
		if errorValue := rows.Scan(&triggerName); errorValue != nil {
			return nil, errorValue
		}
		triggerNames = append(triggerNames, triggerName)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return triggerNames, nil
}

func assertCRMOpportunityRebuildLeftNoForeignKeyViolations(ctx context.Context, connection *sql.Conn) error {
	rows, errorValue := connection.QueryContext(ctx, "PRAGMA foreign_key_check")
	if errorValue != nil {
		return fmt.Errorf("run CRM foreign key check after opportunity rebuild: %w", errorValue)
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("CRM opportunity rebuild left foreign key violations")
	}
	return rows.Err()
}

func migrateCRMOpportunityAwayFromLostReasonForeignKey(ctx context.Context, database *sql.DB) error {
	connection, errorValue := database.Conn(ctx)
	if errorValue != nil {
		return fmt.Errorf("acquire CRM opportunity rebuild connection: %w", errorValue)
	}
	defer connection.Close()
	hasForeignKey, errorValue := crmOpportunityHasLostReasonForeignKey(ctx, connection)
	if errorValue != nil {
		return errorValue
	}
	if !hasForeignKey {
		return nil
	}
	if _, errorValue := connection.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); errorValue != nil {
		return fmt.Errorf("disable CRM foreign keys for opportunity rebuild: %w", errorValue)
	}
	rebuildError := rebuildCRMOpportunityKeepingLostReasonColumnFreeOfConstraint(ctx, connection)
	if _, enableError := connection.ExecContext(ctx, "PRAGMA foreign_keys=ON"); enableError != nil {
		return fmt.Errorf("re-enable CRM foreign keys after opportunity rebuild: %w", enableError)
	}
	if rebuildError != nil {
		return rebuildError
	}
	return assertCRMOpportunityRebuildLeftNoForeignKeyViolations(ctx, connection)
}
