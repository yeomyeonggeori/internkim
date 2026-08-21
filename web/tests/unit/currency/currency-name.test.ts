import { describe, expect, test } from 'bun:test';
import { currencyDisplayNamesFor, currencyNameOf } from '../../../src/lib/currency/currency-name';

const guernseyPound = { code: 'GGP', name: 'Guernsey Pound', minorUnitDigits: 2 };
const taiwanDollar = { code: 'TWD', name: 'New Taiwan Dollar', minorUnitDigits: 2 };

describe('currencyNameOf', () => {
	test('names a currency in the reader locale when the platform knows it', () => {
		expect(currencyNameOf(taiwanDollar, currencyDisplayNamesFor('ko'))).toBe('신 타이완 달러');
		expect(currencyNameOf(taiwanDollar, currencyDisplayNamesFor('en'))).toBe('New Taiwan Dollar');
	});

	test('falls back to the provider name rather than repeating the code', () => {
		const koreanName = currencyNameOf(guernseyPound, currencyDisplayNamesFor('ko'));
		expect(koreanName).toBe('Guernsey Pound');
		expect(koreanName).not.toBe(guernseyPound.code);
	});

	test('falls back for every currency the platform cannot name', () => {
		const displayNames = currencyDisplayNamesFor('ko');
		for (const code of ['GGP', 'IMP', 'JEP']) {
			const entry = { code, name: `${code} provider name`, minorUnitDigits: 2 };
			expect(currencyNameOf(entry, displayNames)).toBe(entry.name);
		}
	});
});
