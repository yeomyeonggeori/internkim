import { convertedAmount } from '$lib/server/converted-amount';
import { frankfurterProvider, type ExchangeRateProvider } from '$lib/server/exchange-rates';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const provider: ExchangeRateProvider = frankfurterProvider();
const currencyCodePattern = /^[A-Z]{3}$/;

function requireAccessToken(request: Request): void {
	const authorization = request.headers.get('authorization') ?? '';
	const accessToken = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!accessToken) error(401, 'sign in first');
}

function requiredAmountMinor(url: URL): number {
	const rawAmountMinor = url.searchParams.get('amountMinor');
	if (rawAmountMinor === null) error(400, 'amountMinor is required');
	const amountMinor = Number(rawAmountMinor);
	if (!Number.isSafeInteger(amountMinor) || amountMinor < 0) error(400, 'amountMinor must be a non-negative safe integer');
	return amountMinor;
}

function requiredCurrencyCode(url: URL, key: string): string {
	const currencyCode = url.searchParams.get(key) ?? '';
	if (!currencyCodePattern.test(currencyCode)) error(400, `${key} must be a three-letter uppercase currency code`);
	return currencyCode;
}

export const GET: RequestHandler = async ({ request, url }) => {
	requireAccessToken(request);
	const amountMinor = requiredAmountMinor(url);
	const from = requiredCurrencyCode(url, 'from');
	const to = requiredCurrencyCode(url, 'to');

	try {
		return json(await convertedAmount(provider, amountMinor, from, to));
	} catch (cause) {
		error(502, cause instanceof Error ? cause.message : 'the exchange rate provider is unavailable');
	}
};
