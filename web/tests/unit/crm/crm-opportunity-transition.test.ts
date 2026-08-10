import { describe, expect, test } from 'bun:test';
import {
	CRMOpportunityTransitionError,
	opportunityAvailableTransitionStages,
	opportunityTransitionOutcome
} from '../../../src/routes/crm/crm-opportunity-transition';
import type { CRMPipelineStage } from '../../../src/routes/crm/crm-types';

const stages: CRMPipelineStage[] = [
	{ pipeline: 'sales', stage: 'lead', position: 1, outcome: 'open' },
	{ pipeline: 'sales', stage: 'won', position: 2, outcome: 'won' },
	{ pipeline: 'sales', stage: 'lost', position: 3, outcome: 'lost' }
];

describe('CRM opportunity transition outcome', () => {
	test('clears realization fields for an open stage', () => {
		expect(opportunityTransitionOutcome(stages, 'sales', 'lead', {
			amountMinor: 10000,
			currencyCode: 'KRW',
			lostReason: 'budget'
		})).toEqual({ lostReason: '', baseAmountMinor: null, baseCurrencyCode: '' });
	});

	test('uses the recorded amount as the realized amount for a won stage', () => {
		expect(opportunityTransitionOutcome(stages, 'sales', 'won', {
			amountMinor: 2500,
			currencyCode: 'USD',
			lostReason: ''
		})).toEqual({ lostReason: '', baseAmountMinor: 2500, baseCurrencyCode: 'USD' });
	});

	test('requires an explicit lost reason', () => {
		try {
			opportunityTransitionOutcome(stages, 'sales', 'lost', {
				amountMinor: null,
				currencyCode: '',
				lostReason: ''
			});
			throw new Error('expected lost transition to reject');
		} catch (error) {
			expect(error).toBeInstanceOf(CRMOpportunityTransitionError);
			expect((error as CRMOpportunityTransitionError).code).toBe('lost_reason_required');
		}
	});

	test('keeps realized opportunities within terminal stages', () => {
		expect(opportunityAvailableTransitionStages(stages, 'won').map((stage) => stage.stage)).toEqual(['won', 'lost']);
		expect(opportunityAvailableTransitionStages(stages, 'lead').map((stage) => stage.stage)).toEqual(['lead', 'won', 'lost']);
	});
});
