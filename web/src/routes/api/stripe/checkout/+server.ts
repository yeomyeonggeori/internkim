import { environmentOfPlatform } from '$lib/server/agent-request';
import { adminCallerOf, asMember, controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import { signedInAccessTokenOf } from '$lib/server/member-request';
import { subscriptionCheckoutURL } from '$lib/server/stripe-checkout';
import { error, json } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, url }) => {
	const environment = environmentOfPlatform(platform?.env);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');
	const secretKey = environment.STRIPE_SECRET_KEY ?? '';
	const priceID = environment.STRIPE_PRICE_ID ?? '';
	if (!secretKey || !priceID) error(503, 'Stripe billing is not configured');

	const accessToken = await signedInAccessTokenOf(request, plane);
	const caller = await adminCallerOf(asMember(plane, accessToken));
	if (!caller) error(403, 'only an admin starts the subscription');

	const record = controlPlane(plane);
	const checkoutURL = await subscriptionCheckoutURL({
		secretKey,
		priceID,
		memberCount: await billedMemberCount(record, caller.companyID),
		companyID: caller.companyID,
		customerID: await stripeCustomerOf(record, caller.companyID),
		returnURL: url.origin
	});
	return json({ url: checkoutURL });
};

async function billedMemberCount(record: SupabaseClient, companyID: string): Promise<number> {
	const { count, error: countError } = await record
		.from('member')
		.select('id', { count: 'exact', head: true })
		.eq('company_id', companyID)
		.eq('kind', 'human')
		.not('status', 'in', '(departed,withdrawn)');
	if (countError) error(500, countError.message);
	return count ?? 0;
}

async function stripeCustomerOf(record: SupabaseClient, companyID: string): Promise<string | null> {
	const { data, error: readError } = await record
		.from('company')
		.select('stripe_customer_id')
		.eq('id', companyID)
		.single<{ stripe_customer_id: string | null }>();
	if (readError) error(500, readError.message);
	return data.stripe_customer_id;
}
