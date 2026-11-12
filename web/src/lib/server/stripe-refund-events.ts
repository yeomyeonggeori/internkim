import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';
import { type StripeEventOutcome, StripeEventRefused, type StripeLookups } from './stripe-event-contract';
import { creditsPaidFor, paidInvoice } from './stripe-subscription-invoice';
import { creditsBoughtWith } from './stripe-top-up';
import { topUpInvoice } from './stripe-top-up-events';
import type { TopUpRates } from './top-up-rates';

type CreditKind = 'subscription' | 'permanent';

type CreditsTakenBack = { companyID: string; kind: CreditKind; credits: number };

const refundedCharge = z.object({
	id: z.string(),
	currency: z.string(),
	amount_refunded: z.number().int(),
	payment_intent: z.string().nullable()
});

const earlierRefund = z.object({ amount_refunded: z.number().int() });

const refundedSubscriptionInvoice = paidInvoice.extend({ amount_paid: z.number().int().positive() });

export async function applyRefundedCharge(
	client: SupabaseClient,
	event: { id: string; data: { object: unknown; previous_attributes?: unknown } },
	lookups: StripeLookups,
	topUpRates: TopUpRates
): Promise<StripeEventOutcome> {
	const charge = refundedCharge.safeParse(event.data.object);
	if (!charge.success) throw new StripeEventRefused(`refunded charge ${event.id}: ${z.prettifyError(charge.error)}`);
	const refundedBefore = earlierRefund.safeParse(event.data.previous_attributes);
	const refundedNow = charge.data.amount_refunded - (refundedBefore.success ? refundedBefore.data.amount_refunded : 0);
	if (refundedNow <= 0 || !charge.data.payment_intent) return 'ignored';

	const invoice = await lookups.invoicePaidBy(charge.data.payment_intent);
	const takenBack = await creditsTakenBackFor(invoice, refundedNow, charge.data.currency, lookups, topUpRates);
	if (!takenBack) return 'ignored';

	const { error } = await client.rpc('company_credits_refunded', {
		target_company: takenBack.companyID,
		target_event: event.id,
		refunded_kind: takenBack.kind,
		refunded_credits: takenBack.credits
	});
	if (error) throw new Error(`refunded charge ${charge.data.id}: ${error.message}`);
	return 'applied';
}

async function creditsTakenBackFor(
	invoice: unknown,
	refundedAmount: number,
	currency: string,
	lookups: StripeLookups,
	topUpRates: TopUpRates
): Promise<CreditsTakenBack | null> {
	const topUp = topUpInvoice.safeParse(invoice);
	if (topUp.success) {
		const credits = creditsBoughtWith(topUpRates, refundedAmount, currency);
		if (credits === null) throw new Error(`top-up invoice ${topUp.data.id} was refunded in ${currency}, which no top-up product sells in`);
		return { companyID: topUp.data.metadata.company_id, kind: 'permanent', credits };
	}
	const subscription = refundedSubscriptionInvoice.safeParse(invoice);
	if (!subscription.success) return null;
	const granted = await creditsPaidFor(subscription.data.lines.data, lookups.unitAmountOfPrice);
	return {
		companyID: subscription.data.parent.subscription_details.metadata.company_id,
		kind: 'subscription',
		credits: Math.round((granted * Math.min(refundedAmount, subscription.data.amount_paid)) / subscription.data.amount_paid)
	};
}
