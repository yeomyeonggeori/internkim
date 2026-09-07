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
	formatViewMoney,
	formatViewMoneyTotals,
	parseAmountInput,
	sumOpportunityMoney,
	formatViewRateHint
} from '../../../src/routes/crm/crm-money';
import type { CRMOpportunity } from '../../../src/routes/crm/crm-types';
import type { CRMViewCurrencyReader } from '../../../src/routes/crm/crm-view-currency.svelte';

describe('CRM money formatting', () => {
	test('formats and parses an integer amount with thousands separators', () => {
		expect(formatAmountInput('12000000')).toBe('12,000,000');
		expect(formatAmountInput('0012,000,000원')).toBe('12,000,000');
		expect(parseAmountInput('12,000,000')).toBe(12000000);
		expect(parseAmountInput('')).toBe(undefined);
	});

	test('compacts by 만 and 억 for a Korean reader, grouping the compact digits', () => {
		expect(formatMoney(35000000, 'KRW')).toBe('₩3,500만');
		expect(formatMoney(18000000, 'KRW')).toBe('₩1,800만');
		expect(formatMoney(110000000, 'KRW')).toBe('₩1.1억');
		expect(formatMoney(13500, 'USD')).toBe('$1.4만');
	});

	test('compacts by K and M for an English reader', () => {
		expect(formatMoney(18000000, 'USD', '-', 'en')).toBe('$18M');
		expect(formatMoney(2500, 'USD', '-', 'en')).toBe('$2.5K');
		expect(formatMoney(750, 'USD', '-', 'en')).toBe('$750');
	});

	test('lets the reader locale, not the currency, choose the compact unit', () => {
		expect(formatMoney(250000000, 'KRW', '-', 'ko')).toBe('₩2.5억');
		expect(formatMoney(250000000, 'KRW', '-', 'en')).toBe('₩250M');
	});

	test('falls back to the code for a currency with no symbol', () => {
		expect(formatMoney(12000000, 'XAG')).toBe('XAG\u00a01,200만');
		expect(formatMoney(undefined, 'XAG')).toBe('-');
	});

	test('keeps each currency in its own scale without applying exchange rates', () => {
		expect(formatMoneyTotals({ KRW: 12000000, USD: 2500 }, interimCurrencyCatalogue)).toBe('₩1,200만 · $2.5천');
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

describe('view-currency-aware money formatting', () => {
	test('keeps the currency symbol on an amount already in the view currency', () => {
		expect(formatViewMoney({ value: 18000000, currency: 'KRW', isConverted: false })).toBe('₩1,800만');
		expect(formatViewMoney({ value: 13500, currency: 'USD', isConverted: true })).toBe('$1.4만');
	});

	test('collapses every currency total into one converted estimate when a view currency is active', () => {
		const view: CRMViewCurrencyReader = {
			selected: 'USD',
			viewAmount: (value, currency) =>
				currency === 'USD'
					? { value, currency: 'USD', isConverted: true }
					: { value: value * 0.00075, currency: 'USD', isConverted: true }
		};

		expect(formatViewMoneyTotals({ KRW: 12000000, USD: 2500 }, interimCurrencyCatalogue, view)).toBe('$1.2만');
		expect(formatViewMoneyTotals({}, interimCurrencyCatalogue, view)).toBe('-');
	});

	test('falls back to the per-currency breakdown while any rate is still missing', () => {
		const view: CRMViewCurrencyReader = {
			selected: 'KRW',
			viewAmount: (value, currency) =>
				currency === 'KRW'
					? { value, currency: 'KRW', isConverted: false }
					: { value, currency, isConverted: false }
		};

		expect(formatViewMoneyTotals({ KRW: 18000000, USD: 42000 }, interimCurrencyCatalogue, view, '-', 'ko')).toBe(
			formatMoneyTotals({ KRW: 18000000, USD: 42000 }, interimCurrencyCatalogue, '-', 'ko')
		);
	});

	test('keeps today\'s per-currency breakdown byte-for-byte until a view currency is chosen', () => {
		const view: CRMViewCurrencyReader = {
			selected: '',
			viewAmount: (value, currency) => ({ value, currency, isConverted: false })
		};
		const totals = { KRW: 12000000, USD: 2500 };

		expect(formatViewMoneyTotals(totals, interimCurrencyCatalogue, view, '-', 'ko')).toBe(
			formatMoneyTotals(totals, interimCurrencyCatalogue, '-', 'ko')
		);
	});
});

describe('formatViewRateHint always reads one dollar in the chosen currency', () => {
	test('names what a dollar buys, rounding whole units and keeping small rates precise', () => {
		expect(formatViewRateHint('KRW', { USD: 1396.35 })).toBe('USD 1 = KRW 1,396');
		expect(formatViewRateHint('EUR', { USD: 0.857 })).toBe('USD 1 = EUR 0.86');
	});

	test('ignores every source currency that is not the dollar', () => {
		expect(formatViewRateHint('KRW', { USD: 1396.35, JPY: 9.19, EUR: 0.86 })).toBe('USD 1 = KRW 1,396');
	});

	test('stays silent for the dollar itself, for no view, and before the rate loads', () => {
		expect(formatViewRateHint('USD', { KRW: 0.00072 })).toBe('');
		expect(formatViewRateHint('', { USD: 1396.35 })).toBe('');
		expect(formatViewRateHint('KRW', {})).toBe('');
	});
});

describe('minor and major amounts follow the catalogue', () => {
	const catalogue: CurrencyCatalogue = [
		...interimCurrencyCatalogue,
		{ code: 'GBP', name: 'British Pound', minorUnitDigits: 2 },
		{ code: 'CLP', name: 'Chilean Peso', minorUnitDigits: 0 },
		{ code: 'BHD', name: 'Bahraini Dinar', minorUnitDigits: 3 }
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
