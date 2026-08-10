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

	test('formats each supported currency without applying exchange rates', () => {
		expect(formatMoney(12000000, 'KRW')).toBe('₩1,200만');
		expect(formatMoney(12000000, 'USD')).toBe('$1,200만');
		expect(formatMoney(12000000, 'JPY')).toBe('¥1,200만');
		expect(formatMoney(12000000, 'EUR')).toBe('€1,200만');
		expect(formatMoneyTotals({ KRW: 12000000, USD: 2500 })).toBe('₩1,200만 · $2,500');
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
