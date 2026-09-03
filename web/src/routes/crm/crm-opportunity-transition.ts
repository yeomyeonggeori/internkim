import type { CRMTransitionPayload } from './crm-api-types';
import type { CRMCurrency, CRMPipelineStage } from './crm-types';

export type CRMOpportunityTransitionValues = {
	amountMinor: number | null;
	currencyCode: CRMCurrency | '';
	baseAmountMinor: number | null;
	baseCurrencyCode: CRMCurrency | '';
	lostReason: string;
};

export type CRMOpportunitySettlement = {
	baseCurrency: CRMCurrency;
};

export type CRMOpportunityTransitionErrorCode = 'lost_reason_required';

export class CRMOpportunityTransitionError extends Error {
	constructor(readonly code: CRMOpportunityTransitionErrorCode) {
		super(code);
		this.name = 'CRMOpportunityTransitionError';
	}
}

export function opportunityStageOutcome(
	stages: CRMPipelineStage[],
	stage: string
): CRMPipelineStage['outcome'] | undefined {
	return stages.find((candidate) => candidate.stage === stage)?.outcome;
}

export function opportunitySettlesOnTransition(
	stages: CRMPipelineStage[],
	stage: string
): boolean {
	const outcome = opportunityStageOutcome(stages, stage);
	return outcome === 'won' || outcome === 'lost';
}

export function opportunityAvailableTransitionStages(
	stages: CRMPipelineStage[],
	currentStage: string
): CRMPipelineStage[] {
	const currentOutcome = stages.find((candidate) => candidate.stage === currentStage)?.outcome;
	if (currentOutcome !== 'won' && currentOutcome !== 'lost') return stages;
	return stages.filter((candidate) => candidate.outcome === 'won' || candidate.outcome === 'lost');
}

export function opportunityTransitionOutcome(
	stages: CRMPipelineStage[],
	stage: string,
	values: CRMOpportunityTransitionValues,
	settlement: CRMOpportunitySettlement
): Pick<CRMTransitionPayload, 'lostReason' | 'baseAmountMinor' | 'baseCurrencyCode'> {
	const outcome = opportunityStageOutcome(stages, stage);
	if (outcome !== 'won' && outcome !== 'lost') {
		return { lostReason: '', baseAmountMinor: null, baseCurrencyCode: '' };
	}
	const lostReason = outcome === 'lost' ? values.lostReason.trim() : '';
	if (outcome === 'lost' && !lostReason) throw new CRMOpportunityTransitionError('lost_reason_required');
	if (values.amountMinor === null) return { lostReason, baseAmountMinor: null, baseCurrencyCode: '' };
	if (values.baseAmountMinor !== null && values.baseCurrencyCode !== '') {
		return { lostReason, baseAmountMinor: values.baseAmountMinor, baseCurrencyCode: values.baseCurrencyCode };
	}
	return { lostReason, baseAmountMinor: null, baseCurrencyCode: '' };
}
