import { crmInterimCurrencyCatalogue } from './crm-money';
import type { CRMCurrency, CRMMoneyTotals } from './crm-types';

export type CRMCurrencyComparisonRow = {
	currency: CRMCurrency;
	expectedValue: number | undefined;
	wonValue: number | undefined;
	expectedPercent: number;
	wonPercent: number;
};

export function buildCRMCurrencyComparisonRows(
	expectedTotals: CRMMoneyTotals,
	wonTotals: CRMMoneyTotals
): CRMCurrencyComparisonRow[] {
	return crmInterimCurrencyCatalogue
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
