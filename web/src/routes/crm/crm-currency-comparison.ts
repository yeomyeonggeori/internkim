import type { CRMCurrency, CRMMoneyTotals } from './crm-types';
import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';

export type CRMCurrencyComparisonRow = {
	currency: CRMCurrency;
	expectedValue: number | undefined;
	wonValue: number | undefined;
	expectedPercent: number;
	wonPercent: number;
};

export function buildCRMCurrencyComparisonRows(
	catalogue: CurrencyCatalogue,
	expectedTotals: CRMMoneyTotals,
	wonTotals: CRMMoneyTotals
): CRMCurrencyComparisonRow[] {
	return catalogue
		.map((entry) => entry.code)
		.filter((currency) => expectedTotals[currency] !== undefined || wonTotals[currency] !== undefined)
		.map((currency) => {
			const expectedValue = expectedTotals[currency];
			const wonValue = wonTotals[currency];
			const maximumValue = Math.max(expectedValue ?? 0, wonValue ?? 0);

			return {
				currency,
				expectedValue,
				wonValue,
				expectedPercent: percentageOfMaximum(expectedValue, maximumValue),
				wonPercent: percentageOfMaximum(wonValue, maximumValue)
			};
		});
}

function percentageOfMaximum(value: number | undefined, maximumValue: number): number {
	if (value === undefined || maximumValue <= 0) return 0;
	return (value / maximumValue) * 100;
}
