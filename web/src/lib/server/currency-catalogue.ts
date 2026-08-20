import type { SupportedCurrency } from '$lib/server/exchange-rates';

export type CurrencyCatalogueEntry = {
	code: string;
	name: string;
	symbol: string;
	minorUnitDigits: number;
	grouping: 'myriad' | 'thousand';
};

const myriadGroupedCurrencies = new Set(['KRW', 'JPY', 'CNY', 'TWD', 'HKD', 'MOP']);

const currencySymbols: Readonly<Record<string, string>> = {
	AUD: 'A$',
	BRL: 'R$',
	CAD: 'C$',
	CHF: 'CHF',
	CNY: '¥',
	EUR: '€',
	GBP: '£',
	HKD: 'HK$',
	INR: '₹',
	JPY: '¥',
	KRW: '₩',
	MXN: 'MX$',
	NZD: 'NZ$',
	PHP: '₱',
	PLN: 'zł',
	SEK: 'kr',
	SGD: 'S$',
	THB: '฿',
	TRY: '₺',
	USD: '$',
	ZAR: 'R'
};

export function currencyCatalogueOf(currencies: SupportedCurrency[]): CurrencyCatalogueEntry[] {
	return currencies
		.map((currency) => ({
			code: currency.code,
			name: currency.name,
			symbol: currencySymbols[currency.code] ?? currency.code,
			minorUnitDigits: currency.minorUnitDigits,
			grouping: myriadGroupedCurrencies.has(currency.code) ? ('myriad' as const) : ('thousand' as const)
		}))
		.sort((left, right) => left.code.localeCompare(right.code));
}
