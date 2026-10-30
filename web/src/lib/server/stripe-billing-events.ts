import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';

export const subscriptionCreditsPerMember = 10_000;

const periodOpeningReasons = new Set(['subscription_create', 'subscription_cycle']);

const companyMetadata = z.object({ company_id: z.guid() });

const namedCompany = z.object({ company_id: z.string() });

const invoiceNamingACompany = z.object({
	parent: z.object({ subscription_details: z.object({ metadata: namedCompany }) })
});

const subscriptionNamingACompany = z.object({ metadata: namedCompany });

const paidInvoice = z.object({
	customer: z.string(),
	billing_reason: z.string().nullable(),
	parent: z.object({
		subscription_details: z.object({ subscription: z.string(), metadata: companyMetadata })
	}),
	lines: z.object({
		data: z.array(
			z.object({
				amount: z.number().int(),
				period: z.object({ end: z.number().int() }),
				pricing: z.object({ unit_amount_decimal: z.string() }).nullable()
			})
		).min(1)
	})
});

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
	data: z.object({ object: z.unknown() })
});

export type StripeEventOutcome = 'applied' | 'ignored';

export class StripeEventRefused extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'StripeEventRefused';
	}
}

export async function applyStripeEvent(client: SupabaseClient, payload: unknown): Promise<StripeEventOutcome> {
	const event = stripeEvent.safeParse(payload);
	if (!event.success) throw new StripeEventRefused('the body is not a Stripe event');
	if (event.data.type === 'invoice.paid') return applyPaidInvoice(client, event.data.id, event.data.data.object);
	if (event.data.type.startsWith('customer.subscription.')) {
		return applySubscriptionChange(client, event.data, event.data.data.object);
	}
	return 'ignored';
}

async function applyPaidInvoice(client: SupabaseClient, eventID: string, object: unknown): Promise<StripeEventOutcome> {
	if (!invoiceNamingACompany.safeParse(object).success) return 'ignored';
	const invoice = paidInvoice.safeParse(object);
	if (!invoice.success) throw new StripeEventRefused(`paid invoice ${eventID}: ${z.prettifyError(invoice.error)}`);
	const details = invoice.data.parent.subscription_details;
	const { error } = await client.rpc('company_subscription_invoice_paid', {
		target_company: details.metadata.company_id,
		target_event: eventID,
		target_customer: invoice.data.customer,
		target_subscription: details.subscription,
		granted_credits: creditsPaidFor(invoice.data.lines.data),
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

type PaidLine = { amount: number; pricing: { unit_amount_decimal: string } | null };

function creditsPaidFor(lines: PaidLine[]): number {
	return lines.reduce((credits, line) => {
		const unitAmount = Number(line.pricing?.unit_amount_decimal ?? 0);
		if (line.amount <= 0 || unitAmount <= 0) return credits;
		return credits + Math.round((subscriptionCreditsPerMember * line.amount) / unitAmount);
	}, 0);
}

function latestPeriodEnd(lines: { period: { end: number } }[]): number {
	return Math.max(...lines.map((line) => line.period.end));
}
