import { describe, expect, test } from 'bun:test';
import { fractionDigitsOf, roundedMoney } from '../../src/lib/money';

describe('money', () => {
	test('knows how many decimals a currency has', () => {
		expect(fractionDigitsOf('KRW')).toBe(0);
		expect(fractionDigitsOf('JPY')).toBe(0);
		expect(fractionDigitsOf('USD')).toBe(2);
		expect(fractionDigitsOf('EUR')).toBe(2);
		expect(fractionDigitsOf('KWD')).toBe(3);
		expect(fractionDigitsOf('BHD')).toBe(3);
	});

	test('rounds an amount half up to the decimals its currency has', () => {
		expect(roundedMoney(1200 * 1350.12, 'KRW')).toEqual({ amount: 1620144, currency: 'KRW' });
		expect(roundedMoney(250.5, 'KRW')).toEqual({ amount: 251, currency: 'KRW' });
		expect(roundedMoney(0.3085, 'KWD')).toEqual({ amount: 0.309, currency: 'KWD' });
		expect(roundedMoney(0.1 + 0.2, 'USD')).toEqual({ amount: 0.3, currency: 'USD' });
	});
});
