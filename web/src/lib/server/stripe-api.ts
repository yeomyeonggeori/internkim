import { z } from 'zod';
import type { UnitAmountOfPrice } from './stripe-billing-events';

const stripeAPI = 'https://api.stripe.com/v1';

const price = z.object({ unit_amount_decimal: z.string() });

export async function askStripe(secretKey: string, method: 'GET' | 'POST', path: string, form?: URLSearchParams): Promise<unknown> {
	const response = await fetch(`${stripeAPI}${path}`, {
		method,
		headers: {
			authorization: `Bearer ${secretKey}`,
			'content-type': 'application/x-www-form-urlencoded'
		},
		body: form
	});
	const body: unknown = await response.json().catch(() => null);
	if (!response.ok) throw new Error(`Stripe refused ${method} ${path}: ${response.status} ${JSON.stringify(body)}`);
	return body;
}

export function unitAmountOfStripePrice(secretKey: string): UnitAmountOfPrice {
	return async (priceID) => Number(price.parse(await askStripe(secretKey, 'GET', `/prices/${priceID}`)).unit_amount_decimal);
}
