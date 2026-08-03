package admind

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
)

func (service *Service) transitionCRMOpportunityStage(ctx context.Context, transition crmOpportunityStageTransition) error {
	transition.OpportunityID = strings.TrimSpace(transition.OpportunityID)
	transition.Stage = strings.TrimSpace(transition.Stage)
	transition.BeforeOpportunityID = strings.TrimSpace(transition.BeforeOpportunityID)
	transition.ActorPersonID = strings.TrimSpace(transition.ActorPersonID)
	if transition.OpportunityID == "" || transition.Stage == "" || transition.ActorPersonID == "" {
		return fmt.Errorf("CRM stage transition opportunity, stage, and actor are required")
	}
	if errorValue := crmValidateTimestamp(transition.OccurredAt); errorValue != nil {
		return fmt.Errorf("invalid CRM stage transition time: %w", errorValue)
	}
	if math.IsNaN(transition.StagePosition) || math.IsInf(transition.StagePosition, 0) {
		return fmt.Errorf("invalid CRM stage position")
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	opportunity, errorValue := scanCRMOpportunity(transaction.QueryRowContext(ctx, crmOpportunitySelectSQL+" WHERE id = ?", transition.OpportunityID))
	if errorValue == sql.ErrNoRows {
		_ = transaction.Rollback()
		return errCRMRecordNotFound
	}
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if opportunity.Stage == transition.Stage {
		_ = transaction.Rollback()
		return newCRMConflict(fmt.Sprintf("CRM opportunity is already in stage %s", transition.Stage))
	}
	if transition.StagePosition == 0 {
		transition.StagePosition = 1024
	}
	result, errorValue := transaction.ExecContext(ctx, `
UPDATE opportunity
SET stage = ?, stage_position = ?, stage_changed_at = ?, lost_reason = ?,
	base_amount_minor = ?, base_currency_code = ?, updated_at = ?, updated_by_person_id = ?
WHERE id = ?`, transition.Stage, transition.StagePosition, transition.OccurredAt,
		crmNullableString(transition.LostReason), crmNullableInt64(transition.BaseAmountMinor),
		crmNullableString(transition.BaseCurrencyCode), transition.OccurredAt, transition.ActorPersonID,
		transition.OpportunityID)
	if errorValue != nil {
		_ = transaction.Rollback()
		return fmt.Errorf("transition CRM opportunity %s: %w", transition.OpportunityID, errorValue)
	}
	if errorValue := crmRequireAffectedRecord(result); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := rebalanceCRMStagePositionsOnCollision(ctx, transaction, opportunity.Pipeline, transition.Stage, transition.OpportunityID, transition.StagePosition, transition.BeforeOpportunityID, transition.OccurredAt, transition.ActorPersonID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	activity := crmActivity{
		ID:            newCRMID("activity"),
		AccountID:     opportunity.AccountID,
		OpportunityID: opportunity.ID,
		Business:      opportunity.Business,
		Kind:          "stage_change",
		Title:         opportunity.Stage + " → " + transition.Stage,
		OccurredAt:    transition.OccurredAt,
		Audit: crmAuditFields{
			CreatedAt:         transition.OccurredAt,
			CreatedByPersonID: transition.ActorPersonID,
			UpdatedAt:         transition.OccurredAt,
			UpdatedByPersonID: transition.ActorPersonID,
		},
	}
	if errorValue := insertCRMActivityInTransaction(ctx, transaction, activity); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit CRM stage transition %s: %w", transition.OpportunityID, errorValue)
	}
	return nil
}

func (service *Service) setCRMOpportunityStagePosition(ctx context.Context, opportunityID string, position float64, beforeOpportunityID string, updatedAt string, actorPersonID string) error {
	if errorValue := crmValidateTimestamp(updatedAt); errorValue != nil {
		return fmt.Errorf("invalid CRM position update time: %w", errorValue)
	}
	if math.IsNaN(position) || math.IsInf(position, 0) {
		return fmt.Errorf("invalid CRM stage position")
	}
	opportunityID = strings.TrimSpace(opportunityID)
	beforeOpportunityID = strings.TrimSpace(beforeOpportunityID)
	actorPersonID = strings.TrimSpace(actorPersonID)
	if opportunityID == "" || actorPersonID == "" {
		return fmt.Errorf("CRM position opportunity and actor are required")
	}
	if opportunityID == beforeOpportunityID {
		return newCRMConflict("CRM position anchor must differ from the moved opportunity")
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	var pipeline string
	var stage string
	if errorValue := transaction.QueryRowContext(ctx, "SELECT pipeline, stage FROM opportunity WHERE id = ?", opportunityID).Scan(&pipeline, &stage); errorValue != nil {
		_ = transaction.Rollback()
		if errorValue == sql.ErrNoRows {
			return errCRMRecordNotFound
		}
		return errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, `
UPDATE opportunity
SET stage_position = ?, updated_at = ?, updated_by_person_id = ?
WHERE id = ?`, position, updatedAt, actorPersonID, opportunityID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := rebalanceCRMStagePositionsOnCollision(ctx, transaction, pipeline, stage, opportunityID, position, beforeOpportunityID, updatedAt, actorPersonID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	return nil
}

func rebalanceCRMStagePositionsOnCollision(ctx context.Context, transaction *sql.Tx, pipeline string, stage string, movedOpportunityID string, position float64, beforeOpportunityID string, updatedAt string, actorPersonID string) error {
	var collisionCount int
	if errorValue := transaction.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM opportunity
WHERE pipeline = ? AND stage = ? AND stage_position = ?`, pipeline, stage, position).Scan(&collisionCount); errorValue != nil {
		return errorValue
	}
	if collisionCount <= 1 && beforeOpportunityID == "" {
		return nil
	}
	return rebalanceCRMStagePositions(ctx, transaction, pipeline, stage, movedOpportunityID, position, beforeOpportunityID, updatedAt, actorPersonID)
}

func rebalanceCRMStagePositions(ctx context.Context, transaction *sql.Tx, pipeline string, stage string, movedOpportunityID string, position float64, beforeOpportunityID string, updatedAt string, actorPersonID string) error {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, stage_position
FROM opportunity
WHERE pipeline = ? AND stage = ? AND id <> ?
ORDER BY stage_position, id`, pipeline, stage, movedOpportunityID)
	if errorValue != nil {
		return errorValue
	}
	type positionedOpportunity struct {
		id       string
		position float64
	}
	positioned := []positionedOpportunity{}
	for rows.Next() {
		var opportunity positionedOpportunity
		if errorValue := rows.Scan(&opportunity.id, &opportunity.position); errorValue != nil {
			rows.Close()
			return errorValue
		}
		positioned = append(positioned, opportunity)
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	ids := make([]string, 0, len(positioned)+1)
	inserted := false
	for _, opportunity := range positioned {
		if !inserted && ((beforeOpportunityID != "" && opportunity.id == beforeOpportunityID) || (beforeOpportunityID == "" && opportunity.position >= position)) {
			ids = append(ids, movedOpportunityID)
			inserted = true
		}
		ids = append(ids, opportunity.id)
	}
	if beforeOpportunityID != "" && !inserted {
		return newCRMConflict(fmt.Sprintf("CRM position anchor %s is not in pipeline %s stage %s", beforeOpportunityID, pipeline, stage))
	}
	if !inserted {
		ids = append(ids, movedOpportunityID)
	}
	for index, id := range ids {
		if _, errorValue := transaction.ExecContext(ctx, `
UPDATE opportunity
SET stage_position = ?, updated_at = ?, updated_by_person_id = ?
WHERE id = ?`, float64(index+1)*1024, updatedAt, actorPersonID, id); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
