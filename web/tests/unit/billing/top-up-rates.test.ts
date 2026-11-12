import { describe, expect, test } from 'bun:test';
import { creditsBoughtWith } from '../../../src/lib/server/stripe-top-up';
import { topUpRatesOf } from '../../../src/lib/server/top-up-rates';

describe('what a top-up buys', () => {
	test('the deployment names the credits each currency buys', () => {
		expect(topUpRatesOf('{"krw": 2, "usd": 150}')).toEqual({ krw: 2, usd: 150 });
		expect(topUpRatesOf(undefined)).toBeNull();
		expect(topUpRatesOf('')).toBeNull();
	});

	test('a setting that is not currencies and positive rates is refused by name', () => {
		expect(() => topUpRatesOf('two')).toThrow('STRIPE_TOP_UP_RATES');
		expect(() => topUpRatesOf('{"krw": 0}')).toThrow('STRIPE_TOP_UP_RATES');
	});

	test('an amount buys whole credits, only in a currency that has both a product and a rate', () => {
		expect(creditsBoughtWith({ krw: 2 }, 30000, 'krw')).toBe(60000);
		expect(creditsBoughtWith({ krw: 0.3 }, 5, 'krw')).toBe(1);
		expect(creditsBoughtWith({ krw: 2 }, 100, 'usd')).toBeNull();
		expect(creditsBoughtWith({ jpy: 2 }, 100, 'jpy')).toBeNull();
	});
});
