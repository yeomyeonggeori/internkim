import { describe, expect, test } from 'bun:test';
import { createMockFetch } from '../test-fetch';
import { convertMinorAmount, frankfurterProvider, minorUnitDigitsOf } from '../../../src/lib/server/exchange-rates';

const oneHourInMilliseconds = 60 * 60 * 1000;
const twelveHoursInMilliseconds = 12 * oneHourInMilliseconds;
const epochInMilliseconds = Date.UTC(2026, 0, 1);
const requestTimeoutInMilliseconds = 10;

function pendingResponseUntilAbort(signal: AbortSignal | null | undefined): Promise<Response> {
	if (!signal) throw new Error('expected the provider to pass an abort signal');
	return new Promise((_, reject) => {
		signal.addEventListener('abort', () => reject(signal.reason), { once: true });
	});
}

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
	test('aborts a stalled request and retries without caching the cold failure', async () => {
		let fetchCallCount = 0;
		let requestAborted = false;
		const provider = frankfurterProvider({
			fetch: createMockFetch(async (_, init) => {
				fetchCallCount += 1;
				if (fetchCallCount === 1) {
					init?.signal?.addEventListener(
						'abort',
						() => {
							requestAborted = true;
						},
						{ once: true }
					);
					return pendingResponseUntilAbort(init?.signal);
				}
				return Response.json([{ iso_code: 'USD', name: 'United States Dollar' }]);
			}),
			requestTimeoutInMilliseconds
		});

		await expect(provider.supportedCurrencies()).rejects.toThrow(
			'Frankfurter request to /v2/currencies timed out after 10ms'
		);
		expect(requestAborted).toBe(true);
		expect(await provider.supportedCurrencies()).toEqual([
			{ code: 'USD', name: 'United States Dollar', minorUnitDigits: 2 }
		]);
		expect(fetchCallCount).toBe(2);
	}, 1000);

	test('walks the cache through a cold failure, population, a hit, expiry, and a fallback', async () => {
		let fetchCallCount = 0;
		let fetchShouldFail = true;
		let currentTimeInMilliseconds = epochInMilliseconds;
		const provider = frankfurterProvider({
			fetch: createMockFetch(async () => {
				fetchCallCount += 1;
				if (fetchShouldFail) return new Response('service unavailable', { status: 503 });
				return Response.json([
					{ iso_code: 'USD', name: 'United States Dollar' },
					{ iso_code: 'KRW', name: 'South Korean Won' }
				]);
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
			expect(error.message).toContain('/v2/currencies');
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
	test('clears the deadline after a success or a non-timeout failure', async () => {
		const requestSignals: AbortSignal[] = [];
		const provider = frankfurterProvider({
			fetch: createMockFetch(async (_, init) => {
				if (!init?.signal) throw new Error('expected the provider to pass an abort signal');
				requestSignals.push(init.signal);
				if (requestSignals.length === 2) return new Response('service unavailable', { status: 503 });
				return Response.json([{ date: '2026-01-01', base: 'USD', quote: 'KRW', rate: 1400 }]);
			}),
			requestTimeoutInMilliseconds
		});

		await provider.latestRate('USD', 'KRW');
		await expect(provider.latestRate('EUR', 'KRW')).rejects.toThrow('503');
		await new Promise<void>((resolve) => setTimeout(resolve, 2 * requestTimeoutInMilliseconds));
		expect(requestSignals).toHaveLength(2);
		for (const signal of requestSignals) expect(signal.aborted).toBe(false);
	}, 1000);

	test('aborts a stalled response body and retries without caching an invented rate', async () => {
		let fetchCallCount = 0;
		let bodyAborted = false;
		const provider = frankfurterProvider({
			fetch: createMockFetch(async (_, init) => {
				fetchCallCount += 1;
				if (fetchCallCount > 1) {
					return Response.json([{ date: '2026-01-01', base: 'USD', quote: 'KRW', rate: 1400 }]);
				}
				const signal = init?.signal;
				if (!signal) throw new Error('expected the provider to pass an abort signal');
				return new Response(
					new ReadableStream({
						start(controller) {
							controller.enqueue(new TextEncoder().encode('[{"date":'));
							signal.addEventListener(
								'abort',
								() => {
									bodyAborted = true;
									controller.error(signal.reason);
								},
								{ once: true }
							);
						}
					})
				);
			}),
			requestTimeoutInMilliseconds
		});

		await expect(provider.latestRate('USD', 'KRW')).rejects.toThrow(
			'Frankfurter request to /v2/rates?base=USD&quotes=KRW timed out after 10ms'
		);
		expect(bodyAborted).toBe(true);
		expect(await provider.latestRate('USD', 'KRW')).toEqual({
			base: 'USD', quote: 'KRW', rate: 1400, asOf: '2026-01-01'
		});
		expect(fetchCallCount).toBe(2);
	}, 1000);

	test('preserves a cached rate and its date only for the same pair on timeout, then retries', async () => {
		let fetchCallCount = 0;
		let fetchShouldStall = false;
		let currentTimeInMilliseconds = epochInMilliseconds;
		const provider = frankfurterProvider({
			fetch: createMockFetch(async (_, init) => {
				fetchCallCount += 1;
				if (fetchShouldStall) return pendingResponseUntilAbort(init?.signal);
				return Response.json([
					fetchCallCount === 1
						? { date: '2025-12-30', base: 'USD', quote: 'KRW', rate: 1350.12 }
						: { date: '2026-01-01', base: 'USD', quote: 'KRW', rate: 1400 }
				]);
			}),
			now: () => currentTimeInMilliseconds,
			requestTimeoutInMilliseconds
		});
		const originalRate = { base: 'USD', quote: 'KRW', rate: 1350.12, asOf: '2025-12-30' };
		expect(await provider.latestRate('USD', 'KRW')).toEqual(originalRate);

		currentTimeInMilliseconds += oneHourInMilliseconds + 1;
		fetchShouldStall = true;
		expect(await provider.latestRate('USD', 'KRW')).toEqual(originalRate);
		expect(fetchCallCount).toBe(2);
		await expect(provider.latestRate('EUR', 'KRW')).rejects.toThrow('timed out');
		expect(fetchCallCount).toBe(3);

		fetchShouldStall = false;
		expect(await provider.latestRate('USD', 'KRW')).toEqual({
			base: 'USD', quote: 'KRW', rate: 1400, asOf: '2026-01-01'
		});
		expect(fetchCallCount).toBe(4);
	}, 1000);

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
				return Response.json([{ date: '2026-08-19', base: 'USD', quote: 'KRW', rate: 1350.12 }]);
			}),
			now: () => epochInMilliseconds
		});

		const rate = await provider.latestRate('USD', 'KRW');

		expect(rate).toEqual({ base: 'USD', quote: 'KRW', rate: 1350.12, asOf: '2026-08-19' });
		expect(requestedURL).toBe('https://api.frankfurter.dev/v2/rates?base=USD&quotes=KRW');
	});

	test('rejects a payload that is not the array of rate rows v2 returns', async () => {
		const provider = frankfurterProvider({
			fetch: createMockFetch(async () => Response.json({ amount: 1, base: 'USD', date: '2026-08-19' })),
			now: () => epochInMilliseconds
		});

		await expect(provider.latestRate('USD', 'KRW')).rejects.toThrow('expected a JSON array');
	});

	test('rejects a payload missing the requested quote', async () => {
		const provider = frankfurterProvider({
			fetch: createMockFetch(async () =>
				Response.json([{ date: '2026-08-19', base: 'USD', quote: 'EUR', rate: 0.92 }])
			),
			now: () => epochInMilliseconds
		});

		await expect(provider.latestRate('USD', 'KRW')).rejects.toThrow('no rate for KRW');
	});

	test('rejects a non-finite rate', async () => {
		const overflowingRateBody = '[{"date":"2026-08-19","base":"USD","quote":"KRW","rate":1e400}]';
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
			expect(error.message).toContain('/v2/rates');
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
				return Response.json([{ date: '2026-08-19', base: 'USD', quote: 'KRW', rate: 1350.12 }]);
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
