import { environmentOfPlatform } from '$lib/server/agent-request';
import { adminCallerOf, asMember, controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import { signedInAccessTokenOf } from '$lib/server/member-request';
import { subscriptionCheckoutURL } from '$lib/server/stripe-checkout';
import { billedSeatsOf } from '$lib/server/stripe-seats';
import { holdsAnOpenSubscription } from '$lib/company/credits-contract';
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
	const company = await billingOf(record, caller.companyID);
	if (holdsAnOpenSubscription(company.subscription_status)) {
		error(409, 'the company already holds a subscription; settle its payment instead of starting another');
	}
	const checkoutURL = await subscriptionCheckoutURL({
		secretKey,
		priceID,
		memberCount: await billedSeatsOf(record, caller.companyID),
		companyID: caller.companyID,
		customerID: company.stripe_customer_id,
		returnURL: url.origin
	});
	return json({ url: checkoutURL });
};

async function billingOf(
	record: SupabaseClient,
	companyID: string
): Promise<{ stripe_customer_id: string | null; subscription_status: string | null }> {
	const { data, error: readError } = await record
		.from('company')
		.select('stripe_customer_id, subscription_status')
		.eq('id', companyID)
		.single<{ stripe_customer_id: string | null; subscription_status: string | null }>();
	if (readError) error(500, readError.message);
	return data;
}
