import { frankfurterProvider, type ExchangeRateProvider } from '$lib/server/exchange-rates';
import { currencyCatalogueOf } from '$lib/server/currency-catalogue';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const provider: ExchangeRateProvider = frankfurterProvider();

export const GET: RequestHandler = async () => {
	try {
		return json({ currencies: currencyCatalogueOf(await provider.supportedCurrencies()) });
	} catch (cause) {
		error(502, cause instanceof Error ? cause.message : 'the exchange rate provider is unavailable');
	}
};
