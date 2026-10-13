import { describe, expect, test } from 'bun:test';
import {
	CRMOpportunityTransitionError,
	opportunityAvailableTransitionStages,
	opportunityTransitionOutcome
} from '../../../src/routes/crm/crm-opportunity-transition';
import type { CRMPipelineStage } from '../../../src/routes/crm/crm-types';


const stages: CRMPipelineStage[] = [
	{ stage: 'waiting', label: 'waiting', position: 1, outcome: 'open' },
	{ stage: 'done', label: 'done', position: 2, outcome: 'won' },
	{ stage: 'lost', label: 'lost', position: 3, outcome: 'lost' }
];

describe('CRM opportunity transition outcome', () => {
	test('clears realization fields for an open stage', () => {
		expect(opportunityTransitionOutcome(stages, 'waiting', {
			amount: { amount: 10000, currency: 'KRW' },
			settledAmount: null,
			lostReason: 'budget'
		}, { baseCurrency: 'KRW' })).toEqual({ lostReason: '', finalAmount: null });
	});

	test('leaves an amount already in the company currency to the record to settle', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amount: { amount: 2500, currency: 'KRW' },
			settledAmount: null,
			lostReason: ''
		}, { baseCurrency: 'KRW' })).toEqual({ lostReason: '', finalAmount: null });
	});

	test('preserves a converted base amount for foreign currency', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amount: { amount: 25, currency: 'USD' },
			settledAmount: { amount: 3400000, currency: 'KRW' },
			lostReason: ''
		}, { baseCurrency: 'KRW' })).toEqual({ lostReason: '', finalAmount: { amount: 3400000, currency: 'KRW' } });
	});

	test('leaves the base amount to the server when the server converts', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amount: { amount: 25, currency: 'USD' },
			settledAmount: null,
			lostReason: ''
		}, { baseCurrency: 'KRW' })).toEqual({
			lostReason: '',
			finalAmount: null
		});
	});

	test('keeps a settled base amount even when the server converts', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amount: { amount: 25, currency: 'USD' },
			settledAmount: { amount: 3400000, currency: 'KRW' },
			lostReason: ''
		}, { baseCurrency: 'KRW' })).toEqual({
			lostReason: '',
			finalAmount: { amount: 3400000, currency: 'KRW' }
		});
	});

	test('requires an explicit lost reason', () => {
		try {
			opportunityTransitionOutcome(stages, 'lost', {
				amount: null,
				settledAmount: null,
				lostReason: ''
			}, { baseCurrency: 'KRW' });
			throw new Error('expected lost transition to reject');
		} catch (error) {
			expect(error).toBeInstanceOf(CRMOpportunityTransitionError);
			expect((error as CRMOpportunityTransitionError).code).toBe('lost_reason_required');
		}
	});

	test('keeps realized opportunities within terminal stages', () => {
		expect(opportunityAvailableTransitionStages(stages, 'done').map((stage) => stage.stage)).toEqual(['done', 'lost']);
		expect(opportunityAvailableTransitionStages(stages, 'waiting').map((stage) => stage.stage)).toEqual(['waiting', 'done', 'lost']);
	});
});
