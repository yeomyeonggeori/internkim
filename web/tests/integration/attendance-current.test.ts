import { afterAll, beforeAll, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { z } from 'zod';
import { addMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';
import { runToolOverTheRecord } from '../../src/lib/server/public-api/record';
import { leavesTaskLabelsUndecided } from '../../src/lib/server/public-api/record/task-labels';
import { currentAttendanceSchema } from '../../src/lib/attendance/current-attendance';

const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `own-current-${Date.now()}`;
let companyID = '';
let memberID = '';
let peerID = '';
const accountIDs: string[] = [];
const requests: string[] = [];
let caller: ReturnType<typeof createClient>;

beforeAll(async () => {
	const company = await provisionCompany(client, { name: 'Own Current Sample', slug, country: 'US', locale: 'en', timezone: 'America/New_York' }, `${slug}-admin@example.com`);
	companyID = company.companyID;
	memberID = await addMember(client, companyID, `${slug}-owner@example.com`);
	peerID = await addMember(client, companyID, `${slug}-peer@example.com`);
	const { data, error } = await client.auth.admin.createUser({ email: `${slug}-owner@example.com`, email_confirm: true });
	if (error || !data.user) throw new Error('could not create the owned fixture account');
	accountIDs.push(data.user.id);
	await client.from('member').update({ user_id: data.user.id, status: 'active' }).eq('id', memberID);
	const session = await sessionForMember({ projectURL, serviceRoleKey, signingKey }, memberID);
	caller = createClient(projectURL, publishableKey, {
		auth: { persistSession: false, autoRefreshToken: false },
		global: { headers: { Authorization: `Bearer ${session.accessToken}` }, fetch: Object.assign((input: Parameters<typeof fetch>[0], initialization?: Parameters<typeof fetch>[1]) => {
			requests.push(new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url).pathname);
			return fetch(input, initialization);
		}, { preconnect: fetch.preconnect }) }
	});
	const inserted = await client.from('attendance').insert([
		{ member_id: memberID, kind: 'clock_in', occurred_at: new Date(Date.now()-60*86400000).toISOString() },
		{ member_id: peerID, kind: 'clock_in', occurred_at: new Date(Date.now()-3600000).toISOString() }
	]);
	if (inserted.error) throw new Error(inserted.error.message);
}, 60000);

afterAll(async () => {
	if (companyID) await client.from('company').delete().eq('id', companyID);
	for (const accountID of accountIDs) await client.auth.admin.deleteUser(accountID);
}, 60000);

function run(name: string) {
	return runToolOverTheRecord(caller, client, memberID, name, {}, new Date(), leavesTaskLabelsUndecided);
}

test('current tool makes one caller RPC without company or whole-directory context reads', async () => {
	requests.length = 0;
	const start = performance.now();
	const answer = await run('attendance_current_get');
	expect(answer.status).toBe(200);
	const state = z.object({ result: currentAttendanceSchema }).parse(answer.body).result;
	expect(requests).toEqual(['/rest/v1/rpc/attendance_current']);
	expect(state.memberID).toBe(memberID);
	expect(state.todayEvents).toEqual([]);
	expect(state.latestEvent?.personID).toBe(memberID);
	expect(Date.parse(state.latestEvent?.occurredAt ?? '')).toBeLessThan(Date.now()-30*86400000);
	console.log(JSON.stringify({ ownCurrentRecordRequests: requests.length, responseBytes: JSON.stringify(state).length, recordReadyMilliseconds: Math.round(performance.now()-start) }));
});

test('nullable member email remains readable for its bound account', async () => {
	await client.from('member').update({ email: null }).eq('id', memberID);
	const { data, error } = await caller.rpc('attendance_current');
	expect(error).toBeNull();
	expect(currentAttendanceSchema.parse(data).email).toBe('');
});

test('departure removes access even when the caller still has a previous session', async () => {
	await client.from('member').update({ status: 'departed' }).eq('id', memberID);
	const { data, error } = await caller.rpc('attendance_current');
	expect(data).toBeNull();
	expect(error?.code).toBe('42501');
});
