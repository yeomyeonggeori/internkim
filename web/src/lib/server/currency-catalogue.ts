import type { SupportedCurrency } from '$lib/server/exchange-rates';

export type CurrencyCatalogueEntry = {
	code: string;
	name: string;
	minorUnitDigits: number;
	grouping: 'myriad' | 'thousand';
};

const myriadGroupedCurrencies = new Set(['KRW', 'JPY', 'CNY', 'TWD', 'HKD', 'MOP']);

export function currencyCatalogueOf(currencies: SupportedCurrency[]): CurrencyCatalogueEntry[] {
	return currencies
		.map((currency) => ({
			code: currency.code,
			name: currency.name,
			minorUnitDigits: currency.minorUnitDigits,
			grouping: myriadGroupedCurrencies.has(currency.code) ? ('myriad' as const) : ('thousand' as const)
		}))
		.sort((left, right) => left.code.localeCompare(right.code));
}
