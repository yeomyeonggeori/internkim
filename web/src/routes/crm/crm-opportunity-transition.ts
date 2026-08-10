import type { CRMTransitionPayload } from './crm-api-types';
import type { CRMCurrency, CRMPipelineStage, CRMProgressKind } from './crm-types';

export type CRMOpportunityTransitionValues = {
	amountMinor: number | null;
	currencyCode: CRMCurrency | '';
	lostReason: string;
};

export type CRMOpportunityTransitionErrorCode = 'lost_reason_required';

export class CRMOpportunityTransitionError extends Error {
	constructor(readonly code: CRMOpportunityTransitionErrorCode) {
		super(code);
		this.name = 'CRMOpportunityTransitionError';
	}
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
	pipeline: CRMProgressKind,
	stage: string,
	values: CRMOpportunityTransitionValues
): Pick<CRMTransitionPayload, 'lostReason' | 'baseAmountMinor' | 'baseCurrencyCode'> {
	const outcome = stages.find((candidate) => candidate.pipeline === pipeline && candidate.stage === stage)?.outcome;
	if (outcome !== 'won' && outcome !== 'lost') {
		return { lostReason: '', baseAmountMinor: null, baseCurrencyCode: '' };
	}
	const lostReason = outcome === 'lost' ? values.lostReason.trim() : '';
	if (outcome === 'lost' && !lostReason) throw new CRMOpportunityTransitionError('lost_reason_required');
	return {
		lostReason,
		baseAmountMinor: values.amountMinor,
		baseCurrencyCode: values.amountMinor === null ? '' : values.currencyCode
	};
}
