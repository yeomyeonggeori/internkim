import { environmentOfPlatform } from '$lib/server/agent-request';
import { controlPlane } from '$lib/server/control-plane';
import { stripeLookups } from '$lib/server/stripe-api';
import { applyStripeEvent } from '$lib/server/stripe-billing-events';
import { StripeEventRefused } from '$lib/server/stripe-event-contract';
import { isSignedByStripe } from '$lib/server/stripe-signature';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOfPlatform(platform?.env);
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? '';
	const signingSecret = environment.STRIPE_WEBHOOK_SECRET ?? '';
	const secretKey = environment.STRIPE_SECRET_KEY ?? '';
	if (!projectURL || !serviceRoleKey || !signingSecret || !secretKey) error(503, 'Stripe billing is not configured');

	const payload = await request.text();
	const signed = await isSignedByStripe(payload, request.headers.get('stripe-signature') ?? '', signingSecret);
	if (!signed) error(400, 'the Stripe signature does not match');

	try {
		const outcome = await applyStripeEvent(
			controlPlane({ projectURL, serviceRoleKey }),
			JSON.parse(payload),
			stripeLookups(secretKey)
		);
		return json({ outcome });
	} catch (caughtError) {
		if (caughtError instanceof StripeEventRefused || caughtError instanceof SyntaxError) error(400, caughtError.message);
		throw caughtError;
	}
};
