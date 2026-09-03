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
			amountMinor: 10000,
			currencyCode: 'KRW',
			baseAmountMinor: null,
			baseCurrencyCode: '',
			lostReason: 'budget'
		}, { baseCurrency: 'KRW' })).toEqual({ lostReason: '', baseAmountMinor: null, baseCurrencyCode: '' });
	});

	test('leaves an amount already in the company currency to the record to settle', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amountMinor: 2500,
			currencyCode: 'KRW',
			baseAmountMinor: null,
			baseCurrencyCode: '',
			lostReason: ''
		}, { baseCurrency: 'KRW' })).toEqual({ lostReason: '', baseAmountMinor: null, baseCurrencyCode: '' });
	});

	test('preserves a converted base amount for foreign currency', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amountMinor: 2500,
			currencyCode: 'USD',
			baseAmountMinor: 3400000,
			baseCurrencyCode: 'KRW',
			lostReason: ''
		}, { baseCurrency: 'KRW' })).toEqual({ lostReason: '', baseAmountMinor: 3400000, baseCurrencyCode: 'KRW' });
	});

	test('leaves the base amount to the server when the server converts', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amountMinor: 2500,
			currencyCode: 'USD',
			baseAmountMinor: null,
			baseCurrencyCode: '',
			lostReason: ''
		}, { baseCurrency: 'KRW' })).toEqual({
			lostReason: '',
			baseAmountMinor: null,
			baseCurrencyCode: ''
		});
	});

	test('keeps a settled base amount even when the server converts', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amountMinor: 2500,
			currencyCode: 'USD',
			baseAmountMinor: 3400000,
			baseCurrencyCode: 'KRW',
			lostReason: ''
		}, { baseCurrency: 'KRW' })).toEqual({
			lostReason: '',
			baseAmountMinor: 3400000,
			baseCurrencyCode: 'KRW'
		});
	});

	test('requires an explicit lost reason', () => {
		try {
			opportunityTransitionOutcome(stages, 'lost', {
				amountMinor: null,
				currencyCode: '',
				baseAmountMinor: null,
				baseCurrencyCode: '',
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
