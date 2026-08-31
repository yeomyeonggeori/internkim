import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, controlPlane, provisionCompany } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { GET: readTheFeed } = await import('../../src/routes/calendar/feed/[key].ics/+server');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `calendar-feed-${Date.now()}`;
const feedKey = `feed-key-${Date.now()}`;
const eventTitle = 'Quarter plan, first half';

let companyID = '';

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Calendar Feed Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	const memberID = await addMember(client, companyID, `${slug}-holder@example.test`);

	await client.from('credential').insert({
		member_id: memberID,
		kind: 'calendar_feed',
		name: '',
		external_id: feedKey,
		permission: 'read'
	});
	await client.from('task').insert({
		company_id: companyID,
		title: eventTitle,
		is_event: true,
		starts_at: '2027-04-05T01:00:00Z',
		ends_at: '2027-04-05T02:00:00Z'
	});
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

type FeedAnswer = { status: number; contentType: string; body: string };

async function reachTheFeed(key: string): Promise<FeedAnswer> {
	const url = new URL(`https://space.intern.kim/calendar/feed/${key}.ics`);
	try {
		const response = await readTheFeed({
			params: { key },
			url,
			platform: undefined
		} as unknown as Parameters<typeof readTheFeed>[0]);
		return {
			status: response.status,
			contentType: response.headers.get('content-type') ?? '',
			body: await response.text()
		};
	} catch (thrown) {
		const refusal = thrown as { status?: number };
		if (typeof refusal.status !== 'number') throw thrown;
		return { status: refusal.status, contentType: '', body: '' };
	}
}

describe('the calendar feed route', () => {
	test('a held key answers with the company calendar', async () => {
		const answer = await reachTheFeed(feedKey);
		expect(answer.status).toBe(200);
		expect(answer.contentType).toBe('text/calendar; charset=utf-8');
		expect(answer.body).toContain('BEGIN:VCALENDAR');
		expect(answer.body).toContain('X-WR-CALNAME:Calendar Feed Test');
		expect(answer.body).toContain('SUMMARY:Quarter plan\\, first half');
		expect(answer.body).toContain('DTSTART:20270405T010000Z');
	});

	test('a key nobody holds is refused rather than answered with an empty calendar', async () => {
		const answer = await reachTheFeed('a-key-nobody-was-given');
		expect(answer.status).toBe(404);
		expect(answer.body).toBe('');
	});
});
