import type { CRMCurrency, CRMMoneyTotals, CRMOpportunity } from './crm-types';
import { findCurrencyCatalogueEntry, type CurrencyCatalogue } from '$lib/currency/currency-catalogue';

export function formatAmountInput(value: string): string {
	const digits = value.replace(/\D/g, '');
	if (digits === '') return '';
	const normalized = digits.replace(/^0+(?=\d)/, '');
	return normalized.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
}

export function parseAmountInput(value: string): number | undefined {
	const normalized = value.replaceAll(',', '');
	if (normalized === '') return undefined;
	const amount = Number(normalized);
	if (!Number.isSafeInteger(amount) || amount < 0) return undefined;
	return amount;
}

export function formatMoney(
	value: number | undefined,
	currency: CRMCurrency,
	catalogue: CurrencyCatalogue,
	noValue = '-'
): string {
	if (value === undefined) return noValue;
	const entry = findCurrencyCatalogueEntry(catalogue, currency);
	if (!entry) return `${currency}${value.toLocaleString()}`;
	return entry.grouping === 'myriad'
		? formatMyriadMoney(value, entry.symbol)
		: formatThousandGroupedMoney(value, entry.symbol);
}

export function formatMoneyTotals(totals: CRMMoneyTotals, catalogue: CurrencyCatalogue, noValue = '-'): string {
	const values = catalogue
		.filter((entry) => totals[entry.code] !== undefined)
		.map((entry) => formatMoney(totals[entry.code], entry.code, catalogue, noValue));
	return values.length === 0 ? noValue : values.join(' · ');
}

export function sumOpportunityMoney(
	opportunities: Array<Pick<CRMOpportunity, 'currency' | 'expectedValue'>>
): CRMMoneyTotals {
	return opportunities.reduce<CRMMoneyTotals>((totals, opportunity) => {
		const amount = opportunity.expectedValue;
		if (amount === undefined) return totals;
		return { ...totals, [opportunity.currency]: (totals[opportunity.currency] ?? 0) + amount };
	}, {});
}

function formatMyriadMoney(value: number, currencySymbol: string): string {
	if (value >= 100000000) {
		const hundredMillions = Number((value / 100000000).toFixed(1));
		return `${currencySymbol}${hundredMillions.toLocaleString()}억`;
	}
	if (value >= 10000) {
		return `${currencySymbol}${Math.round(value / 10000).toLocaleString()}만`;
	}
	return `${currencySymbol}${value.toLocaleString()}`;
}

function formatThousandGroupedMoney(value: number, currencySymbol: string): string {
	if (value >= 1000000) {
		const millions = Number((value / 1000000).toFixed(1));
		return `${currencySymbol}${millions.toLocaleString()}M`;
	}
	if (value >= 1000) {
		const thousands = Number((value / 1000).toFixed(1));
		return `${currencySymbol}${thousands.toLocaleString()}K`;
	}
	return `${currencySymbol}${value.toLocaleString()}`;
}
