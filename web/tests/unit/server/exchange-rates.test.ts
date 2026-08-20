import { describe, expect, test } from 'bun:test';
import { createMockFetch } from '../test-fetch';
import { convertMinorAmount, frankfurterProvider, minorUnitDigitsOf } from '../../../src/lib/server/exchange-rates';

const oneHourInMilliseconds = 60 * 60 * 1000;
const twelveHoursInMilliseconds = 12 * oneHourInMilliseconds;
const epochInMilliseconds = Date.UTC(2026, 0, 1);

describe('minorUnitDigitsOf', () => {
	test('zero-decimal currencies have no minor digits', () => {
		expect(minorUnitDigitsOf('KRW')).toBe(0);
		expect(minorUnitDigitsOf('JPY')).toBe(0);
	});

	test('three-decimal currencies keep three minor digits', () => {
		expect(minorUnitDigitsOf('BHD')).toBe(3);
		expect(minorUnitDigitsOf('KWD')).toBe(3);
	});

	test('every other currency defaults to two minor digits', () => {
		expect(minorUnitDigitsOf('USD')).toBe(2);
		expect(minorUnitDigitsOf('EUR')).toBe(2);
	});
});

describe('convertMinorAmount', () => {
	test('converts 1200.00 USD minor units to KRW minor units at the quoted rate', () => {
		expect(convertMinorAmount(120000, 1350.12, 2, 0)).toBe(1620144);
	});

	test('rounds a half unit up when the target currency has no minor digits', () => {
		expect(convertMinorAmount(100, 250.5, 2, 0)).toBe(251);
	});

	test('rounds a half unit up when the target currency has three minor digits', () => {
		expect(convertMinorAmount(100, 0.3085, 2, 3)).toBe(309);
	});
});

describe('frankfurterProvider().supportedCurrencies', () => {
	test('walks the cache through a cold failure, population, a hit, expiry, and a fallback', async () => {
		let fetchCallCount = 0;
		let fetchShouldFail = true;
		let currentTimeInMilliseconds = epochInMilliseconds;
		const provider = frankfurterProvider({
			fetch: createMockFetch(async () => {
				fetchCallCount += 1;
				if (fetchShouldFail) return new Response('service unavailable', { status: 503 });
				return Response.json({ USD: 'United States Dollar', KRW: 'South Korean Won' });
			}),
			now: () => currentTimeInMilliseconds
		});
		const expectedList = [
			{ code: 'USD', name: 'United States Dollar', minorUnitDigits: 2 },
			{ code: 'KRW', name: 'South Korean Won', minorUnitDigits: 0 }
		];

		try {
			await provider.supportedCurrencies();
			throw new Error('expected supportedCurrencies to reject on a cold cache');
		} catch (error) {
			if (!(error instanceof Error)) throw error;
			expect(error.message).toContain('Frankfurter');
			expect(error.message).toContain('/v1/currencies');
			expect(error.message).toContain('503');
		}
		expect(fetchCallCount).toBe(1);

		fetchShouldFail = false;
		expect(await provider.supportedCurrencies()).toEqual(expectedList);
		expect(fetchCallCount).toBe(2);

		currentTimeInMilliseconds = epochInMilliseconds + oneHourInMilliseconds;
		expect(await provider.supportedCurrencies()).toEqual(expectedList);
		expect(fetchCallCount).toBe(2);

		currentTimeInMilliseconds = epochInMilliseconds + twelveHoursInMilliseconds + 1;
		expect(await provider.supportedCurrencies()).toEqual(expectedList);
		expect(fetchCallCount).toBe(3);

		fetchShouldFail = true;
		currentTimeInMilliseconds = epochInMilliseconds + 2 * twelveHoursInMilliseconds + 2;
		expect(await provider.supportedCurrencies()).toEqual(expectedList);
		expect(fetchCallCount).toBe(4);
	});
});

