import { afterEach, beforeEach, expect, mock, spyOn, test } from 'bun:test';
import * as api from '../../../src/lib/public-api-call';
import { AttendanceTeamState } from '../../../src/routes/attendance/team/attendance-team-state.svelte';
import type { AttendanceTeamPage } from '../../../src/lib/attendance/team-page';

let originalState: unknown;
beforeEach(() => { originalState = Reflect.get(globalThis, '$state'); Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value); });
afterEach(() => { mock.restore(); if (originalState === undefined) Reflect.deleteProperty(globalThis, '$state'); else Reflect.set(globalThis, '$state', originalState); });

function page(teamKey = 'team-0'): AttendanceTeamPage {
	return { companyID: 'sample-company', timeZone: 'UTC', serverTime: '2026-10-06T03:00:00Z', authorization: { isAdmin: true, teamViewVisibleToAll: true }, teamOffset: 0, teamLimit: 6, teamTotal: 2, selectedTeamKey: teamKey, memberOffset: 0, memberLimit: 24, memberTotal: 1,
		teams: [{ teamKey, name: 'Sample team', memberCount: 1, working: 1, done: 0, away: 0, needsCheckout: 0, notStarted: 0, recentClockIns: [], recentClockOuts: [], recordedLocations: [], unknownLocationCount: 0 }],
		members: [{ memberID: 'sample-member', name: '이샘플', email: 'sample@example.com', teamKey, status: 'working', latestAt: null, location: null }] };
}

test('same-page team refresh retains loaded cards and reports transient failure', async () => {
	const read = spyOn(api, 'invokeTool').mockResolvedValue(page());
	const state = new AttendanceTeamState();
	await state.loadTeams();
	const gate = Promise.withResolvers<AttendanceTeamPage>();
	read.mockReturnValue(gate.promise);
	const refresh = state.loadTeams();
	expect(state.isLoadingTeams).toBe(true);
	expect(state.teams).toHaveLength(1);
	gate.reject(new Error('temporary outage'));
	await refresh;
	expect(state.teams).toHaveLength(1);
	expect(state.error).toBe('temporary outage');
	expect(state.isLoadingTeams).toBe(false);
});

test('team and filter changes clear foreign rows while stale requests cannot restore them', async () => {
	const read = spyOn(api, 'invokeTool').mockResolvedValue(page());
	const state = new AttendanceTeamState();
	state.selectedTeamKey = 'team-0';
	await state.loadMembers();
	const older = Promise.withResolvers<AttendanceTeamPage>();
	const newer = Promise.withResolvers<AttendanceTeamPage>();
	read.mockReturnValueOnce(older.promise).mockReturnValueOnce(newer.promise);
	state.selectTeam('team-1');
	expect(state.members).toEqual([]);
	state.filter('sample', 'Office');
	older.resolve(page('team-0'));
	await Promise.resolve(); await Promise.resolve();
	expect(state.members).toEqual([]);
	newer.resolve(page('team-1'));
	await Promise.resolve(); await Promise.resolve();
	expect(state.members[0]?.teamKey).toBe('team-1');
});

test('authorization refusal clears previously visible cards and member data', async () => {
	const read = spyOn(api, 'invokeTool').mockResolvedValue(page());
	const state = new AttendanceTeamState();
	await state.loadTeams();
	state.selectedTeamKey = 'team-0';
	await state.loadMembers();
	read.mockRejectedValue(new api.ToolRefused('forbidden', 'FORBIDDEN', 403));
	await state.loadTeams();
	expect(state.teams).toEqual([]);
	expect(state.members).toEqual([]);
	expect(state.selectedTeamKey).toBe('');
});

test('leaving a team retires its pending read without a stuck dashboard busy state', async () => {
	const pending = Promise.withResolvers<AttendanceTeamPage>();
	spyOn(api, 'invokeTool').mockReturnValue(pending.promise);
	const state = new AttendanceTeamState();
	state.selectedTeamKey = 'team-0';
	const reading = state.loadMembers();
	expect(state.isLoadingMembers).toBe(true);
	state.clearSelection();
	expect(state.isLoadingMembers).toBe(false);
	pending.resolve(page());
	await reading;
	expect(state.members).toEqual([]);
	expect(state.isLoadingMembers).toBe(false);
});
