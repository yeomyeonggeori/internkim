import type { CRMCurrency, CRMMoneyTotals, CRMOpportunity } from './crm-types';

export const crmCurrencies: CRMCurrency[] = ['KRW', 'USD', 'JPY', 'EUR'];
const crmCurrencySet = new Set<string>(crmCurrencies);
const myriadGroupedCurrencies = new Set<CRMCurrency>(['KRW', 'JPY']);
export const crmCurrencySymbols: Record<CRMCurrency, string> = {
	KRW: '₩',
	USD: '$',
	JPY: '¥',
	EUR: '€'
};

export function isCRMCurrency(value: string): value is CRMCurrency {
	return crmCurrencySet.has(value);
}

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

export function formatMoney(value: number | undefined, currency: CRMCurrency, noValue = '-'): string {
	if (value === undefined) return noValue;
	const symbol = crmCurrencySymbols[currency];
	return myriadGroupedCurrencies.has(currency)
		? formatMyriadMoney(value, symbol)
		: formatThousandGroupedMoney(value, symbol);
}

export function formatMoneyTotals(totals: CRMMoneyTotals, noValue = '-'): string {
	const values = crmCurrencies
		.filter((currency) => totals[currency] !== undefined)
		.map((currency) => formatMoney(totals[currency], currency, noValue));
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
