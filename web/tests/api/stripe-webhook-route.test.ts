import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { isHttpError } from '@sveltejs/kit';
import { controlPlane, provisionCompany } from '../../src/lib/server/control-plane';
import { subscriptionCreditsPerMember } from '../../src/lib/server/stripe-billing-events';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { POST: receiveStripeEvent } = await import('../../src/routes/api/stripe/webhook/+server');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const stamp = Date.now();
const webhookSecret = `whsec_route_${stamp}`;
const environment = { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, STRIPE_WEBHOOK_SECRET: webhookSecret };
const encoder = new TextEncoder();
const unitAmount = 1900;
const periodEnd = 1_800_000_000;

let companyID = '';

async function signatureOf(payload: string, secret: string): Promise<string> {
	const timestamp = Math.floor(Date.now() / 1000);
	const key = await crypto.subtle.importKey('raw', encoder.encode(secret), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
	const signature = new Uint8Array(await crypto.subtle.sign('HMAC', key, encoder.encode(`${timestamp}.${payload}`)));
	return `t=${timestamp},v1=${Array.from(signature, (byte) => byte.toString(16).padStart(2, '0')).join('')}`;
}

async function deliver(event: Record<string, unknown>, secret: string = webhookSecret): Promise<Response> {
	const payload = JSON.stringify(event);
	const request = new Request('https://intern.example.test/api/stripe/webhook', {
		method: 'POST',
		headers: { 'stripe-signature': await signatureOf(payload, secret), 'content-type': 'application/json' },
		body: payload
	});
	return receiveStripeEvent({ request, platform: { env: environment } } as unknown as Parameters<typeof receiveStripeEvent>[0]);
}

async function statusOfDelivery(event: Record<string, unknown>, secret?: string): Promise<number> {
	try {
		return (await deliver(event, secret)).status;
	} catch (thrown) {
		if (isHttpError(thrown)) return thrown.status;
		throw thrown;
	}
}

function paidInvoice(eventID: string, billingReason: string, lineAmount: number, company: string = companyID) {
	return {
		id: eventID,
		type: 'invoice.paid',
		created: periodEnd,
		data: {
			object: {
				customer: `cus_route_${stamp}`,
				billing_reason: billingReason,
				parent: { subscription_details: { subscription: `sub_route_${stamp}`, metadata: { company_id: company } } },
				lines: { data: [{ amount: lineAmount, period: { end: periodEnd }, pricing: { unit_amount_decimal: String(unitAmount) } }] }
			}
		}
	};
}

async function subscriptionCreditsOfCompany(): Promise<number> {
	const { data, error } = await client
		.from('company')
		.select('subscription_credits')
		.eq('id', companyID)
		.single<{ subscription_credits: number }>();
	if (error) throw new Error(error.message);
	return data.subscription_credits;
}

beforeAll(async () => {
	const slug = `stripe-webhook-${stamp}`;
	const provisioned = await provisionCompany(
		client,
		{ name: 'Stripe webhook sample', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
}, networkHookTimeout);

afterAll(async () => {
	if (companyID) await client.from('company').delete().eq('id', companyID);
}, networkHookTimeout);

describe('the Stripe webhook', () => {
	test('refuses an event Stripe did not sign', async () => {
		expect(await statusOfDelivery(paidInvoice(`evt_forged_${stamp}`, 'subscription_create', 2 * unitAmount), 'whsec_someone_else')).toBe(400);
		expect(await subscriptionCreditsOfCompany()).toBe(0);
	});

	test('grants a paid period once, however often Stripe sends it', async () => {
		const event = paidInvoice(`evt_created_${stamp}`, 'subscription_create', 2 * unitAmount);
		const answer: unknown = await (await deliver(event)).json();
		expect(answer).toEqual({ outcome: 'applied' });
		await deliver(event);
		expect(await subscriptionCreditsOfCompany()).toBe(2 * subscriptionCreditsPerMember);
	});

	test('grants a seat added partway in proportion to what it paid', async () => {
		await deliver(paidInvoice(`evt_prorated_${stamp}`, 'subscription_update', unitAmount / 2));
		expect(await subscriptionCreditsOfCompany()).toBe(2.5 * subscriptionCreditsPerMember);
	});

	test('a renewal replaces what was left with the new period', async () => {
		await deliver(paidInvoice(`evt_renewed_${stamp}`, 'subscription_cycle', 3 * unitAmount));
		expect(await subscriptionCreditsOfCompany()).toBe(3 * subscriptionCreditsPerMember);
	});

	test('writes the subscription status Stripe reports', async () => {
		await deliver({
			id: `evt_status_${stamp}`,
			type: 'customer.subscription.updated',
			created: periodEnd,
			data: { object: { id: `sub_route_${stamp}`, customer: `cus_route_${stamp}`, status: 'past_due', metadata: { company_id: companyID } } }
		});
		const { data } = await client.from('company').select('subscription_status').eq('id', companyID).single<{ subscription_status: string }>();
		expect(data?.subscription_status).toBe('past_due');

		await deliver({
			id: `evt_status_late_${stamp}`,
			type: 'customer.subscription.created',
			created: periodEnd - 60,
			data: { object: { id: `sub_route_${stamp}`, customer: `cus_route_${stamp}`, status: 'incomplete', metadata: { company_id: companyID } } }
		});
		const { data: afterLate } = await client.from('company').select('subscription_status').eq('id', companyID).single<{ subscription_status: string }>();
		expect(afterLate?.subscription_status).toBe('past_due');
	});

	test('refuses an invoice that names a company but is not shaped as one of ours', async () => {
		const event = paidInvoice(`evt_malformed_${stamp}`, 'subscription_cycle', unitAmount, 'not-a-company');
		expect(await statusOfDelivery(event)).toBe(400);
	});

	test('ignores an event for another product that names no company', async () => {
		const response = await deliver({
			id: `evt_other_${stamp}`,
			type: 'invoice.paid',
			created: periodEnd,
			data: { object: { customer: 'cus_other', billing_reason: 'subscription_cycle', lines: { data: [] } } }
		});
		const answer: unknown = await response.json();
		expect(answer).toEqual({ outcome: 'ignored' });
	});
});
