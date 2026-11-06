import { z } from 'zod';
import type { UnitAmountOfPrice } from './stripe-billing-events';

const stripeAPI = 'https://api.stripe.com/v1';

const price = z.object({ unit_amount_decimal: z.string() });

export class StripeRefusal extends Error {
	constructor(
		message: string,
		readonly status: number,
		readonly body: unknown
	) {
		super(message);
		this.name = 'StripeRefusal';
	}
}

export async function askStripe(
	secretKey: string,
	method: 'GET' | 'POST',
	path: string,
	form?: URLSearchParams,
	idempotencyKey?: string
): Promise<unknown> {
	const headers: Record<string, string> = {
		authorization: `Bearer ${secretKey}`,
		'content-type': 'application/x-www-form-urlencoded'
	};
	if (idempotencyKey) headers['idempotency-key'] = idempotencyKey;
	const response = await fetch(`${stripeAPI}${path}`, { method, headers, body: form });
	const body: unknown = await response.json().catch(() => null);
	if (!response.ok) {
		throw new StripeRefusal(`Stripe refused ${method} ${path}: ${response.status} ${JSON.stringify(body)}`, response.status, body);
	}
	return body;
}

export function unitAmountOfStripePrice(secretKey: string): UnitAmountOfPrice {
	return async (priceID) => Number(price.parse(await askStripe(secretKey, 'GET', `/prices/${priceID}`)).unit_amount_decimal);
}
