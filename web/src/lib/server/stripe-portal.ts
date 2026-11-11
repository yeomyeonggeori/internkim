import { z } from 'zod';
import { askStripe } from './stripe-api';

const portalSession = z.object({ url: z.url() });

export async function billingPortalURL(secretKey: string, customerID: string, returnURL: string): Promise<string> {
	const form = new URLSearchParams({ customer: customerID, return_url: returnURL });
	return portalSession.parse(await askStripe(secretKey, 'POST', '/billing_portal/sessions', form)).url;
}