describe('frankfurterProvider().latestRate', () => {
	test('same currency returns rate 1 for today in UTC without calling fetch', async () => {
		let fetchCallCount = 0;
		const provider = frankfurterProvider({
			fetch: createMockFetch(async () => {
				fetchCallCount += 1;
				throw new Error('fetch should not be called for a same-currency lookup');
			}),
			now: () => Date.UTC(2026, 7, 19, 3, 30, 0)
		});

		const rate = await provider.latestRate('KRW', 'KRW');

		expect(rate).toEqual({ base: 'KRW', quote: 'KRW', rate: 1, asOf: '2026-08-19' });
		expect(fetchCallCount).toBe(0);
	});

	test('parses a real latest-rate response', async () => {
		let requestedURL = '';
		const provider = frankfurterProvider({
			fetch: createMockFetch(async (input) => {
				requestedURL = String(input);
				return Response.json({ amount: 1, base: 'USD', date: '2026-08-19', rates: { KRW: 1350.12 } });
			}),
			now: () => epochInMilliseconds
		});

		const rate = await provider.latestRate('USD', 'KRW');

		expect(rate).toEqual({ base: 'USD', quote: 'KRW', rate: 1350.12, asOf: '2026-08-19' });
		expect(requestedURL).toBe('https://api.frankfurter.dev/v1/latest?base=USD&symbols=KRW');
	});

	test('rejects a payload missing rates', async () => {
		const provider = frankfurterProvider({
			fetch: createMockFetch(async () => Response.json({ amount: 1, base: 'USD', date: '2026-08-19' })),
			now: () => epochInMilliseconds
		});

		await expect(provider.latestRate('USD', 'KRW')).rejects.toThrow('missing rates');
	});

	test('rejects a payload missing the requested quote', async () => {
		const provider = frankfurterProvider({
			fetch: createMockFetch(async () =>
				Response.json({ amount: 1, base: 'USD', date: '2026-08-19', rates: { EUR: 0.92 } })
			),
			now: () => epochInMilliseconds
		});

		await expect(provider.latestRate('USD', 'KRW')).rejects.toThrow('no rate for KRW');
	});

	test('rejects a non-finite rate', async () => {
		const overflowingRateBody = '{"amount":1,"base":"USD","date":"2026-08-19","rates":{"KRW":1e400}}';
		const provider = frankfurterProvider({
			fetch: createMockFetch(async () => new Response(overflowingRateBody, { status: 200 })),
			now: () => epochInMilliseconds
		});

		await expect(provider.latestRate('USD', 'KRW')).rejects.toThrow('not a finite number');
	});

	test('names the provider, the path, and the cause when the network request fails', async () => {
		const provider = frankfurterProvider({
			fetch: createMockFetch(async () => {
				throw new Error('network unreachable');
			}),
			now: () => epochInMilliseconds
		});

		try {
			await provider.latestRate('USD', 'KRW');
			throw new Error('expected latestRate to reject');
		} catch (error) {
			if (!(error instanceof Error)) throw error;
			expect(error.message).toContain('Frankfurter');
			expect(error.message).toContain('/v1/latest');
			expect(error.message).toContain('network unreachable');
		}
	});

	test('caches a rate for one hour, refetches after expiry, and falls back to the cache on a later failure', async () => {
		let fetchCallCount = 0;
		let fetchShouldFail = false;
		let currentTimeInMilliseconds = epochInMilliseconds;
		const provider = frankfurterProvider({
			fetch: createMockFetch(async () => {
				fetchCallCount += 1;
				if (fetchShouldFail) return new Response('service unavailable', { status: 503 });
				return Response.json({ amount: 1, base: 'USD', date: '2026-08-19', rates: { KRW: 1350.12 } });
			}),
			now: () => currentTimeInMilliseconds
		});
		const expectedRate = { base: 'USD', quote: 'KRW', rate: 1350.12, asOf: '2026-08-19' };

		expect(await provider.latestRate('USD', 'KRW')).toEqual(expectedRate);
		expect(fetchCallCount).toBe(1);

		currentTimeInMilliseconds = epochInMilliseconds + 30 * 60 * 1000;
		expect(await provider.latestRate('USD', 'KRW')).toEqual(expectedRate);
		expect(fetchCallCount).toBe(1);

		currentTimeInMilliseconds = epochInMilliseconds + oneHourInMilliseconds + 1;
		expect(await provider.latestRate('USD', 'KRW')).toEqual(expectedRate);
		expect(fetchCallCount).toBe(2);

		fetchShouldFail = true;
		currentTimeInMilliseconds = epochInMilliseconds + 2 * oneHourInMilliseconds + 2;
		expect(await provider.latestRate('USD', 'KRW')).toEqual(expectedRate);
		expect(fetchCallCount).toBe(3);
	});
});
