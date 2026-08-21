import type { CRMCurrency, CRMMoneyTotals, CRMOpportunity } from './crm-types';
import { findCurrencyCatalogueEntry, type CurrencyCatalogue } from '$lib/currency/currency-catalogue';
import type { Locale } from '$lib/i18n/locale.svelte';
import type { CRMViewAmount, CRMViewCurrencyReader } from './crm-view-currency.svelte';

export const rateHintAnchorCurrency = 'USD';

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
	noValue = '-',
	locale: Locale = 'ko'
): string {
	if (value === undefined) return noValue;
	const entry = findCurrencyCatalogueEntry(catalogue, currency);
	if (!entry) return `${currency} ${value.toLocaleString()}`;
	return entry.grouping === 'myriad' && locale === 'ko'
		? formatMyriadMoney(value, entry.code)
		: formatThousandGroupedMoney(value, entry.code);
}

export function formatMoneyTotals(
	totals: CRMMoneyTotals,
	catalogue: CurrencyCatalogue,
	noValue = '-',
	locale: Locale = 'ko'
): string {
	const values = catalogue
		.filter((entry) => totals[entry.code] !== undefined)
		.map((entry) => formatMoney(totals[entry.code], entry.code, catalogue, noValue, locale));
	return values.length === 0 ? noValue : values.join(' · ');
}

export function formatViewMoney(
	view: CRMViewAmount,
	catalogue: CurrencyCatalogue,
	noValue = '-',
	locale: Locale = 'ko'
): string {
	const formatted = formatMoney(view.value, view.currency, catalogue, noValue, locale);
	return formatted;
}

export function formatViewMoneyTotals(
	totals: CRMMoneyTotals,
	catalogue: CurrencyCatalogue,
	view: CRMViewCurrencyReader,
	noValue = '-',
	locale: Locale = 'ko'
): string {
	if (view.selected === '') return formatMoneyTotals(totals, catalogue, noValue, locale);
	const currencies = Object.keys(totals);
	if (currencies.length === 0) return noValue;
	const viewAmounts = currencies.flatMap((currency) => {
		const amount = totals[currency];
		return amount === undefined ? [] : [view.viewAmount(amount, currency)];
	});
	if (viewAmounts.some((amount) => amount.currency !== view.selected)) {
		return formatMoneyTotals(totals, catalogue, noValue, locale);
	}
	const collapsedValue = viewAmounts.reduce((sum, amount) => sum + amount.value, 0);
	return formatViewMoney({ value: collapsedValue, currency: view.selected, isConverted: true }, catalogue, noValue, locale);
}

export function formatViewRateHint(
	viewCurrency: string,
	ratesBySource: Record<string, number>,
	catalogue: CurrencyCatalogue
): string {
	if (viewCurrency === '' || viewCurrency === rateHintAnchorCurrency) return '';
	const rate = ratesBySource[rateHintAnchorCurrency];
	if (rate === undefined || !(rate > 0)) return '';
	return `${formatRateSide(1, rateHintAnchorCurrency)} = ${formatRateSide(rate, viewCurrency)}`;
}

function formatRateSide(value: number, currency: string): string {
	const rounded = value >= 100 ? Math.round(value) : Number(value.toFixed(2));
	return `${currency} ${rounded.toLocaleString()}`;
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

function formatMyriadMoney(value: number, currencyCode: string): string {
	if (value >= 100000000) {
		const hundredMillions = Number((value / 100000000).toFixed(1));
		return `${currencyCode} ${hundredMillions.toLocaleString()}억`;
	}
	if (value >= 10000) {
		return `${currencyCode} ${Math.round(value / 10000).toLocaleString()}만`;
	}
	return `${currencyCode} ${value.toLocaleString()}`;
}

function formatThousandGroupedMoney(value: number, currencyCode: string): string {
	if (value >= 1000000) {
		const millions = Number((value / 1000000).toFixed(1));
		return `${currencyCode} ${millions.toLocaleString()}M`;
	}
	if (value >= 1000) {
		const thousands = Number((value / 1000).toFixed(1));
		return `${currencyCode} ${thousands.toLocaleString()}K`;
	}
	return `${currencyCode} ${value.toLocaleString()}`;
}
