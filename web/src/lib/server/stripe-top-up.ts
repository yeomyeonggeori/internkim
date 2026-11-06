import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';
import { askStripe, StripeRefusal } from './stripe-api';

export type TopUpOutcome = 'settled' | 'charged' | 'failed';

type TopUpProduct = { product: string; creditsPerSmallestUnit: number };

const topUpProducts: Record<string, TopUpProduct> = {
	krw: { product: 'internkim_top_up_ko', creditsPerSmallestUnit: 0.4 },
	usd: { product: 'internkim_top_up_en', creditsPerSmallestUnit: 4 }
};

const topUpStatementDescriptor = 'INTERNKIM CREDITS';

const pendingTopUp = z.object({
	id: z.string(),
	company_id: z.string(),
	status: z.enum(['pending', 'paid', 'failed']),
	company: z.object({ stripe_customer_id: z.string().nullable(), stripe_subscription_id: z.string().nullable() })
});

const topUpSetting = z.object({ amount: z.number().int().positive() });

const subscription = z.object({ currency: z.string(), default_payment_method: z.string().nullable() });

const invoice = z.object({ id: z.string() });

const stripeError = z.object({ error: z.object({ code: z.string().optional(), decline_code: z.string().optional() }) });

export function creditsBoughtWith(amount: number, currency: string): number | null {
	const product = topUpProducts[currency];
	if (!product) return null;
	return Math.floor(amount * product.creditsPerSmallestUnit);
}

export async function chargeTopUp(client: SupabaseClient, secretKey: string, topUpID: string): Promise<TopUpOutcome> {
	const topUp = await topUpOf(client, topUpID);
	if (topUp.status !== 'pending') return 'settled';
	const { stripe_customer_id: customerID, stripe_subscription_id: subscriptionID } = topUp.company;
	if (!customerID || !subscriptionID) return recordFailure(client, topUpID, null, 'the company has no Stripe subscription');

	let invoiceID: string | null = null;
	let finalizedInvoiceID: string | null = null;
	try {
		const paidWith = subscription.parse(await askStripe(secretKey, 'GET', `/subscriptions/${subscriptionID}`));
		const product = topUpProducts[paidWith.currency];
		if (!product) return recordFailure(client, topUpID, null, `no top-up product sells in ${paidWith.currency}`);
		if (!paidWith.default_payment_method) return recordFailure(client, topUpID, null, 'the subscription has no saved card');

		const amount = await amountOf(client, topUp.company_id);
		invoiceID = await draftInvoice(secretKey, topUp, customerID, paidWith.currency, paidWith.default_payment_method);
		await askStripe(secretKey, 'POST', '/invoiceitems', new URLSearchParams({
			customer: customerID,
			invoice: invoiceID,
			'price_data[currency]': paidWith.currency,
			'price_data[product]': product.product,
			'price_data[unit_amount]': String(amount),
			'price_data[tax_behavior]': 'inclusive'
		}), `top-up-${topUpID}-item`);
		await askStripe(secretKey, 'POST', `/invoices/${invoiceID}/finalize`, new URLSearchParams(), `top-up-${topUpID}-finalize`);
		finalizedInvoiceID = invoiceID;
		await askStripe(secretKey, 'POST', `/invoices/${invoiceID}/pay`, new URLSearchParams({ off_session: 'true' }), `top-up-${topUpID}-pay`);
		return 'charged';
	} catch (failure) {
		await recordFailure(client, topUpID, invoiceID, reasonOf(failure));
		if (finalizedInvoiceID) await voidInvoice(secretKey, topUpID, finalizedInvoiceID);
		if (!(failure instanceof StripeRefusal)) throw failure;
		return 'failed';
	}
}

async function voidInvoice(secretKey: string, topUpID: string, invoiceID: string): Promise<void> {
	await askStripe(secretKey, 'POST', `/invoices/${invoiceID}/void`, new URLSearchParams(), `top-up-${topUpID}-void`);
}

async function draftInvoice(
	secretKey: string,
	topUp: z.infer<typeof pendingTopUp>,
	customerID: string,
	currency: string,
	paymentMethod: string
): Promise<string> {
	const drafted = await askStripe(secretKey, 'POST', '/invoices', new URLSearchParams({
		customer: customerID,
		currency,
		collection_method: 'charge_automatically',
		auto_advance: 'false',
		pending_invoice_items_behavior: 'exclude',
		default_payment_method: paymentMethod,
		statement_descriptor: topUpStatementDescriptor,
		'metadata[top_up_id]': topUp.id,
		'metadata[company_id]': topUp.company_id
	}), `top-up-${topUp.id}-invoice`);
	return invoice.parse(drafted).id;
}

async function topUpOf(client: SupabaseClient, topUpID: string): Promise<z.infer<typeof pendingTopUp>> {
	const { data, error } = await client
		.from('company_credit_top_up')
		.select('id, company_id, status, company(stripe_customer_id, stripe_subscription_id)')
		.eq('id', topUpID)
		.single();
	if (error) throw new Error(`reading top-up ${topUpID}: ${error.message}`);
	return pendingTopUp.parse(data);
}

async function amountOf(client: SupabaseClient, companyID: string): Promise<number> {
	const { data, error } = await client
		.from('company_credit_top_up_setting')
		.select('amount')
		.eq('company_id', companyID)
		.single();
	if (error) throw new Error(`reading the top-up setting of company ${companyID}: ${error.message}`);
	return topUpSetting.parse(data).amount;
}

async function recordFailure(client: SupabaseClient, topUpID: string, invoiceID: string | null, reason: string): Promise<TopUpOutcome> {
	const { error } = await client.rpc('company_credit_top_up_failed', {
		target_top_up: topUpID,
		target_invoice: invoiceID,
		reason
	});
	if (error) throw new Error(`recording why top-up ${topUpID} failed (${reason}): ${error.message}`);
	return 'failed';
}

function reasonOf(failure: unknown): string {
	if (!(failure instanceof StripeRefusal)) return failure instanceof Error ? failure.message : String(failure);
	const named = stripeError.safeParse(failure.body);
	if (!named.success) return `Stripe answered ${failure.status}`;
	return named.data.error.decline_code ?? named.data.error.code ?? `Stripe answered ${failure.status}`;
}
