import { afterAll, beforeAll, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { addMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';
import { runToolOverTheRecord } from '../../src/lib/server/public-api/record';
import { leavesTaskLabelsUndecided } from '../../src/lib/server/public-api/record/task-labels';
import { attendanceTeamPageSchema } from '../../src/lib/attendance/team-page';

const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `team-page-${Date.now()}`;
const accounts: string[] = [];
const requestPaths: string[] = [];
const requestURLs: URL[] = [];
let companyID = '';
let teamID = '';
let memberID = '';
let peerID = '';
let caller: Awaited<ReturnType<typeof boundCaller>>;
let peer: Awaited<ReturnType<typeof boundCaller>>;

async function boundCaller(id: string, email: string, trace = false) {
	const { data, error } = await client.auth.admin.createUser({ email, email_confirm: true });
	if (error || !data.user) throw new Error('could not create the owned fixture account');
	accounts.push(data.user.id);
	await client.from('member').update({ user_id: data.user.id, status: 'active' }).eq('id', id);
	const session = await sessionForMember({ projectURL, serviceRoleKey, signingKey }, id);
	return createClient(projectURL, publishableKey, {
		auth: { persistSession: false, autoRefreshToken: false },
		global: {
			headers: { Authorization: `Bearer ${session.accessToken}` },
			fetch: trace ? Object.assign((input: Parameters<typeof fetch>[0], init?: Parameters<typeof fetch>[1]) => {
				const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url);
				requestPaths.push(url.pathname);
				requestURLs.push(url);
				return fetch(input, init);
			}, { preconnect: fetch.preconnect }) : fetch
		}
	});
}

beforeAll(async () => {
	const company = await provisionCompany(client, {
		name: 'Team Page Sample', slug, country: 'US', locale: 'en', timezone: 'America/New_York'
	}, `${slug}-administrator@example.com`);
	companyID = company.companyID;
	const { data: team, error: teamError } = await client.from('team').insert({
		company_id: companyID, name: 'Sample Team', position: 0
	}).select('id').single();
	if (teamError || !team) throw new Error(teamError?.message ?? 'no fixture team');
	teamID = team.id;
	memberID = await addMember(client, companyID, `${slug}-owner@example.com`);
	peerID = await addMember(client, companyID, `${slug}-peer@example.com`);
	await client.from('member').update({ name: 'Owner Sample', team_id: teamID, is_admin: true }).eq('id', memberID);
	await client.from('member').update({ name: 'Peer Sample', team_id: teamID }).eq('id', peerID);
	await client.from('company').update({ work_locations: [{ name: 'Office' }], rules: { teamViewVisibleToAll: true } }).eq('id', companyID);
	caller = await boundCaller(memberID, `${slug}-owner@example.com`, true);
	peer = await boundCaller(peerID, `${slug}-peer@example.com`);
	const inserted = await client.from('attendance').insert([
		{ member_id: memberID, kind: 'clock_in', occurred_at: new Date(Date.now() - 1000 * 60 * 20).toISOString(), location: 'Office' },
		{ member_id: peerID, kind: 'clock_in', occurred_at: new Date(Date.now() - 1000 * 60 * 15).toISOString(), location: 'Office' },
		{ member_id: peerID, kind: 'clock_out', occurred_at: new Date(Date.now() - 1000 * 60 * 10).toISOString() }
	]);
	if (inserted.error) throw new Error(inserted.error.message);
}, 60000);

afterAll(async () => {
	if (companyID) await client.from('company').delete().eq('id', companyID);
	for (const account of accounts) await client.auth.admin.deleteUser(account);
}, 60000);

async function page(asCaller: Awaited<ReturnType<typeof boundCaller>>, input: Record<string, unknown>) {
	return runToolOverTheRecord(asCaller, client, memberID, 'attendance_team_page_get', input, new Date(), leavesTaskLabelsUndecided);
}

test('team cards are one requester-scoped RPC with real counts and recorded locations', async () => {
	requestPaths.length = 0;
	const answer = await page(caller, { pageKind: 'teams', teamLimit: 1 });
	expect(answer.status).toBe(200);
	const state = attendanceTeamPageSchema.parse((answer.body as { result: unknown }).result);
	expect(requestPaths).toEqual(['/rest/v1/rpc/attendance_team_page']);
	expect(state.teams).toHaveLength(1);
	expect(state.teams[0].memberCount).toBe(2);
	expect(state.teams[0].working).toBe(1);
	expect(state.teams[0].done).toBe(1);
	expect(state.teams[0].recentClockOuts[0].memberID).toBe(peerID);
	expect(state.teams[0].recordedLocations).toEqual([{ name: 'Office', count: 1 }]);
});

test('employee search and recorded-location filter run before paging', async () => {
	const answer = await page(caller, {
		pageKind: 'members', selectedTeamKey: teamID, memberLimit: 1,
		searchText: 'Peer', locationFilter: 'Office'
	});
	expect(answer.status).toBe(200);
	const state = attendanceTeamPageSchema.parse((answer.body as { result: unknown }).result);
	expect(state.memberTotal).toBe(1);
	expect(state.members.map((member) => member.memberID)).toEqual([peerID]);
	expect(state.members[0].status).toBe('done');
});

test('an exact member history read selects only that directory row', async () => {
	requestURLs.length = 0;
	const from = new Date(Date.now() - 1000 * 60 * 60 * 24 * 2).toISOString().slice(0, 10);
	const to = new Date(Date.now() + 1000 * 60 * 60 * 24 * 2).toISOString().slice(0, 10);
	for (const name of ['attendance_list', 'leave_list']) {
		const answer = await runToolOverTheRecord(caller, client, memberID, name,
			{ personHints: [peerID], from, to }, new Date(), leavesTaskLabelsUndecided);
		expect(answer.status).toBe(200);
	}
	const directoryReads = requestURLs.filter((url) => url.pathname === '/rest/v1/member');
	expect(directoryReads).toHaveLength(2);
	expect(directoryReads.every((url) => url.searchParams.get('id') === `in.(${peerID})`)).toBe(true);
});

test('a hidden team view refuses nonadmin without leaking a card', async () => {
	await client.from('company').update({ rules: { teamViewVisibleToAll: false } }).eq('id', companyID);
	const answer = await page(peer, { pageKind: 'teams' });
	expect(answer.status).toBe(403);
	expect((answer.body as { result?: unknown }).result).toBeUndefined();
});

test('a departed requester cannot use a former team page session', async () => {
	await client.from('member').update({ status: 'departed' }).eq('id', memberID);
	const answer = await page(caller, { pageKind: 'teams' });
	expect(answer.status).toBe(403);
});
