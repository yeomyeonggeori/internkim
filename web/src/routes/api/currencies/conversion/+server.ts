import { convertedAmount } from '$lib/server/converted-amount';
import { env } from '$env/dynamic/private';
import { exchangeRatesOf } from '$lib/server/configured-exchange-rates';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const currencyCodePattern = /^[A-Z]{3}$/;

function requiredAmount(url: URL): number {
	const rawAmount = url.searchParams.get('amount');
	if (rawAmount === null) error(400, 'amount is required');
	const amount = Number(rawAmount);
	if (!Number.isFinite(amount) || amount < 0) error(400, 'amount must be a non-negative number');
	return amount;
}

function requiredCurrencyCode(url: URL, key: string): string {
	const currencyCode = url.searchParams.get(key) ?? '';
	if (!currencyCodePattern.test(currencyCode)) error(400, `${key} must be a three-letter uppercase currency code`);
	return currencyCode;
}

export const GET: RequestHandler = async ({ url }) => {
	const amount = requiredAmount(url);
	const from = requiredCurrencyCode(url, 'from');
	const to = requiredCurrencyCode(url, 'to');

	try {
		return json(await convertedAmount(exchangeRatesOf(env), { amount, currency: from }, to));
	} catch (cause) {
		error(502, cause instanceof Error ? cause.message : 'the exchange rate provider is unavailable');
	}
};
