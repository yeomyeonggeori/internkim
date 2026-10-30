import { z } from 'zod';

const stripeAPI = 'https://api.stripe.com/v1';

export type SubscriptionCheckout = {
	secretKey: string;
	priceID: string;
	memberCount: number;
	companyID: string;
	customerID: string | null;
	returnURL: string;
};

const checkoutSession = z.object({ url: z.url() });

export async function subscriptionCheckoutURL(checkout: SubscriptionCheckout): Promise<string> {
	const form = new URLSearchParams({
		mode: 'subscription',
		'line_items[0][price]': checkout.priceID,
		'line_items[0][quantity]': String(checkout.memberCount),
		client_reference_id: checkout.companyID,
		'subscription_data[metadata][company_id]': checkout.companyID,
		success_url: `${checkout.returnURL}?subscription=started`,
		cancel_url: `${checkout.returnURL}?subscription=cancelled`
	});
	if (checkout.customerID) form.set('customer', checkout.customerID);

	const response = await fetch(`${stripeAPI}/checkout/sessions`, {
		method: 'POST',
		headers: {
			authorization: `Bearer ${checkout.secretKey}`,
			'content-type': 'application/x-www-form-urlencoded'
		},
		body: form
	});
	const body: unknown = await response.json().catch(() => null);
	if (!response.ok) {
		throw new Error(`Stripe refused the checkout for company ${checkout.companyID}: ${response.status} ${JSON.stringify(body)}`);
	}
	const session = checkoutSession.safeParse(body);
	if (!session.success) throw new Error(`Stripe answered a checkout without a URL for company ${checkout.companyID}`);
	return session.data.url;
}
