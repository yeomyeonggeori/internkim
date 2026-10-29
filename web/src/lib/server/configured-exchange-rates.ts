import type { Environment } from '$lib/server/agent-request';
import { frankfurterProvider, frankfurterURL, type ExchangeRateProvider } from '$lib/server/exchange-rates';

const providersByBaseURL = new Map<string, ExchangeRateProvider>();

export function exchangeRatesOf(environment: Environment): ExchangeRateProvider {
	const baseURL = environment.FRANKFURTER_URL || frankfurterURL;
	const known = providersByBaseURL.get(baseURL);
	if (known) return known;
	const provider = frankfurterProvider({ baseURL });
	providersByBaseURL.set(baseURL, provider);
	return provider;
}
