import { afterAll, describe, expect, mock, test } from 'bun:test';
import { isHttpError } from '@sveltejs/kit';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';
import { controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { POST: startCheckout } = await import('../../src/routes/api/stripe/checkout/+server');

const plane = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey
};

async function statusOfCheckout(environment: Record<string, string>, authorization: string): Promise<number> {
	const request = new Request('https://intern.example.test/api/stripe/checkout', {
		method: 'POST',
		headers: { authorization }
	});
	try {
		const response = await startCheckout({
			request,
			platform: { env: environment },
			url: new URL(request.url)
		} as unknown as Parameters<typeof startCheckout>[0]);
		return response.status;
	} catch (thrown) {
		if (isHttpError(thrown)) return thrown.status;
		throw thrown;
	}
}

describe('the Stripe checkout', () => {
	test('answers 503 while Stripe billing is not configured', async () => {
		expect(await statusOfCheckout(plane, 'Bearer anything')).toBe(503);
	});

	test('asks for a sign-in before it reaches Stripe', async () => {
		const configured = { ...plane, STRIPE_SECRET_KEY: 'sk_test_unused', STRIPE_PRICE_ID: 'price_unused' };
		expect(await statusOfCheckout(configured, '')).toBe(401);
	});

	test('refuses a second subscription while the company still holds one', async () => {
		const configured = { ...plane, STRIPE_SECRET_KEY: 'sk_test_unused', STRIPE_PRICE_ID: 'price_unused' };
		const { accessToken } = await adminOfACompanyWhoseCardWasDeclined();
		expect(await statusOfCheckout(configured, `Bearer ${accessToken}`)).toBe(409);
	});
});

const record = controlPlane({ projectURL, serviceRoleKey });
const provisionedCompanies: string[] = [];
const provisionedUsers: string[] = [];

async function adminOfACompanyWhoseCardWasDeclined(): Promise<{ accessToken: string }> {
	const slug = `checkout-open-${Date.now()}`;
	const email = `${slug}@example.test`;
	const { companyID, adminMemberID } = await provisionCompany(
		record,
		{ name: 'Checkout Sample', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		email
	);
	provisionedCompanies.push(companyID);
	const { data: account, error: accountError } = await record.auth.admin.createUser({ email, email_confirm: true });
	if (accountError || !account.user) throw new Error(`creating ${email}: ${accountError?.message}`);
	provisionedUsers.push(account.user.id);
	await record.from('member').update({ user_id: account.user.id, status: 'active' }).eq('id', adminMemberID);
	const { error } = await record.rpc('company_subscription_changed', {
		target_company: companyID,
		target_customer: `cus_${slug}`,
		target_subscription: `sub_${slug}`,
		target_status: 'past_due',
		status_at: new Date().toISOString()
	});
	if (error) throw new Error(error.message);
	const session = await sessionForMember({ projectURL, serviceRoleKey, signingKey }, adminMemberID);
	return { accessToken: session.accessToken };
}

afterAll(async () => {
	for (const companyID of provisionedCompanies) await record.from('company').delete().eq('id', companyID);
	for (const userID of provisionedUsers) await record.auth.admin.deleteUser(userID);
});
