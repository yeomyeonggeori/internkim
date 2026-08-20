import { isSupabaseConfigured } from '$lib/supabase';

export type CurrencyGrouping = 'myriad' | 'thousand';

export type CurrencyCatalogueEntry = {
	code: string;
	name: string;
	symbol: string;
	minorUnitDigits: number;
	grouping: CurrencyGrouping;
};

export type CurrencyCatalogue = CurrencyCatalogueEntry[];

export const interimCurrencyCatalogue: CurrencyCatalogue = [
	{ code: 'KRW', name: 'Korean Won', symbol: '₩', minorUnitDigits: 0, grouping: 'myriad' },
	{ code: 'USD', name: 'US Dollar', symbol: '$', minorUnitDigits: 2, grouping: 'thousand' },
	{ code: 'JPY', name: 'Japanese Yen', symbol: '¥', minorUnitDigits: 0, grouping: 'myriad' },
	{ code: 'EUR', name: 'Euro', symbol: '€', minorUnitDigits: 2, grouping: 'thousand' }
];

export function findCurrencyCatalogueEntry(
	catalogue: CurrencyCatalogue,
	currency: string
): CurrencyCatalogueEntry | undefined {
	return catalogue.find((entry) => entry.code === currency);
}

export function majorAmountOf(
	amountMinor: number,
	currency: string,
	catalogue: CurrencyCatalogue
): number {
	const entry = findCurrencyCatalogueEntry(catalogue, currency);
	return amountMinor / 10 ** (entry?.minorUnitDigits ?? 2);
}

export async function loadCurrencyCatalogue(): Promise<CurrencyCatalogue> {
	if (!isSupabaseConfigured()) return interimCurrencyCatalogue;
	const response = await fetch('/api/currencies');
	if (!response.ok) return interimCurrencyCatalogue;
	const payload = (await response.json()) as { currencies?: CurrencyCatalogue };
	const currencies = payload.currencies;
	return currencies && currencies.length > 0 ? currencies : interimCurrencyCatalogue;
}
