import { describe, expect, test } from 'bun:test';
import {
	formatAmountInput,
	formatMoney,
	formatMoneyTotals,
	parseAmountInput,
	sumOpportunityMoney
} from '../../../src/routes/crm/crm-money';
import type { CRMOpportunity } from '../../../src/routes/crm/crm-types';

describe('CRM money formatting', () => {
	test('formats and parses an integer amount with thousands separators', () => {
		expect(formatAmountInput('12000000')).toBe('12,000,000');
		expect(formatAmountInput('0012,000,000원')).toBe('12,000,000');
		expect(parseAmountInput('12,000,000')).toBe(12000000);
		expect(parseAmountInput('')).toBe(undefined);
	});

	test('compacts myriad-grouped currencies by 만 and 억', () => {
		expect(formatMoney(12000000, 'KRW')).toBe('₩1,200만');
		expect(formatMoney(12000000, 'JPY')).toBe('¥1,200만');
		expect(formatMoney(250000000, 'KRW')).toBe('₩2.5억');
		expect(formatMoney(8000, 'KRW')).toBe('₩8,000');
	});

	test('compacts thousand-grouped currencies by K and M', () => {
		expect(formatMoney(12000000, 'USD')).toBe('$12M');
		expect(formatMoney(12000000, 'EUR')).toBe('€12M');
		expect(formatMoney(12000, 'USD')).toBe('$12K');
		expect(formatMoney(2500, 'USD')).toBe('$2.5K');
		expect(formatMoney(750, 'USD')).toBe('$750');
	});

	test('keeps each currency in its own scale without applying exchange rates', () => {
		expect(formatMoneyTotals({ KRW: 12000000, USD: 2500 })).toBe('₩1,200만 · $2.5K');
	});

	test('groups opportunity totals by their selected currency', () => {
		const opportunities: Array<Pick<CRMOpportunity, 'expectedValue' | 'currency'>> = [
			{ expectedValue: 1000000, currency: 'KRW' },
			{ expectedValue: 2500, currency: 'USD' },
			{ expectedValue: 500000, currency: 'KRW' }
		];

		expect(sumOpportunityMoney(opportunities)).toEqual({ KRW: 1500000, USD: 2500 });
	});
});
