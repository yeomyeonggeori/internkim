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

function deferredRate() {
	let resolve: (value: ConvertedAmount | null) => void = () => {};
	let reject: (reason: Error) => void = () => {};
	const promise = new Promise<ConvertedAmount | null>((success, failure) => { resolve = success; reject = failure; });
	return { promise, resolve, reject };
}

function quote(currencyCode: string, rate: number): ConvertedAmount {
	return { amountMinor: Math.round(1000000 * rate), currencyCode, rate, asOf: '2026-08-20' };
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
		expect(view.ratesBySource).toEqual({ USD: 1350 });

		await view.follow('KRW', ['KRW', 'USD']);
		expect(view.selected).toBe('KRW');
		expect(view.viewAmount(42000, 'USD')).toEqual({ value: 42000 * 1350, currency: 'KRW', isConverted: true });

		expect(await view.choose('KRW', ['KRW', 'USD'])).toBe(true);
		await view.follow('USD', ['KRW', 'USD', 'JPY']);
		expect(view.selected).toBe('KRW');
		expect(view.viewAmount(800000, 'JPY')).toEqual({ value: 800000 * 9.2, currency: 'KRW', isConverted: true });
	});

	test('retries a failed follow only when its inputs change', async () => {
		const calls: Array<[string, string]> = [];
		const view = new CRMViewCurrency(stubRateLoader({ USD: 1350 }, calls));

		await view.follow('KRW', ['KRW', 'EUR']);
		await view.follow('KRW', ['KRW', 'EUR']);
		expect(view.selected).toBe('');
		expect(calls).toEqual([['EUR', 'KRW'], ['USD', 'KRW']]);

		await view.follow('KRW', ['KRW', 'USD']);
		expect(view.selected).toBe('KRW');
		expect(calls).toEqual([
			['EUR', 'KRW'],
			['USD', 'KRW'],
			['USD', 'KRW']
		]);

		await view.follow('KRW', ['KRW', 'USD']);
		expect(calls.length).toBe(3);
	});

	test('prices the dollar the rate hint reads even when no deal is priced in it', async () => {
		const calls: Array<[string, string]> = [];
		const view = new CRMViewCurrency(stubRateLoader({ KRW: 0.00075, USD: 1350 }, calls));

		await view.follow('USD', ['KRW']);
		expect(view.selected).toBe('USD');
		expect(calls).toEqual([['KRW', 'USD']]);

		expect(await view.choose('KRW', ['KRW'])).toBe(true);
		expect(view.ratesBySource['USD']).toBe(1350);
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

	test('keeps the committed currency and amounts together while a delayed choice loads', async () => {
		const delayed = deferredRate();
		const view = new CRMViewCurrency(async (_amount, _from, to) => to === 'KRW' ? quote('KRW', 1350) : delayed.promise);
		await view.follow('KRW', ['KRW']);
		const choosing = view.choose('USD', ['KRW']);
		expect(view.pending).toBe('USD');
		expect(view.isLoading).toBe(true);
		expect(view.selected).toBe('KRW');
		expect(view.viewAmount(18000000, 'KRW')).toEqual({ value: 18000000, currency: 'KRW', isConverted: false });
		delayed.resolve(quote('USD', 0.00075));
		expect(await choosing).toBe(true);
		expect(view.pending).toBe('');
		expect(view.isLoading).toBe(false);
		expect(view.selected).toBe('USD');
		expect(view.viewAmount(18000000, 'KRW')).toEqual({ value: 13500, currency: 'USD', isConverted: true });
	});

	test('contains a rejected rate, stops automatic retries, and recovers on an explicit retry', async () => {
		let rejectRequest = true;
		let calls = 0;
		const view = new CRMViewCurrency(async (_amount, _from, to) => {
			calls += 1;
			if (rejectRequest) throw new Error('provider timed out');
			return quote(to, 0.00075);
		});
		expect(await view.choose('USD', ['KRW'])).toBe(false);
		expect(view.failed).toBe('USD');
		expect(view.isLoading).toBe(false);
		expect(view.selected).toBe('');
		await view.follow('KRW', ['KRW']);
		expect(calls).toBe(1);
		rejectRequest = false;
		expect(await view.choose('USD', ['KRW'])).toBe(true);
		expect(view.selected).toBe('USD');
		expect(view.failed).toBe('');
	});

	test('ignores an older successful response after a newer currency choice commits', async () => {
		const oldRequest = deferredRate();
		const view = new CRMViewCurrency(async (_amount, _from, to) => to === 'USD' ? oldRequest.promise : quote(to, 0.001));
		const oldChoice = view.choose('USD', ['KRW']);
		expect(await view.choose('EUR', ['KRW'])).toBe(true);
		oldRequest.resolve(quote('USD', 0.00075));
		expect(await oldChoice).toBe(false);
		expect(view.selected).toBe('EUR');
		expect(view.viewAmount(1000, 'KRW')).toEqual({ value: 1, currency: 'EUR', isConverted: true });
		expect(view.failed).toBe('');
	});

	test('an older rejected request cannot clear the latest loading state or report its error', async () => {
		const oldRequest = deferredRate();
		const newRequest = deferredRate();
		const view = new CRMViewCurrency(async (_amount, _from, to) => to === 'USD' ? oldRequest.promise : newRequest.promise);
		const oldChoice = view.choose('USD', ['KRW']);
		const newChoice = view.choose('EUR', ['KRW']);
		oldRequest.reject(new Error('old request failed'));
		expect(await oldChoice).toBe(false);
		expect(view.isLoading).toBe(true);
		expect(view.pending).toBe('EUR');
		expect(view.failed).toBe('');
		newRequest.resolve(quote('EUR', 0.001));
		expect(await newChoice).toBe(true);
		expect(view.isLoading).toBe(false);
	});

	test('reuses only the committed target rates when a new source arrives or a pending choice is reversed', async () => {
		const calls: Array<[string, string]> = [];
		const delayed = deferredRate();
		const view = new CRMViewCurrency(async (_amount, from, to) => {
			calls.push([from, to]);
			if (to === 'USD') return delayed.promise;
			return quote(to, from === 'USD' ? 1350 : 9.2);
		});
		await view.follow('KRW', ['KRW']);
		await view.follow('KRW', ['KRW', 'JPY']);
		expect(calls).toEqual([['USD', 'KRW'], ['JPY', 'KRW']]);
		const choosingDollars = view.choose('USD', ['KRW', 'JPY']);
		await view.choose('KRW', ['KRW', 'JPY']);
		expect(calls).toEqual([['USD', 'KRW'], ['JPY', 'KRW'], ['KRW', 'USD'], ['JPY', 'USD']]);
		delayed.resolve(quote('USD', 0.00075));
		await choosingDollars;
		expect(view.selected).toBe('KRW');
		expect(view.ratesBySource).toEqual({ USD: 1350, JPY: 9.2 });
	});

	test('clears a failure when its source disappears and retries if that source returns', async () => {
		let failYen = true;
		const view = new CRMViewCurrency(async (_amount, from, to) => from === 'JPY' && failYen ? null : quote(to, 9.2));
		await view.follow('KRW', ['KRW']);
		await view.follow('KRW', ['KRW', 'JPY']);
		expect(view.failed).toBe('KRW');
		await view.follow('KRW', ['KRW']);
		expect(view.failed).toBe('');
		failYen = false;
		await view.follow('KRW', ['KRW', 'JPY']);
		expect(view.viewAmount(100, 'JPY')).toEqual({ value: 100 * 9.2, currency: 'KRW', isConverted: true });
	});

	for (const invalid of [quote('EUR', 0.00075), quote('USD', 0), quote('USD', -1), quote('USD', Infinity)]) {
		test(`refuses an unusable quote (${invalid.currencyCode}, ${invalid.rate}) without relabeling money`, async () => {
			const view = new CRMViewCurrency(async () => invalid);
			expect(await view.choose('USD', ['KRW'])).toBe(false);
			expect(view.selected).toBe('');
			expect(view.ratesBySource).toEqual({});
			expect(view.viewAmount(1000, 'KRW')).toEqual({ value: 1000, currency: 'KRW', isConverted: false });
		});
	}
});
