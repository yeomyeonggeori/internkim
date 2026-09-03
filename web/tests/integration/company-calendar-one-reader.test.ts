import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import {
	addMember,
	asMember,
	controlPlane,
	provisionCompany,
	sessionForMember
} from '../../src/lib/server/control-plane';
import { dayOffColor } from '../../src/lib/calendar/day-off-color';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

let sessionToken = '';

mock.module('$lib/supabase', () => ({
	supabase: () => ({ auth: { getSession: async () => ({ data: { session: { access_token: sessionToken } } }) } })
}));

const { runToolOverTheRecord } = await import('../../src/lib/server/public-api/record');
const { fallback: reachTheAPI } = await import('../../src/routes/api/v1/[...path]/+server');
const { calendarFeedForToken } = await import('../../src/lib/server/calendar/feed');
const { issueFeedToken } = await import('../../src/lib/server/calendar/feed-token');
const { companyCalendarEntries } = await import('../../src/lib/calendar/company-calendar');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const credentials = { projectURL, publishableKey, serviceRoleKey };
const slug = `one-calendar-reader-${Date.now()}`;
const now = new Date();
const seoulOffset = 9 * 60 * 60 * 1000;
const millisecondsInADay = 24 * 60 * 60 * 1000;

const timedEventTitle = '분기 검토';
const wholeDayEventTitle = '팀 워크숍';

let companyID = '';
let adminID = '';
let colleagueID = '';
let feedToken = '';
let caller: ReturnType<typeof asMember>;

function companyDay(instant: Date): string {
	return new Date(instant.getTime() + seoulOffset).toISOString().slice(0, 10);
}

function companyMidnight(instant: Date, daysLater: number): string {
	const day = new Date(`${companyDay(instant)}T00:00:00.000Z`);
	return new Date(day.getTime() + daysLater * millisecondsInADay - seoulOffset).toISOString();
}

function inDays(days: number): Date {
	return new Date(now.getTime() + days * millisecondsInADay);
}

async function seatMember(email: string, isAdmin: boolean): Promise<string> {
	const memberID = await addMember(client, companyID, email);
	const { data: account } = await client.auth.admin.createUser({ email, email_confirm: true });
	await client
		.from('member')
		.update({ user_id: account.user!.id, is_admin: isAdmin, status: 'active' })
		.eq('id', memberID);
	return memberID;
}

async function seedEvent(title: string, startsAt: string, endsAt: string, isWholeDay: boolean): Promise<void> {
	const { data, error } = await client
		.from('task')
		.insert({
			company_id: companyID,
			title,
			is_event: true,
			is_whole_day: isWholeDay,
			starts_at: startsAt,
			ends_at: endsAt,
			requester_id: adminID,
			status: 'planned'
		})
		.select('id')
		.single<{ id: string }>();
	if (error) throw new Error(error.message);
	await client.from('task_participant').insert({ task_id: data.id, member_id: adminID });
}

const originalFetch = globalThis.fetch;

