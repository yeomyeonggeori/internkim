import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';
import { creditsBoughtWith } from './stripe-top-up';
import type { TopUpRates } from './top-up-rates';

export const topUpInvoice = z.object({
	id: z.string(),
	currency: z.string(),
	amount_paid: z.number().int(),
	metadata: z.object({ top_up_id: z.guid(), company_id: z.guid() })
});

export function isTopUpInvoice(object: unknown): boolean {
	return topUpInvoice.safeParse(object).success;
}

export async function applyPaidTopUp(client: SupabaseClient, eventID: string, object: unknown, rates: TopUpRates): Promise<void> {
	const paid = topUpInvoice.parse(object);
	const credits = creditsBoughtWith(rates, paid.amount_paid, paid.currency);
	if (credits === null) throw new Error(`top-up invoice ${paid.id} was paid in ${paid.currency}, which no top-up product sells in`);
	const { error } = await client.rpc('company_credit_top_up_paid', {
		target_top_up: paid.metadata.top_up_id,
		target_event: eventID,
		target_invoice: paid.id,
		granted_credits: credits
	});
	if (error) throw new Error(`paid top-up invoice ${paid.id}: ${error.message}`);
}

export async function applyFailedTopUp(client: SupabaseClient, eventID: string, object: unknown): Promise<void> {
	const failed = topUpInvoice.parse(object);
	const { error } = await client.rpc('company_credit_top_up_failed', {
		target_top_up: failed.metadata.top_up_id,
		target_invoice: failed.id,
		reason: `Stripe event ${eventID}: the invoice payment failed`
	});
	if (error) throw new Error(`failed top-up invoice ${failed.id}: ${error.message}`);
}
