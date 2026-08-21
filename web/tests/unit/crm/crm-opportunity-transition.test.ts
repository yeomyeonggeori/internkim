import { describe, expect, test } from 'bun:test';
import {
	CRMOpportunityTransitionError,
	opportunityAvailableTransitionStages,
	opportunityTransitionOutcome
} from '../../../src/routes/crm/crm-opportunity-transition';
import type { CRMPipelineStage } from '../../../src/routes/crm/crm-types';

function onTheDevice(baseCurrency: string) {
	return { baseCurrency, isConvertedByServer: false };
}

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
		}, onTheDevice('KRW'))).toEqual({ lostReason: '', baseAmountMinor: null, baseCurrencyCode: '' });
	});

	test('uses the recorded amount when it already uses the company base currency', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amountMinor: 2500,
			currencyCode: 'KRW',
			baseAmountMinor: null,
			baseCurrencyCode: '',
			lostReason: ''
		}, onTheDevice('KRW'))).toEqual({ lostReason: '', baseAmountMinor: 2500, baseCurrencyCode: 'KRW' });
	});

	test('preserves a converted base amount for foreign currency', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amountMinor: 2500,
			currencyCode: 'USD',
			baseAmountMinor: 3400000,
			baseCurrencyCode: 'KRW',
			lostReason: ''
		}, onTheDevice('KRW'))).toEqual({ lostReason: '', baseAmountMinor: 3400000, baseCurrencyCode: 'KRW' });
	});

	test('rejects foreign currency without a converted base amount', () => {
		expect(() => opportunityTransitionOutcome(stages, 'done', {
			amountMinor: 2500,
			currencyCode: 'USD',
			baseAmountMinor: null,
			baseCurrencyCode: '',
			lostReason: ''
		}, onTheDevice('KRW'))).toThrow(new CRMOpportunityTransitionError('base_currency_conversion_required'));
	});

	test('settles a dollar amount without conversion for a dollar company', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amountMinor: 2500,
			currencyCode: 'USD',
			baseAmountMinor: null,
			baseCurrencyCode: '',
			lostReason: ''
		}, onTheDevice('USD'))).toEqual({ lostReason: '', baseAmountMinor: 2500, baseCurrencyCode: 'USD' });
	});

	test('asks a dollar company to convert a won amount', () => {
		expect(() => opportunityTransitionOutcome(stages, 'done', {
			amountMinor: 2500,
			currencyCode: 'KRW',
			baseAmountMinor: null,
			baseCurrencyCode: '',
			lostReason: ''
		}, onTheDevice('USD'))).toThrow(new CRMOpportunityTransitionError('base_currency_conversion_required'));
	});

	test('leaves the base amount to the server when the server converts', () => {
		expect(opportunityTransitionOutcome(stages, 'done', {
			amountMinor: 2500,
			currencyCode: 'USD',
			baseAmountMinor: null,
			baseCurrencyCode: '',
			lostReason: ''
		}, { baseCurrency: 'KRW', isConvertedByServer: true })).toEqual({
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
		}, { baseCurrency: 'KRW', isConvertedByServer: true })).toEqual({
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
			}, onTheDevice('KRW'));
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
