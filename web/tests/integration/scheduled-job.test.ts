import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { SQL } from 'bun';
import { isHttpError } from '@sveltejs/kit';
import { controlPlane, issueAgentKey, provisionCompany, revokeAgent } from '../../src/lib/server/control-plane';
import { callingScheduledJob } from '../../src/lib/server/scheduled-job-request';
import { localDatabaseURL } from '../../../supabase/scripts/local-database-url';
import { projectURL, serviceRoleKey } from './supabase-environment';

const environment = { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey };
const client = controlPlane({ projectURL, serviceRoleKey });
const stamp = Date.now();
let jobSecret = '';
let agentKey = '';
let agentID = '';

function requestCarrying(bearer: string): Request {
	const headers = bearer ? { Authorization: `Bearer ${bearer}` } : undefined;
	return new Request('https://app.example.test/api/agent/unclosed-shifts', { method: 'POST', headers });
}

async function refusalStatusOf(request: Request): Promise<number> {
	try {
		await callingScheduledJob(request, environment);
	} catch (refusal) {
		if (isHttpError(refusal)) return refusal.status;
		throw refusal;
	}
	return 200;
}

beforeAll(async () => {
	const database = new SQL(localDatabaseURL);
	const [row] = await database`select decrypted_secret from vault.decrypted_secrets where name = 'scheduled_job_secret'`;
	await database.close();
	jobSecret = row.decrypted_secret;

	const company = await provisionCompany(
		client,
		{ name: 'Scheduled', slug: `scheduled-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`scheduled-${stamp}-admin@example.test`
	);
	const issued = await issueAgentKey(client, company.companyID, 'day digest');
	agentKey = issued.apiKey;
	agentID = issued.agentID;
});

afterAll(async () => {
	await revokeAgent(client, agentID);
	await client.from('company').delete().eq('slug', `scheduled-${stamp}`);
});

describe('callingScheduledJob', () => {
	test('the secret the plane keeps for its jobs is let in with the service client', async () => {
		const answered = await callingScheduledJob(requestCarrying(jobSecret), environment);
		const { error } = await answered.from('company').select('id').limit(1);
		expect(error).toBeNull();
	});

	test('a company agent key is refused, since the jobs belong to no company', async () => {
		expect(await refusalStatusOf(requestCarrying(agentKey))).toBe(403);
	});

	test('a request with no secret is told it carries none', async () => {
		expect(await refusalStatusOf(requestCarrying(''))).toBe(401);
	});

	test('a wrong secret is refused', async () => {
		expect(await refusalStatusOf(requestCarrying(`${jobSecret}0`))).toBe(403);
	});
});
