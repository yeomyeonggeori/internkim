import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { accountOfAddress, addMember, controlPlane, provisionCompany } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: {
		SUPABASE_URL: projectURL,
		SUPABASE_SECRET_KEY: serviceRoleKey,
		SUPABASE_PUBLISHABLE_KEY: publishableKey,
		SUPABASE_JWT_SIGNING_KEY: signingKey,
		AUTH_CLAIM_WITHOUT_EMAIL: '1'
	}
}));

const { POST: claim } = await import('../../src/routes/api/auth/claim/+server');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `claim-mailbox-${Date.now()}`;
const unclaimedEmail = `${slug}-unclaimed@example.test`;
let companyID = '';

beforeAll(async () => {
	companyID = (
		await provisionCompany(
			client,
			{ name: 'Claim Mailbox', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
			`${slug}-admin@example.test`
		)
	).companyID;
	await addMember(client, companyID, unclaimedEmail);
}, networkHookTimeout);

afterAll(async () => {
	const account = await accountOfAddress(client, unclaimedEmail);
	if (account) await client.auth.admin.deleteUser(account.id);
	if (companyID) await client.from('company').delete().eq('id', companyID);
}, networkHookTimeout);

describe('claiming an address', () => {
	test('hands nobody a password, whatever the deployment says about mail', async () => {
		const request = new Request('https://intern.example.test/api/auth/claim', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email: unclaimedEmail })
		});
		const event = { request, url: new URL(request.url), platform: undefined };

		let answered: unknown = null;
		try {
			answered = await (await claim(event as unknown as Parameters<typeof claim>[0])).json();
		} catch (thrown) {
			const refusal = thrown as { status?: number };
			if (refusal.status !== 429) throw thrown;
		}

		expect(JSON.stringify(answered ?? {})).not.toContain('password');
		const account = await accountOfAddress(client, unclaimedEmail);
		expect(account?.email_confirmed_at ?? null).toBeNull();
	});
});
