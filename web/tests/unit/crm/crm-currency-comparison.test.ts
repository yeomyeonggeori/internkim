import { describe, expect, test } from 'bun:test';
import { crmInterimCurrencyCatalogue } from '../../../src/routes/crm/crm-money';
import { buildCRMCurrencyComparisonRows } from '../../../src/routes/crm/crm-currency-comparison';

describe('CRM currency comparison', () => {
	test('compares expected and won values within each currency', () => {
		expect(
			buildCRMCurrencyComparisonRows(crmInterimCurrencyCatalogue, 
				{ KRW: 200, USD: 80 },
				{ KRW: 100, EUR: 50 }
			)
		).toEqual([
			{
				currency: 'KRW',
				expectedValue: 200,
				wonValue: 100,
				expectedPercent: 100,
				wonPercent: 50
			},
			{
				currency: 'USD',
				expectedValue: 80,
				wonValue: undefined,
				expectedPercent: 100,
				wonPercent: 0
			},
			{
				currency: 'EUR',
				expectedValue: undefined,
				wonValue: 50,
				expectedPercent: 0,
				wonPercent: 100
			}
		]);
	});

	test('keeps recorded zero values without creating an invalid scale', () => {
		expect(buildCRMCurrencyComparisonRows(crmInterimCurrencyCatalogue, { KRW: 0 }, {})).toEqual([
			{
				currency: 'KRW',
				expectedValue: 0,
				wonValue: undefined,
				expectedPercent: 0,
				wonPercent: 0
			}
		]);
	});

	test('returns no rows when both totals are empty', () => {
		expect(buildCRMCurrencyComparisonRows(crmInterimCurrencyCatalogue, {}, {})).toEqual([]);
	});
});
