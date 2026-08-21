import { describe, expect, test } from 'bun:test';
import type { ConvertedAmount } from '../../../src/lib/currency/converted-amount';

Object.assign(globalThis, {
	$state<Value>(value: Value): Value {
		return value;
	}
});

const { CRMViewCurrency } = await import('../../../src/routes/crm/crm-view-currency.svelte');

function stubRateLoader(rates: Record<string, number>, calls: Array<[string, string]>) {
	return async (amountMinor: number, from: string, to: string): Promise<ConvertedAmount | null> => {
		calls.push([from, to]);
		const rate = rates[from];
		if (rate === undefined) return null;
		return { amountMinor: Math.round(amountMinor * rate), currencyCode: to, rate, asOf: '2026-08-20' };
	};
}

describe('CRM view currency state', () => {
	test('leaves amounts untouched until a view currency arrives', () => {
		const view = new CRMViewCurrency(stubRateLoader({}, []));

		expect(view.selected).toBe('');
		expect(view.viewAmount(18000000, 'KRW')).toEqual({ value: 18000000, currency: 'KRW', isConverted: false });
	});

	test('keeps following the company base currency until a manual choice lands', async () => {
		const calls: Array<[string, string]> = [];
		const view = new CRMViewCurrency(stubRateLoader({ KRW: 0.00075, USD: 1350 }, calls));

		await view.follow('', ['KRW']);
		expect(view.selected).toBe('');

		await view.follow('KRW', ['KRW']);
		expect(view.selected).toBe('KRW');

		await view.follow('USD', ['KRW', 'USD']);
		expect(view.selected).toBe('USD');

		expect(await view.choose('KRW', ['KRW', 'USD'])).toBe(true);
		await view.follow('USD', ['KRW', 'USD']);
		expect(view.selected).toBe('KRW');
	});

	test('loads the rates a newly seen source currency needs, keeping the view', async () => {
		const calls: Array<[string, string]> = [];
		const view = new CRMViewCurrency(stubRateLoader({ USD: 1350, JPY: 9.2 }, calls));

		await view.follow('KRW', ['KRW']);
		expect(view.selected).toBe('KRW');
		expect(view.ratesBySource).toEqual({});

		await view.follow('KRW', ['KRW', 'USD']);
		expect(view.selected).toBe('KRW');
		expect(view.viewAmount(42000, 'USD')).toEqual({ value: 42000 * 1350, currency: 'KRW', isConverted: true });

		expect(await view.choose('KRW', ['KRW', 'USD'])).toBe(true);
		await view.follow('USD', ['KRW', 'USD', 'JPY']);
		expect(view.selected).toBe('KRW');
		expect(view.viewAmount(800000, 'JPY')).toEqual({ value: 800000 * 9.2, currency: 'KRW', isConverted: true });
	});

	test('converts into the view currency and leaves matching amounts exact and unmarked', async () => {
		const calls: Array<[string, string]> = [];
		const view = new CRMViewCurrency(stubRateLoader({ KRW: 0.00075 }, calls));

		expect(await view.choose('USD', ['KRW', 'USD'])).toBe(true);
		expect(calls).toEqual([['KRW', 'USD']]);
		expect(view.viewAmount(18000000, 'KRW')).toEqual({ value: 13500, currency: 'USD', isConverted: true });
		expect(view.viewAmount(500, 'USD')).toEqual({ value: 500, currency: 'USD', isConverted: false });
	});

	test('keeps the previous view and reports failure when any rate is missing, applying no partial conversion', async () => {
		const calls: Array<[string, string]> = [];
		const view = new CRMViewCurrency(stubRateLoader({ KRW: 0.00075 }, calls));

		expect(await view.choose('USD', ['KRW', 'EUR'])).toBe(false);
		expect(view.selected).toBe('');
		expect(view.ratesBySource).toEqual({});
		expect(view.viewAmount(18000000, 'KRW')).toEqual({ value: 18000000, currency: 'KRW', isConverted: false });
	});
});
