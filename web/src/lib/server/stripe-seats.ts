import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';
import { askStripe } from './stripe-api';

export type SeatsOutcome = 'unsubscribed' | 'unchanged' | 'added' | 'removed';

const subscribedCompany = z.object({ stripe_subscription_id: z.string().nullable() });

const subscription = z.object({
	status: z.string(),
	items: z.object({ data: z.array(z.object({ id: z.string(), quantity: z.number().int() })).length(1) })
});

const subscriptionsThatBill = new Set(['active', 'past_due', 'trialing']);

export async function billedSeatsOf(client: SupabaseClient, companyID: string): Promise<number> {
	const { data, error } = await client.rpc('company_billed_seats', { target_company: companyID });
	if (error) throw new Error(`counting the billed seats of company ${companyID}: ${error.message}`);
	return z.number().int().parse(data);
}

export async function followSeats(client: SupabaseClient, secretKey: string, companyID: string): Promise<SeatsOutcome> {
	const subscriptionID = await subscriptionOf(client, companyID);
	if (!subscriptionID) return 'unsubscribed';

	const current = subscription.parse(await askStripe(secretKey, 'GET', `/subscriptions/${subscriptionID}`));
	if (!subscriptionsThatBill.has(current.status)) return 'unsubscribed';

	const [item] = current.items.data;
	const seats = await billedSeatsOf(client, companyID);
	if (seats === item.quantity) return 'unchanged';

	const added = seats > item.quantity;
	await askStripe(
		secretKey,
		'POST',
		`/subscriptions/${subscriptionID}`,
		new URLSearchParams({
			'items[0][id]': item.id,
			'items[0][quantity]': String(seats),
			proration_behavior: added ? 'always_invoice' : 'none'
		})
	);
	return added ? 'added' : 'removed';
}

async function subscriptionOf(client: SupabaseClient, companyID: string): Promise<string | null> {
	const { data, error } = await client.from('company').select('stripe_subscription_id').eq('id', companyID).maybeSingle();
	if (error) throw new Error(`reading the subscription of company ${companyID}: ${error.message}`);
	if (!data) return null;
	return subscribedCompany.parse(data).stripe_subscription_id;
}
