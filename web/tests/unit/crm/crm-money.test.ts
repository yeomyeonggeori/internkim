import { describe, expect, test } from 'bun:test';
import {
	interimCurrencyCatalogue,
	majorAmountOf,
	minorAmountOf,
	type CurrencyCatalogue
} from '../../../src/lib/currency/currency-catalogue';
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
		expect(formatMoney(12000000, 'KRW', interimCurrencyCatalogue)).toBe('₩1,200만');
		expect(formatMoney(12000000, 'JPY', interimCurrencyCatalogue)).toBe('¥1,200만');
		expect(formatMoney(250000000, 'KRW', interimCurrencyCatalogue)).toBe('₩2.5억');
		expect(formatMoney(8000, 'KRW', interimCurrencyCatalogue)).toBe('₩8,000');
	});

	test('compacts thousand-grouped currencies by K and M', () => {
		expect(formatMoney(12000000, 'USD', interimCurrencyCatalogue)).toBe('$12M');
		expect(formatMoney(12000000, 'EUR', interimCurrencyCatalogue)).toBe('€12M');
		expect(formatMoney(12000, 'USD', interimCurrencyCatalogue)).toBe('$12K');
		expect(formatMoney(2500, 'USD', interimCurrencyCatalogue)).toBe('$2.5K');
		expect(formatMoney(750, 'USD', interimCurrencyCatalogue)).toBe('$750');
	});

	test('renders a currency the catalogue does not know with plain separators and its code as the prefix', () => {
		expect(formatMoney(12000000, 'XAG', interimCurrencyCatalogue)).toBe('XAG12,000,000');
		expect(formatMoney(undefined, 'XAG', interimCurrencyCatalogue)).toBe('-');
	});

	test('keeps each currency in its own scale without applying exchange rates', () => {
		expect(formatMoneyTotals({ KRW: 12000000, USD: 2500 }, interimCurrencyCatalogue)).toBe('₩1,200만 · $2.5K');
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

describe('minor and major amounts follow the catalogue', () => {
	const catalogue: CurrencyCatalogue = [
		...interimCurrencyCatalogue,
		{ code: 'GBP', name: 'British Pound', symbol: '£', minorUnitDigits: 2, grouping: 'thousand' },
		{ code: 'CLP', name: 'Chilean Peso', symbol: '$', minorUnitDigits: 0, grouping: 'thousand' },
		{ code: 'BHD', name: 'Bahraini Dinar', symbol: 'BHD', minorUnitDigits: 3, grouping: 'thousand' }
	];

	test('a currency with no minor unit keeps the amount as it was entered', () => {
		expect(minorAmountOf(18000000, 'KRW', catalogue)).toBe(18000000);
		expect(majorAmountOf(18000000, 'KRW', catalogue)).toBe(18000000);
		expect(minorAmountOf(1200, 'CLP', catalogue)).toBe(1200);
	});

	test('a two-digit currency the seed never carried still scales by a hundred', () => {
		expect(minorAmountOf(1200, 'GBP', catalogue)).toBe(120000);
		expect(majorAmountOf(120000, 'GBP', catalogue)).toBe(1200);
	});

	test('a three-digit currency scales by a thousand', () => {
		expect(minorAmountOf(12, 'BHD', catalogue)).toBe(12000);
		expect(majorAmountOf(12000, 'BHD', catalogue)).toBe(12);
	});

	test('an unknown currency is assumed to have two minor digits', () => {
		expect(minorAmountOf(1200, 'ZZZ', catalogue)).toBe(120000);
	});
});
