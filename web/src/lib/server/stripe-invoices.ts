import { z } from 'zod';
import { askStripe } from './stripe-api';
import type { CompanyInvoice } from '$lib/company/invoice-contract';

const invoicesShown = 24;

const stripeInvoice = z.object({
	id: z.string(),
	created: z.number().int(),
	status: z.string().nullable(),
	total: z.number().int(),
	amount_paid: z.number().int(),
	currency: z.string(),
	description: z.string().nullable().optional(),
	hosted_invoice_url: z.string().nullable().optional(),
	lines: z.object({ data: z.array(z.object({ description: z.string().nullable().optional() })) }),
	payments: z
		.object({ data: z.array(z.object({ payment: z.object({ payment_intent: z.string().nullable().optional() }) })) })
		.optional()
});

const stripeInvoices = z.object({ data: z.array(stripeInvoice) });

const stripeCharges = z.object({
	data: z.array(z.object({ payment_intent: z.string().nullable(), amount_refunded: z.number().int() }))
});

export async function invoicesOfCustomer(secretKey: string, customerID: string): Promise<CompanyInvoice[]> {
	const invoiceQuery = new URLSearchParams({ customer: customerID, limit: String(invoicesShown), 'expand[]': 'data.payments' });
	const chargeQuery = new URLSearchParams({ customer: customerID, limit: '100' });
	const [invoices, charges] = await Promise.all([
		askStripe(secretKey, 'GET', `/invoices?${invoiceQuery}`).then((body) => stripeInvoices.parse(body)),
		askStripe(secretKey, 'GET', `/charges?${chargeQuery}`).then((body) => stripeCharges.parse(body))
	]);
	const refundedByPayment = new Map<string, number>();
	for (const charge of charges.data) {
		if (!charge.payment_intent) continue;
		refundedByPayment.set(charge.payment_intent, (refundedByPayment.get(charge.payment_intent) ?? 0) + charge.amount_refunded);
	}
	return invoices.data
		.filter((invoice) => invoice.status !== 'draft')
		.map((invoice) => companyInvoiceOf(invoice, refundedByPayment));
}

function companyInvoiceOf(invoice: z.infer<typeof stripeInvoice>, refundedByPayment: Map<string, number>): CompanyInvoice {
	const refunded = (invoice.payments?.data ?? []).reduce(
		(sum, { payment }) => sum + (payment.payment_intent ? (refundedByPayment.get(payment.payment_intent) ?? 0) : 0),
		0
	);
	return {
		id: invoice.id,
		issuedAt: new Date(invoice.created * 1000).toISOString(),
		description: invoice.description || invoice.lines.data[0]?.description || '',
		amount: invoice.status === 'paid' ? invoice.amount_paid : invoice.total,
		currency: invoice.currency,
		status: statusOf(invoice.status, refunded, invoice.amount_paid),
		receiptURL: invoice.hosted_invoice_url ?? null
	};
}

function statusOf(stripeStatus: string | null, refunded: number, paid: number): CompanyInvoice['status'] {
	if (stripeStatus === 'paid' && refunded > 0) return refunded >= paid ? 'refunded' : 'partially_refunded';
	if (stripeStatus === 'paid') return 'paid';
	if (stripeStatus === 'void' || stripeStatus === 'uncollectible') return 'void';
	return 'open';
}
