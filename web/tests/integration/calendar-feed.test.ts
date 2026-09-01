import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import {
	addMember,
	asMember,
	controlPlane,
	provisionCompany,
	sessionForMember
} from '../../src/lib/server/control-plane';
import { calendarFeedForToken } from '../../src/lib/server/calendar/feed';
import { forgetFeedToken, issueFeedToken } from '../../src/lib/server/calendar/feed-token';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';

const networkHookTimeout = 60_000;
const credentials = { projectURL, publishableKey, serviceRoleKey };
const record = controlPlane({ projectURL, serviceRoleKey });
const stamp = Date.now();
const now = new Date();

type Company = {
	companyID: string;
	adminID: string;
	colleagueID: string;
	token: string;
};

let mine: Company;
let theirs: Company;

async function clientForMember(memberID: string) {
	const session = await sessionForMember({ projectURL, serviceRoleKey }, memberID);
	return asMember({ projectURL, publishableKey }, session.accessToken);
}

async function seatMember(companyID: string, email: string, isAdmin: boolean): Promise<string> {
	const memberID = await addMember(record, companyID, email);
	const { data: account } = await record.auth.admin.createUser({ email, email_confirm: true });
	await record
		.from('member')
		.update({ user_id: account.user!.id, is_admin: isAdmin, status: 'active' })
		.eq('id', memberID);
	return memberID;
}

const seoulOffset = 9 * 60 * 60 * 1000;

function companyDay(instant: Date): string {
	return new Date(instant.getTime() + seoulOffset).toISOString().slice(0, 10);
}

function companyMidnight(instant: Date, daysLater: number): string {
	const day = new Date(`${companyDay(instant)}T00:00:00.000Z`);
	return new Date(day.getTime() + daysLater * 24 * 60 * 60 * 1000 - seoulOffset).toISOString();
}

async function companyWithAnEvent(slug: string, title: string): Promise<Company> {
	const provisioned = await provisionCompany(
		record,
		{ name: slug, slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-founder@example.test`
	);
	const adminID = await seatMember(provisioned.companyID, `${slug}-admin@example.test`, true);
	const colleagueID = await seatMember(provisioned.companyID, `${slug}-member@example.test`, false);

	const startsAt = new Date(now.getTime() + 24 * 60 * 60 * 1000).toISOString();
	const endsAt = new Date(now.getTime() + 25 * 60 * 60 * 1000).toISOString();
	const { data: task, error } = await record
		.from('task')
		.insert({
			company_id: provisioned.companyID,
			title,
			is_event: true,
			starts_at: startsAt,
			ends_at: endsAt,
			requester_id: adminID,
			status: 'planned'
		})
		.select('id')
		.single<{ id: string }>();
	if (error) throw new Error(error.message);
	await record.from('task_participant').insert({ task_id: task.id, member_id: adminID });

	const dayOff = new Date(now.getTime() + 3 * 24 * 60 * 60 * 1000);
	const { error: leaveRefused } = await record.from('leave').insert({
		member_id: colleagueID,
		kind: '연차',
		is_paid: true,
		is_deducted: true,
		days: 1,
		status: 'approved',
		starts_at: companyMidnight(dayOff, 0),
		ends_at: companyMidnight(dayOff, 1)
	});
	if (leaveRefused) throw new Error(leaveRefused.message);

	return {
		companyID: provisioned.companyID,
		adminID,
		colleagueID,
		token: await issueFeedToken(await clientForMember(adminID))
	};
}

beforeAll(async () => {
	mine = await companyWithAnEvent(`feed-mine-${stamp}`, '우리 회의');
	theirs = await companyWithAnEvent(`feed-theirs-${stamp}`, '남의 회의');
}, networkHookTimeout);

afterAll(async () => {
	for (const company of [mine, theirs]) {
		if (!company?.companyID) continue;
		const { data: members } = await record
			.from('member')
			.select('user_id')
			.eq('company_id', company.companyID);
		await record.from('company').delete().eq('id', company.companyID);
		for (const member of members ?? []) {
			if (member.user_id) await record.auth.admin.deleteUser(member.user_id);
		}
	}
}, networkHookTimeout);

describe('what the record keeps of a subscription', () => {
	test('is what the address hashes to, never the address', async () => {
		const held = await record
			.from('company')
			.select('calendar')
			.eq('id', mine.companyID)
			.maybeSingle<{ calendar: { subscription?: { tokenHash?: string } } }>();

		const kept = held.data?.calendar.subscription?.tokenHash ?? '';
		expect(kept).toMatch(/^[0-9a-f]{64}$/);
		expect(kept).not.toBe(mine.token);
	});
});

describe('a subscription address', () => {
	test('answers with the events of the company whose token it carries', async () => {
		const feed = await calendarFeedForToken(credentials, mine.token, now);

		expect(feed).not.toBeNull();
		expect(feed).toContain('BEGIN:VCALENDAR');
		expect(feed).toContain('SUMMARY:우리 회의');
	});

	test('shows who is off, not only what is scheduled', async () => {
		const feed = await calendarFeedForToken(credentials, mine.token, now);
		const dayOff = companyDay(new Date(now.getTime() + 3 * 24 * 60 * 60 * 1000)).replace(/-/g, '');

		expect(feed).toContain('· 휴가');
		expect(feed).toContain(`DTSTART;VALUE=DATE:${dayOff}`);
	});

	test('shows nothing of another company, whose token is its own', async () => {
		expect(await calendarFeedForToken(credentials, mine.token, now)).not.toContain('남의 회의');
		expect(await calendarFeedForToken(credentials, theirs.token, now)).toContain('남의 회의');
	});

	test('answers nothing at all to a token nobody holds', async () => {
		expect(await calendarFeedForToken(credentials, 'f'.repeat(64), now)).toBeNull();
		expect(await calendarFeedForToken(credentials, 'not-a-token', now)).toBeNull();
	});

	test('dies when it is replaced, so a leaked address stops working', async () => {
		const replaced = await issueFeedToken(await clientForMember(mine.adminID));

		expect(await calendarFeedForToken(credentials, mine.token, now)).toBeNull();
		expect(await calendarFeedForToken(credentials, replaced, now)).toContain('우리 회의');
		mine.token = replaced;
	});

	test('dies when it is given up', async () => {
		await forgetFeedToken(await clientForMember(theirs.adminID));

		expect(await calendarFeedForToken(credentials, theirs.token, now)).toBeNull();
	});
});

describe('the calendar belongs to the company', () => {
	test('so somebody who is not an admin cannot register a subscription', async () => {
		const colleague = await clientForMember(mine.colleagueID);

		expect(issueFeedToken(colleague)).rejects.toThrow();
	});

	test('so one address answers for the whole company, not for whoever registered it', async () => {
		expect(await calendarFeedForToken(credentials, mine.token, now)).toContain('우리 회의');
	});

	test('so the company is what carries it', async () => {
		const colleague = await clientForMember(mine.colleagueID);
		const seen = await colleague
			.from('company')
			.select('calendar')
			.eq('id', mine.companyID)
			.maybeSingle<{ calendar: { subscription?: { tokenHash?: string } } }>();

		expect(seen.error).toBeNull();
		expect(seen.data?.calendar.subscription?.tokenHash).toMatch(/^[0-9a-f]{64}$/);
	});
});
