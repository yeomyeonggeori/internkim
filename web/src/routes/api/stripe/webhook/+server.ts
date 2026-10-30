import { environmentOfPlatform } from '$lib/server/agent-request';
import { controlPlane } from '$lib/server/control-plane';
import { applyStripeEvent, StripeEventRefused } from '$lib/server/stripe-billing-events';
import { isSignedByStripe } from '$lib/server/stripe-signature';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOfPlatform(platform?.env);
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? '';
	const signingSecret = environment.STRIPE_WEBHOOK_SECRET ?? '';
	if (!projectURL || !serviceRoleKey || !signingSecret) error(503, 'Stripe billing is not configured');

	const payload = await request.text();
	const signed = await isSignedByStripe(payload, request.headers.get('stripe-signature') ?? '', signingSecret);
	if (!signed) error(400, 'the Stripe signature does not match');

	try {
		const outcome = await applyStripeEvent(controlPlane({ projectURL, serviceRoleKey }), JSON.parse(payload));
		return json({ outcome });
	} catch (caughtError) {
		if (caughtError instanceof StripeEventRefused || caughtError instanceof SyntaxError) error(400, caughtError.message);
		throw caughtError;
	}
};
