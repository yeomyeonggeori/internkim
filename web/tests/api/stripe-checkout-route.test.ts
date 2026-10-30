import { describe, expect, mock, test } from 'bun:test';
import { isHttpError } from '@sveltejs/kit';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { POST: startCheckout } = await import('../../src/routes/api/stripe/checkout/+server');

const plane = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey
};

async function statusOfCheckout(environment: Record<string, string>, authorization: string): Promise<number> {
	const request = new Request('https://intern.example.test/api/stripe/checkout', {
		method: 'POST',
		headers: { authorization }
	});
	try {
		const response = await startCheckout({
			request,
			platform: { env: environment },
			url: new URL(request.url)
		} as unknown as Parameters<typeof startCheckout>[0]);
		return response.status;
	} catch (thrown) {
		if (isHttpError(thrown)) return thrown.status;
		throw thrown;
	}
}

describe('the Stripe checkout', () => {
	test('answers 503 while Stripe billing is not configured', async () => {
		expect(await statusOfCheckout(plane, 'Bearer anything')).toBe(503);
	});

	test('asks for a sign-in before it reaches Stripe', async () => {
		const configured = { ...plane, STRIPE_SECRET_KEY: 'sk_test_unused', STRIPE_PRICE_ID: 'price_unused' };
		expect(await statusOfCheckout(configured, '')).toBe(401);
	});
});
