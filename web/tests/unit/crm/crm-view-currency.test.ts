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
	test('leaves amounts in their original currency while no view is chosen', () => {
		const view = new CRMViewCurrency(stubRateLoader({}, []));

		expect(view.selected).toBe('original');
		expect(view.viewAmount(18000000, 'KRW')).toEqual({ value: 18000000, currency: 'KRW', isConverted: false });
	});

	test('converts a value into the view currency and skips the currency already matching it', async () => {
		const calls: Array<[string, string]> = [];
		const view = new CRMViewCurrency(stubRateLoader({ KRW: 0.00075 }, calls));

		expect(await view.choose('USD', ['KRW', 'USD'])).toBe(true);
		expect(calls).toEqual([['KRW', 'USD']]);
		expect(view.viewAmount(18000000, 'KRW')).toEqual({ value: 13500, currency: 'USD', isConverted: true });
		expect(view.viewAmount(500, 'USD')).toEqual({ value: 500, currency: 'USD', isConverted: true });

		expect(await view.choose('original', [])).toBe(true);
		expect(view.selected).toBe('original');
		expect(view.viewAmount(18000000, 'KRW')).toEqual({ value: 18000000, currency: 'KRW', isConverted: false });
	});

	test('reverts to original and reports failure when any rate is missing, applying no partial conversion', async () => {
		const calls: Array<[string, string]> = [];
		const view = new CRMViewCurrency(stubRateLoader({ KRW: 0.00075 }, calls));

		expect(await view.choose('USD', ['KRW', 'EUR'])).toBe(false);
		expect(view.selected).toBe('original');
		expect(view.ratesBySource).toEqual({});
		expect(view.viewAmount(18000000, 'KRW')).toEqual({ value: 18000000, currency: 'KRW', isConverted: false });
	});
});
