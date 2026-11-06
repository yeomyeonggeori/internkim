import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';
import { StripeEventRefused, type StripeEventOutcome, type StripeLookups, type UnitAmountOfPrice } from './stripe-event-contract';
import { applyRefundedCharge } from './stripe-refund-events';
import { companyMetadata, creditsPaidFor, paidInvoice } from './stripe-subscription-invoice';
import { applyFailedTopUp, applyPaidTopUp, isTopUpInvoice } from './stripe-top-up-events';

const periodOpeningReasons = new Set(['subscription_create', 'subscription_cycle']);

const namedCompany = z.object({ company_id: z.string() });

const invoiceNamingACompany = z.object({
	parent: z.object({ subscription_details: z.object({ metadata: namedCompany }) })
});

const subscriptionNamingACompany = z.object({ metadata: namedCompany });

const subscription = z.object({
	id: z.string(),
	customer: z.string(),
	status: z.string(),
	metadata: companyMetadata
});

const stripeEvent = z.object({
	id: z.string(),
	type: z.string(),
	created: z.number().int(),
	data: z.object({ object: z.unknown(), previous_attributes: z.unknown().optional() })
});

export async function applyStripeEvent(
	client: SupabaseClient,
	payload: unknown,
	lookups: StripeLookups
): Promise<StripeEventOutcome> {
	const event = stripeEvent.safeParse(payload);
	if (!event.success) throw new StripeEventRefused('the body is not a Stripe event');
	if (isTopUpInvoice(event.data.data.object)) return applyTopUpEvent(client, event.data);
	if (event.data.type === 'invoice.paid') return applyPaidInvoice(client, event.data.id, event.data.data.object, lookups.unitAmountOfPrice);
	if (event.data.type === 'charge.refunded') return applyRefundedCharge(client, event.data, lookups);
	if (event.data.type.startsWith('customer.subscription.')) {
		return applySubscriptionChange(client, event.data, event.data.data.object);
	}
	return 'ignored';
}

async function applyTopUpEvent(
	client: SupabaseClient,
	event: { id: string; type: string; data: { object: unknown } }
): Promise<StripeEventOutcome> {
	if (event.type === 'invoice.paid') {
		await applyPaidTopUp(client, event.id, event.data.object);
		return 'applied';
	}
	if (event.type === 'invoice.payment_failed') {
		await applyFailedTopUp(client, event.id, event.data.object);
		return 'applied';
	}
	return 'ignored';
}

async function applyPaidInvoice(
	client: SupabaseClient,
	eventID: string,
	object: unknown,
	unitAmountOfPrice: UnitAmountOfPrice
): Promise<StripeEventOutcome> {
	if (!invoiceNamingACompany.safeParse(object).success) return 'ignored';
	const invoice = paidInvoice.safeParse(object);
	if (!invoice.success) throw new StripeEventRefused(`paid invoice ${eventID}: ${z.prettifyError(invoice.error)}`);
	const details = invoice.data.parent.subscription_details;
	const { error } = await client.rpc('company_subscription_invoice_paid', {
		target_company: details.metadata.company_id,
		target_event: eventID,
		target_customer: invoice.data.customer,
		target_subscription: details.subscription,
		granted_credits: await creditsPaidFor(invoice.data.lines.data, unitAmountOfPrice),
		opens_period: periodOpeningReasons.has(invoice.data.billing_reason ?? ''),
		period_ends_at: new Date(latestPeriodEnd(invoice.data.lines.data) * 1000).toISOString()
	});
	if (error) throw new Error(`paid invoice ${eventID}: ${error.message}`);
	return 'applied';
}

async function applySubscriptionChange(
	client: SupabaseClient,
	event: { id: string; created: number },
	object: unknown
): Promise<StripeEventOutcome> {
	if (!subscriptionNamingACompany.safeParse(object).success) return 'ignored';
	const changed = subscription.safeParse(object);
	if (!changed.success) throw new StripeEventRefused(`subscription event ${event.id}: ${z.prettifyError(changed.error)}`);
	const { error } = await client.rpc('company_subscription_changed', {
		target_company: changed.data.metadata.company_id,
		target_customer: changed.data.customer,
		target_subscription: changed.data.id,
		target_status: changed.data.status,
		status_at: new Date(event.created * 1000).toISOString()
	});
	if (error) throw new Error(`subscription ${changed.data.id}: ${error.message}`);
	return 'applied';
}

function latestPeriodEnd(lines: { period: { end: number } }[]): number {
	return Math.max(...lines.map((line) => line.period.end));
}
