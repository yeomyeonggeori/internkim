import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { invoicesOfCustomer } from '../../../src/lib/server/stripe-invoices';

type StripeCharge = { payment_intent: string | null; amount_refunded: number };

let realFetch: typeof globalThis.fetch;

beforeEach(() => {
	realFetch = globalThis.fetch;
});

afterEach(() => {
	globalThis.fetch = realFetch;
});

function paidInvoice(id: string, paymentIntent: string, amountPaid: number): Record<string, unknown> {
	return {
		id,
		created: 1794000000,
		status: 'paid',
		total: amountPaid,
		amount_paid: amountPaid,
		currency: 'krw',
		description: null,
		hosted_invoice_url: `https://invoice.stripe.example.test/${id}`,
		lines: { data: [{ description: '2 × InternKim seat' }] },
		payments: { data: [{ payment: { payment_intent: paymentIntent } }] }
	};
}

function stripeAnswers(invoices: unknown[], charges: StripeCharge[]): void {
	const answer = async (input: Parameters<typeof fetch>[0]): Promise<Response> => {
		const path = new URL(String(input)).pathname;
		return Response.json({ data: path.endsWith('/charges') ? charges : invoices });
	};
	globalThis.fetch = Object.assign(answer, { preconnect: realFetch.preconnect });
}

describe('the invoices a company is shown', () => {
	test('a payment that was declined and then went through still shows its refund', async () => {
		stripeAnswers(
			[paidInvoice('in_retried', 'pi_retried', 40000)],
			[
				{ payment_intent: 'pi_retried', amount_refunded: 40000 },
				{ payment_intent: 'pi_retried', amount_refunded: 0 }
			]
		);
		const [invoice] = await invoicesOfCustomer('sk_test_unused', 'cus_sample');
		expect(invoice.status).toBe('refunded');
	});

	test('a part of the payment given back is a partial refund, and none is a paid invoice', async () => {
		stripeAnswers(
			[paidInvoice('in_part', 'pi_part', 40000), paidInvoice('in_kept', 'pi_kept', 20000)],
			[
				{ payment_intent: 'pi_part', amount_refunded: 15000 },
				{ payment_intent: 'pi_kept', amount_refunded: 0 }
			]
		);
		const invoices = await invoicesOfCustomer('sk_test_unused', 'cus_sample');
		expect(invoices.map((invoice) => invoice.status)).toEqual(['partially_refunded', 'paid']);
		expect(invoices[0]).toMatchObject({ amount: 40000, currency: 'krw', description: '2 × InternKim seat' });
	});

	test('a draft is left out and an unpaid invoice asks for its whole total', async () => {
		const open = { ...paidInvoice('in_open', 'pi_open', 0), status: 'open', total: 30000 };
		const draft = { ...paidInvoice('in_draft', 'pi_draft', 0), status: 'draft', total: 30000 };
		stripeAnswers([open, draft], []);
		const invoices = await invoicesOfCustomer('sk_test_unused', 'cus_sample');
		expect(invoices).toHaveLength(1);
		expect(invoices[0]).toMatchObject({ id: 'in_open', status: 'open', amount: 30000 });
	});
});
