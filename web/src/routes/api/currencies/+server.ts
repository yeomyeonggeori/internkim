import { env } from '$env/dynamic/private';
import { exchangeRatesOf } from '$lib/server/configured-exchange-rates';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async () => {
	try {
		return json({ currencies: await exchangeRatesOf(env).supportedCurrencies() });
	} catch (cause) {
		error(502, cause instanceof Error ? cause.message : 'the exchange rate provider is unavailable');
	}
};
