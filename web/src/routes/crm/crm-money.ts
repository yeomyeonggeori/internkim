import type { CRMMoneyTotals, CRMOpportunity } from './crm-types';
import { formatCompactMoney, type Money } from '$lib/money';
import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
import type { Locale } from '$lib/i18n/locale.svelte';
import type { CRMViewCurrencyReader } from './crm-view-currency.svelte';

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

export function moneyFromAmountInput(value: string, currency: string): Money | undefined {
	const amount = parseAmountInput(value);
	return amount === undefined ? undefined : { amount, currency };
}

export function formatOptionalMoney(money: Money | undefined, noValue = '-', locale: Locale = 'ko'): string {
	return money === undefined ? noValue : formatCompactMoney(money, locale);
}

export function formatMoneyTotals(
	totals: CRMMoneyTotals,
	catalogue: CurrencyCatalogue,
	noValue = '-',
	locale: Locale = 'ko'
): string {
	const values = catalogue.flatMap((entry) => {
		const amount = totals[entry.code];
		return amount === undefined ? [] : [formatOptionalMoney({ amount, currency: entry.code }, noValue, locale)];
	});
	return values.length === 0 ? noValue : values.join(' · ');
}

export function collapsedViewMoneyTotal(totals: CRMMoneyTotals, view: CRMViewCurrencyReader): Money | undefined {
	if (view.selected === '') return undefined;
	const currencies = Object.keys(totals);
	if (currencies.length === 0) return undefined;
	const viewAmounts = currencies.flatMap((currency) => {
		const amount = totals[currency];
		return amount === undefined ? [] : [view.viewAmount({ amount, currency })];
	});
	if (viewAmounts.some((viewAmount) => viewAmount.currency !== view.selected)) return undefined;
	return { amount: viewAmounts.reduce((sum, viewAmount) => sum + viewAmount.amount, 0), currency: view.selected };
}

export function formatViewMoneyTotals(
	totals: CRMMoneyTotals,
	catalogue: CurrencyCatalogue,
	view: CRMViewCurrencyReader,
	noValue = '-',
	locale: Locale = 'ko'
): string {
	const collapsedTotal = collapsedViewMoneyTotal(totals, view);
	if (collapsedTotal === undefined) return formatMoneyTotals(totals, catalogue, noValue, locale);
	return formatOptionalMoney(collapsedTotal, noValue, locale);
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

export function sumOpportunityMoney(opportunities: Array<Pick<CRMOpportunity, 'amount'>>): CRMMoneyTotals {
	return opportunities.reduce<CRMMoneyTotals>((totals, opportunity) => {
		const money = opportunity.amount;
		if (money === undefined) return totals;
		return { ...totals, [money.currency]: (totals[money.currency] ?? 0) + money.amount };
	}, {});
}
