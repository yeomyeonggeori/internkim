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
	BGN: 'лв',
	BRL: 'R$',
	CAD: 'C$',
	CHF: 'Fr.',
	CNY: '¥',
	CZK: 'Kč',
	DKK: 'kr',
	EUR: '€',
	GBP: '£',
	HKD: 'HK$',
	HUF: 'Ft',
	IDR: 'Rp',
	ILS: '₪',
	INR: '₹',
	ISK: 'kr',
	JPY: '¥',
	KRW: '₩',
	MXN: 'MX$',
	MYR: 'RM',
	NOK: 'kr',
	NZD: 'NZ$',
	PHP: '₱',
	PLN: 'zł',
	RON: 'lei',
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