function answeredByTheRoute(path: string, options: RequestInit): Promise<Response> {
	const request = new Request(`https://space.example.test${path}`, {
		...options,
		headers: { ...(options.headers as Record<string, string>) }
	});
	return Promise.resolve(
		reachTheAPI({
			request,
			url: new URL(request.url),
			params: { path: path.replace('/api/v1/', '') },
			platform: undefined
		} as unknown as Parameters<typeof reachTheAPI>[0])
	);
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'One Calendar Reader', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-founder@example.test`
	);
	companyID = provisioned.companyID;
	adminID = await seatMember(`${slug}-admin@example.test`, true);
	colleagueID = await seatMember(`${slug}-colleague@example.test`, false);
	await client.from('member').update({ name: '이샘플' }).eq('id', adminID);
	await client.from('member').update({ name: '박예시' }).eq('id', colleagueID);

	await seedEvent(
		timedEventTitle,
		new Date(inDays(1).getTime()).toISOString(),
		new Date(inDays(1).getTime() + 60 * 60 * 1000).toISOString(),
		false
	);
	await seedEvent(
		wholeDayEventTitle,
		companyMidnight(inDays(2), 0),
		companyMidnight(inDays(2), 1),
		true
	);

	const { error: leaveRefused } = await client.from('leave').insert({
		member_id: colleagueID,
		kind: '연차',
		is_paid: true,
		is_deducted: true,
		days: 1,
		status: 'approved',
		starts_at: companyMidnight(inDays(3), 0),
		ends_at: companyMidnight(inDays(3), 1)
	});
	if (leaveRefused) throw new Error(leaveRefused.message);

	const session = await sessionForMember({ projectURL, serviceRoleKey }, adminID);
	sessionToken = session.accessToken;
	caller = asMember({ projectURL, publishableKey }, session.accessToken);
	feedToken = await issueFeedToken(caller);

	globalThis.fetch = ((input: RequestInfo | URL, options: RequestInit = {}) => {
		if (typeof input === 'string' && input.startsWith('/api/v1/')) return answeredByTheRoute(input, options);
		return originalFetch(input, options);
	}) as typeof fetch;
}, networkHookTimeout);

afterAll(async () => {
	globalThis.fetch = originalFetch;
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

type AnsweredEntry = {
	eventID: string;
	title: string;
	isWholeDay: boolean;
	source: string;
	readOnly: boolean;
};

async function entriesTheToolAnswers(): Promise<AnsweredEntry[]> {
	const answered = await runToolOverTheRecord(
		caller,
		client,
		adminID,
		'event_list',
		{ startsAt: inDays(-1).toISOString(), endsAt: inDays(7).toISOString() },
		now
	);
	expect(answered.status).toBe(200);
	return (answered.body as { result: { events: AnsweredEntry[] } }).result.events;
}

function titlesOf(entries: { title: string }[]): string[] {
	return entries.map((entry) => entry.title).sort();
}

function leaveTitle(): string {
	return '박예시 · 휴가';
}

function expectedTitles(): string[] {
	return [timedEventTitle, wholeDayEventTitle, leaveTitle()].sort();
}

describe('what is on the company calendar', () => {
	test('reaches the agent through event_list, leave included and named as leave', async () => {
		const entries = await entriesTheToolAnswers();

		expect(titlesOf(entries)).toEqual(expectedTitles());
		const dayOff = entries.find((entry) => entry.title === leaveTitle());
		expect(dayOff).toMatchObject({ source: 'leave', readOnly: true, isWholeDay: true });
		expect(entries.find((entry) => entry.title === wholeDayEventTitle)).toMatchObject({
			source: 'event',
			readOnly: false,
			isWholeDay: true
		});
		expect(entries.find((entry) => entry.title === timedEventTitle)).toMatchObject({
			source: 'event',
			readOnly: false,
			isWholeDay: false
		});
	}, networkHookTimeout);

	test('reaches the app as the same three entries, the day off in the colour of a day off', async () => {
		const entries = await companyCalendarEntries(inDays(-1), inDays(7));

		expect(titlesOf(entries)).toEqual(expectedTitles());
		expect(entries.find((entry) => entry.title === leaveTitle())).toMatchObject({
			color: dayOffColor,
			readOnly: true,
			isAllDay: true,
			source: 'leave'
		});
		expect(entries.find((entry) => entry.title === wholeDayEventTitle)).toMatchObject({
			isAllDay: true,
			readOnly: false
		});
	}, networkHookTimeout);

	test('reaches a calendar app through the subscription feed as the same three entries', async () => {
		const feed = await calendarFeedForToken(credentials, feedToken, now);

		expect(feed).toContain(`SUMMARY:${timedEventTitle}`);
		expect(feed).toContain(`SUMMARY:${wholeDayEventTitle}`);
		expect(feed).toContain(`SUMMARY:${leaveTitle()}`);
		expect(feed).toContain(`DTSTART;VALUE=DATE:${companyDay(inDays(3)).replace(/-/g, '')}`);
		expect(feed).toContain(`DTSTART;VALUE=DATE:${companyDay(inDays(2)).replace(/-/g, '')}`);
	}, networkHookTimeout);

	test('is one set of entries, whichever surface is asked', async () => {
		const entries = await entriesTheToolAnswers();
		const feed = (await calendarFeedForToken(credentials, feedToken, now)) ?? '';
		const subscribed = feed
			.split('\r\n')
			.filter((line) => line.startsWith('UID:'))
			.map((line) => line.slice('UID:'.length))
			.sort();

		expect(subscribed).toEqual(entries.map((entry) => entry.eventID).sort());
	}, networkHookTimeout);
});
