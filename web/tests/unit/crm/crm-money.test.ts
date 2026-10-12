import { describe, expect, test } from 'bun:test';
import { interimCurrencyCatalogue } from '../../../src/lib/currency/currency-catalogue';
import {
	formatAmountInput,
	formatOptionalMoney,
	formatMoneyTotals,
	formatViewMoneyTotals,
	moneyFromAmountInput,
	parseAmountInput,
	sumOpportunityMoney,
	formatViewRateHint
} from '../../../src/routes/crm/crm-money';
import type { CRMOpportunity } from '../../../src/routes/crm/crm-types';
import type { CRMViewAmount, CRMViewCurrencyReader } from '../../../src/routes/crm/crm-view-currency.svelte';

describe('CRM money formatting', () => {
	test('formats and parses an integer amount with thousands separators', () => {
		expect(formatAmountInput('12000000')).toBe('12,000,000');
		expect(formatAmountInput('0012,000,000원')).toBe('12,000,000');
		expect(parseAmountInput('12,000,000')).toBe(12000000);
		expect(parseAmountInput('')).toBe(undefined);
	});

	test('reads a typed amount as money in the chosen currency', () => {
		expect(moneyFromAmountInput('12,000,000', 'KRW')).toEqual({ amount: 12000000, currency: 'KRW' });
		expect(moneyFromAmountInput('', 'KRW')).toBe(undefined);
	});

	test('compacts by 만 and 억 for a Korean reader, grouping the compact digits', () => {
		expect(formatOptionalMoney({ amount: 35000000, currency: 'KRW' })).toBe('₩3,500만');
		expect(formatOptionalMoney({ amount: 18000000, currency: 'KRW' })).toBe('₩1,800만');
		expect(formatOptionalMoney({ amount: 110000000, currency: 'KRW' })).toBe('₩1.1억');
		expect(formatOptionalMoney({ amount: 13500, currency: 'USD' })).toBe('$1.4만');
	});

	test('compacts by K and M for an English reader', () => {
		expect(formatOptionalMoney({ amount: 18000000, currency: 'USD' }, '-', 'en')).toBe('$18M');
		expect(formatOptionalMoney({ amount: 2500, currency: 'USD' }, '-', 'en')).toBe('$2.5K');
		expect(formatOptionalMoney({ amount: 750, currency: 'USD' }, '-', 'en')).toBe('$750');
	});

	test('lets the reader locale, not the currency, choose the compact unit', () => {
		expect(formatOptionalMoney({ amount: 250000000, currency: 'KRW' }, '-', 'ko')).toBe('₩2.5억');
		expect(formatOptionalMoney({ amount: 250000000, currency: 'KRW' }, '-', 'en')).toBe('₩250M');
	});

	test('falls back to the code for a currency with no symbol', () => {
		expect(formatOptionalMoney({ amount: 12000000, currency: 'XAG' })).toBe('XAG\u00a01,200만');
		expect(formatOptionalMoney(undefined)).toBe('-');
	});

	test('keeps each currency in its own scale without applying exchange rates', () => {
		expect(formatMoneyTotals({ KRW: 12000000, USD: 2500 }, interimCurrencyCatalogue)).toBe('₩1,200만 · $2.5천');
	});

	test('groups opportunity totals by their selected currency', () => {
		const opportunities: Array<Pick<CRMOpportunity, 'amount'>> = [
			{ amount: { amount: 1000000, currency: 'KRW' } },
			{ amount: { amount: 2500, currency: 'USD' } },
			{ amount: undefined },
			{ amount: { amount: 500000, currency: 'KRW' } }
		];

		expect(sumOpportunityMoney(opportunities)).toEqual({ KRW: 1500000, USD: 2500 });
	});
});

describe('view-currency-aware money formatting', () => {
	test('keeps the currency symbol on an amount already in the view currency', () => {
		const unconverted: CRMViewAmount = { amount: 18000000, currency: 'KRW', isConverted: false };
		const converted: CRMViewAmount = { amount: 13500, currency: 'USD', isConverted: true };

		expect(formatOptionalMoney(unconverted)).toBe('₩1,800만');
		expect(formatOptionalMoney(converted)).toBe('$1.4만');
	});

	test('collapses every currency total into one converted estimate when a view currency is active', () => {
		const view: CRMViewCurrencyReader = {
			selected: 'USD',
			viewAmount: (money) =>
				money.currency === 'USD'
					? { ...money, isConverted: true }
					: { amount: money.amount * 0.00075, currency: 'USD', isConverted: true }
		};

		expect(formatViewMoneyTotals({ KRW: 12000000, USD: 2500 }, interimCurrencyCatalogue, view)).toBe('$1.2만');
		expect(formatViewMoneyTotals({}, interimCurrencyCatalogue, view)).toBe('-');
	});

	test('falls back to the per-currency breakdown while any rate is still missing', () => {
		const view: CRMViewCurrencyReader = {
			selected: 'KRW',
			viewAmount: (money) => ({ ...money, isConverted: false })
		};

		expect(formatViewMoneyTotals({ KRW: 18000000, USD: 42000 }, interimCurrencyCatalogue, view, '-', 'ko')).toBe(
			formatMoneyTotals({ KRW: 18000000, USD: 42000 }, interimCurrencyCatalogue, '-', 'ko')
		);
	});

	test('keeps today\'s per-currency breakdown byte-for-byte until a view currency is chosen', () => {
		const view: CRMViewCurrencyReader = {
			selected: '',
			viewAmount: (money) => ({ ...money, isConverted: false })
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
