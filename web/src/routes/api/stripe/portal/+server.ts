import { billingAdminOf } from '$lib/server/stripe-billing-admin';
import { billingPortalURL } from '$lib/server/stripe-portal';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, url }) => {
	const admin = await billingAdminOf(request, platform?.env, 'only an admin manages the payment');
	if (!admin.customerID) error(409, 'the company has never paid through Stripe; start the subscription first');
	const returnURL = `${url.origin}/${admin.companySlug}/settings`;
	return json({ url: await billingPortalURL(admin.secretKey, admin.customerID, returnURL) });
};
