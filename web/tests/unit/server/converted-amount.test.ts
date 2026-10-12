import { describe, expect, test } from 'bun:test';
import { convertedAmount } from '../../../src/lib/server/converted-amount';
import type { ExchangeRate, ExchangeRateProvider } from '../../../src/lib/server/exchange-rates';

function mockProvider(rate: ExchangeRate): ExchangeRateProvider {
	return {
		supportedCurrencies: async () => [],
		latestRate: async (base, quote) => {
			expect(base).toBe(rate.base);
			expect(quote).toBe(rate.quote);
			return rate;
		}
	};
}

describe('convertedAmount', () => {
	test('converts between currencies with differing minor-unit digits', async () => {
		const provider = mockProvider({ base: 'USD', quote: 'KRW', rate: 1350.12, asOf: '2026-08-19' });

		const converted = await convertedAmount(provider, { amount: 1200, currency: 'USD' }, 'KRW');

		expect(converted).toEqual({ amount: { amount: 1620144, currency: 'KRW' }, rate: 1350.12, asOf: '2026-08-19' });
	});

	test('returns the amount unchanged for a same-currency conversion', async () => {
		const provider = mockProvider({ base: 'USD', quote: 'USD', rate: 1, asOf: '2026-08-19' });

		const converted = await convertedAmount(provider, { amount: 1200, currency: 'USD' }, 'USD');

		expect(converted).toEqual({ amount: { amount: 1200, currency: 'USD' }, rate: 1, asOf: '2026-08-19' });
	});
});
