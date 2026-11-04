import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { SQL } from 'bun';
import { isHttpError } from '@sveltejs/kit';
import { localDatabaseURL } from '../../../supabase/scripts/local-database-url';
import { controlPlane } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { POST: followSeats } = await import('../../src/routes/api/stripe/seats/+server');

const plane = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey
};
const configured = { ...plane, STRIPE_SECRET_KEY: 'sk_test_unused' };

const service = controlPlane({ projectURL, serviceRoleKey });
const unsubscribedCompany = '56800000-0000-0000-0000-000000000000';
let jobSecret = '';

beforeAll(async () => {
	const database = new SQL(localDatabaseURL);
	const [row] = await database`select decrypted_secret from vault.decrypted_secrets where name = 'scheduled_job_secret'`;
	await database.close();
	jobSecret = row.decrypted_secret;
	const inserted = await service.from('company').insert({
		id: unsubscribedCompany,
		name: 'Seatless Sample',
		slug: 'seatless-sample',
		country: 'KR',
		locale: 'ko',
		timezone: 'Asia/Seoul',
		currency_code: 'KRW'
	});
	if (inserted.error) throw new Error(`founding the sample company: ${inserted.error.message}`);
});

afterAll(async () => {
	await service.from('company').delete().eq('id', unsubscribedCompany);
});

async function answerOfSeats(
	environment: Record<string, string>,
	authorization: string,
	body: unknown
): Promise<{ status: number; outcome?: string }> {
	const request = new Request('https://intern.example.test/api/stripe/seats', {
		method: 'POST',
		headers: { authorization, 'content-type': 'application/json' },
		body: JSON.stringify(body)
	});
	try {
		const response = await followSeats({
			request,
			platform: { env: environment },
			url: new URL(request.url)
		} as unknown as Parameters<typeof followSeats>[0]);
		const answer: { outcome?: string } = await response.json();
		return { status: response.status, outcome: answer.outcome };
	} catch (thrown) {
		if (isHttpError(thrown)) return { status: thrown.status };
		throw thrown;
	}
}

describe('the seats a subscription follows', () => {
	test('answer 503 while Stripe billing is not configured', async () => {
		expect((await answerOfSeats(plane, `Bearer ${jobSecret}`, { company_id: unsubscribedCompany })).status).toBe(503);
	});

	test('are asked for only by the scheduled job secret', async () => {
		expect((await answerOfSeats(configured, '', { company_id: unsubscribedCompany })).status).toBe(401);
		expect((await answerOfSeats(configured, 'Bearer not-the-secret', { company_id: unsubscribedCompany })).status).toBe(403);
	});

	test('refuse an ask that names no company', async () => {
		expect((await answerOfSeats(configured, `Bearer ${jobSecret}`, { company_id: 'nobody' })).status).toBe(400);
	});

	test('leave a company without a subscription alone, before reaching Stripe', async () => {
		expect(await answerOfSeats(configured, `Bearer ${jobSecret}`, { company_id: unsubscribedCompany })).toEqual({
			status: 200,
			outcome: 'unsubscribed'
		});
	});
});
