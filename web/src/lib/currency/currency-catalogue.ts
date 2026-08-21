import { isSupabaseConfigured } from '$lib/supabase';

export type CurrencyCatalogueEntry = {
	code: string;
	name: string;
	minorUnitDigits: number;
};

export type CurrencyCatalogue = CurrencyCatalogueEntry[];

export const interimCurrencyCatalogue: CurrencyCatalogue = [
	{ code: 'KRW', name: 'Korean Won', minorUnitDigits: 0 },
	{ code: 'USD', name: 'US Dollar', minorUnitDigits: 2 },
	{ code: 'JPY', name: 'Japanese Yen', minorUnitDigits: 0 },
	{ code: 'EUR', name: 'Euro', minorUnitDigits: 2 }
];

export function findCurrencyCatalogueEntry(
	catalogue: CurrencyCatalogue,
	currency: string
): CurrencyCatalogueEntry | undefined {
	return catalogue.find((entry) => entry.code === currency);
}

const assumedMinorUnitDigits = 2;

function minorUnitDigitsOf(currency: string, catalogue: CurrencyCatalogue): number {
	return findCurrencyCatalogueEntry(catalogue, currency)?.minorUnitDigits ?? assumedMinorUnitDigits;
}

export function majorAmountOf(amountMinor: number, currency: string, catalogue: CurrencyCatalogue): number {
	return amountMinor / 10 ** minorUnitDigitsOf(currency, catalogue);
}

export function minorAmountOf(amountMajor: number, currency: string, catalogue: CurrencyCatalogue): number {
	return Math.round(amountMajor * 10 ** minorUnitDigitsOf(currency, catalogue));
}

const providerPatienceInMilliseconds = 5000;

export async function loadCurrencyCatalogue(): Promise<CurrencyCatalogue> {
	if (!isSupabaseConfigured()) return interimCurrencyCatalogue;
	try {
		const response = await fetch('/api/currencies', {
			signal: AbortSignal.timeout(providerPatienceInMilliseconds)
		});
		if (!response.ok) return interimCurrencyCatalogue;
		const payload = (await response.json()) as { currencies?: CurrencyCatalogue };
		const currencies = payload.currencies;
		return currencies && currencies.length > 0 ? currencies : interimCurrencyCatalogue;
	} catch {
		return interimCurrencyCatalogue;
	}
}
