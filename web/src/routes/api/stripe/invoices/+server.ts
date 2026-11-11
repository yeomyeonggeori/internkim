import { billingAdminOf } from '$lib/server/stripe-billing-admin';
import { invoicesOfCustomer } from '$lib/server/stripe-invoices';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform }) => {
	const admin = await billingAdminOf(request, platform?.env, 'only an admin reads the invoices');
	if (!admin.customerID) return json({ invoices: [] });
	return json({ invoices: await invoicesOfCustomer(admin.secretKey, admin.customerID) });
};
