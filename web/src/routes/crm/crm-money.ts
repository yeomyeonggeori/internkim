import type { CRMCurrency, CRMMoneyTotals, CRMOpportunity } from './crm-types';
import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
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
	noValue = '-',
	locale: Locale = 'ko'
): string {
	if (value === undefined) return noValue;
	return new Intl.NumberFormat(locale, {
		style: 'currency',
		currency,
		currencyDisplay: 'narrowSymbol',
		notation: 'compact',
		maximumFractionDigits: 1,
		useGrouping: 'always'
	}).format(value);
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
	locale: Locale = 'ko'
): string {
	return formatMoney(view.value, view.currency, noValue, locale);
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
	return formatViewMoney({ value: collapsedValue, currency: view.selected, isConverted: true }, noValue, locale);
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
