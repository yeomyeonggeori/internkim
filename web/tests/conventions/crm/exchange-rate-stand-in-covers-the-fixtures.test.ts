import { readFileSync } from 'node:fs';
import { expect, test } from 'bun:test';
import { standInCurrencies } from '../../support/exchange-rate-stand-in';

function currenciesTheLocalFixturesHold(): string[] {
	const fixtures = readFileSync('../supabase/seed.dev.sql', 'utf8');
	const isoCurrencies = new Set(Intl.supportedValuesOf('currency'));
	const quotedCodes = [...fixtures.matchAll(/'([A-Z]{3})'/g)].map((match) => match[1]);
	return [...new Set(quotedCodes.filter((code) => isoCurrencies.has(code)))].sort();
}

test('the end-to-end exchange-rate stand-in prices every currency the local fixtures hold', () => {
	const held = currenciesTheLocalFixturesHold();
	expect(held.length).toBeGreaterThan(0);
	expect(held.filter((code) => standInCurrencies[code] === undefined)).toEqual([]);
});
