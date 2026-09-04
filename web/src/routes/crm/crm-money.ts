import type { CRMCurrency, CRMMoneyTotals, CRMOpportunity } from './crm-types';
import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
import type { Locale } from '$lib/i18n/locale.svelte';
import type { CRMViewAmount, CRMViewCurrencyReader } from './crm-view-currency.svelte';

export const rateHintAnchorCurrency = 'USD';

const roundedMyriadThreshold = 100;

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
	noValue = '-',
	locale: Locale = 'ko'
): string {
	return formatAmount(value, noValue, locale, `${currency} `);
}

export function formatAmountAlone(
	value: number | undefined,
	noValue = '-',
	locale: Locale = 'ko'
): string {
	return formatAmount(value, noValue, locale, '');
}

function formatAmount(value: number | undefined, noValue: string, locale: Locale, prefix: string): string {
	if (value === undefined) return noValue;
	return locale === 'ko' ? formatMyriadMoney(value, prefix) : formatThousandGroupedMoney(value, prefix);
}

export function formatMoneyTotals(
	totals: CRMMoneyTotals,
	catalogue: CurrencyCatalogue,
	noValue = '-',
	locale: Locale = 'ko'
): string {
	const values = catalogue
		.filter((entry) => totals[entry.code] !== undefined)
		.map((entry) => formatMoney(totals[entry.code], entry.code, noValue, locale));
	return values.length === 0 ? noValue : values.join(' · ');
}

export function formatViewMoney(
	view: CRMViewAmount,
	noValue = '-',
	locale: Locale = 'ko',
	viewCurrency = ''
): string {
	return viewCurrency !== '' && view.currency === viewCurrency
		? formatAmountAlone(view.value, noValue, locale)
		: formatMoney(view.value, view.currency, noValue, locale);
}

export function collapsedViewMoneyTotal(totals: CRMMoneyTotals, view: CRMViewCurrencyReader): number | undefined {
	if (view.selected === '') return undefined;
	const currencies = Object.keys(totals);
	if (currencies.length === 0) return undefined;
	const viewAmounts = currencies.flatMap((currency) => {
		const amount = totals[currency];
		return amount === undefined ? [] : [view.viewAmount(amount, currency)];
	});
	if (viewAmounts.some((amount) => amount.currency !== view.selected)) return undefined;
	return viewAmounts.reduce((sum, amount) => sum + amount.value, 0);
}

export function formatViewMoneyTotals(
	totals: CRMMoneyTotals,
	catalogue: CurrencyCatalogue,
	view: CRMViewCurrencyReader,
	noValue = '-',
	locale: Locale = 'ko'
): string {
	const collapsedValue = collapsedViewMoneyTotal(totals, view);
	if (collapsedValue === undefined) return formatMoneyTotals(totals, catalogue, noValue, locale);
	return formatViewMoney({ value: collapsedValue, currency: view.selected, isConverted: true }, noValue, locale, view.selected);
}

export function formatViewRateHint(viewCurrency: string, ratesBySource: Record<string, number>): string {
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

function formatMyriadMoney(value: number, prefix: string): string {
	if (value >= 100000000) {
		const hundredMillions = Number((value / 100000000).toFixed(1));
		return `${prefix}${hundredMillions.toLocaleString()}억`;
	}
	if (value >= 10000) {
		const tenThousands = value / 10000;
		const shown = tenThousands >= roundedMyriadThreshold ? Math.round(tenThousands) : Number(tenThousands.toFixed(1));
		return `${prefix}${shown.toLocaleString()}만`;
	}
	return `${prefix}${value.toLocaleString()}`;
}

function formatThousandGroupedMoney(value: number, prefix: string): string {
	if (value >= 1000000) {
		const millions = Number((value / 1000000).toFixed(1));
		return `${prefix}${millions.toLocaleString()}M`;
	}
	if (value >= 1000) {
		const thousands = Number((value / 1000).toFixed(1));
		return `${prefix}${thousands.toLocaleString()}K`;
	}
	return `${prefix}${value.toLocaleString()}`;
}
