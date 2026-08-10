import type { CRMOpportunity, CRMOpportunityStage } from './crm-types';

export type CRMPipelineBoardMoveRequest = {
	opportunityID: string;
	targetStage: CRMOpportunityStage;
	beforeOpportunityID: string | null;
};

export function moveCRMOpportunityOnBoard(
	opportunities: CRMOpportunity[],
	request: CRMPipelineBoardMoveRequest,
	stageOrder: CRMOpportunityStage[] = orderedStages(opportunities)
): CRMOpportunity[] | null {
	if (request.beforeOpportunityID === request.opportunityID) return null;
	const movedOpportunity = opportunities.find((opportunity) => opportunity.id === request.opportunityID);
	if (!movedOpportunity) return null;

	const remainingOpportunities = opportunities.filter((opportunity) => opportunity.id !== request.opportunityID);
	const insertIndex = targetInsertIndex(remainingOpportunities, request.targetStage, request.beforeOpportunityID, stageOrder);
	const nextOpportunity = {
		...movedOpportunity,
		stage: request.targetStage,
		staleDays: movedOpportunity.stage === request.targetStage ? movedOpportunity.staleDays : 0
	};
	const nextOpportunities = [
		...remainingOpportunities.slice(0, insertIndex),
		nextOpportunity,
		...remainingOpportunities.slice(insertIndex)
	];
	if (sameOpportunityState(opportunities, nextOpportunities)) return null;
	return nextOpportunities;
}

function targetInsertIndex(
	opportunities: CRMOpportunity[],
	targetStage: CRMOpportunityStage,
	beforeOpportunityID: string | null,
	stageOrder: CRMOpportunityStage[]
): number {
	if (beforeOpportunityID) {
		const beforeIndex = opportunities.findIndex(
			(opportunity) => opportunity.id === beforeOpportunityID && opportunity.stage === targetStage
		);
		if (beforeIndex >= 0) return beforeIndex;
	}

	const lastTargetIndex = lastOpportunityIndexForStage(opportunities, targetStage);
	if (lastTargetIndex >= 0) return lastTargetIndex + 1;

	const targetStageIndex = stageOrder.indexOf(targetStage);
	const followingStageIndex = opportunities.findIndex(
		(opportunity) => stageOrder.indexOf(opportunity.stage) > targetStageIndex
	);
	return followingStageIndex >= 0 ? followingStageIndex : opportunities.length;
}

function orderedStages(opportunities: CRMOpportunity[]): CRMOpportunityStage[] {
	return [...new Set(opportunities.map((opportunity) => opportunity.stage))];
}

function lastOpportunityIndexForStage(
	opportunities: CRMOpportunity[],
	targetStage: CRMOpportunityStage
): number {
	for (let index = opportunities.length - 1; index >= 0; index -= 1) {
		if (opportunities[index]?.stage === targetStage) return index;
	}
	return -1;
}

function sameOpportunityState(left: CRMOpportunity[], right: CRMOpportunity[]): boolean {
	if (left.length !== right.length) return false;
	return left.every((opportunity, index) => {
		const nextOpportunity = right[index];
		return opportunity.id === nextOpportunity?.id
			&& opportunity.stage === nextOpportunity.stage
			&& opportunity.staleDays === nextOpportunity.staleDays;
	});
}
