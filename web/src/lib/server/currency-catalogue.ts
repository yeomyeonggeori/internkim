import type { SupportedCurrency } from '$lib/server/exchange-rates';

export type CurrencyCatalogueEntry = {
	code: string;
	name: string;
	minorUnitDigits: number;
};

export function currencyCatalogueOf(currencies: SupportedCurrency[]): CurrencyCatalogueEntry[] {
	return currencies
		.map((currency) => ({
			code: currency.code,
			name: currency.name,
			minorUnitDigits: currency.minorUnitDigits
		}))
		.sort((left, right) => left.code.localeCompare(right.code));
}
