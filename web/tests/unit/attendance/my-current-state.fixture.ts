import { mock } from 'bun:test';
import { readFileSync } from 'node:fs';
import { compileModule } from 'svelte/compiler';
import type { AttendanceSummary, AttendanceEvent } from '../../../src/routes/attendance/attendance-context.svelte';
import type { AttendanceWriteResult } from '../../../src/lib/attendance/attendance-write';

Bun.plugin({ name: 'current-state-runes', setup(build) {
	build.onLoad({ filter: /\.svelte\.(js|ts)$/ }, ({ path }) => ({
		contents: compileModule(new Bun.Transpiler({ loader: path.endsWith('.ts') ? 'ts' : 'js' }).transformSync(readFileSync(path, 'utf8')), { filename: path, generate: 'client' }).js.code,
		loader: 'js'
	}));
} });

const now = new Date().toISOString();
const olderInstant = new Date(Date.now() - 2 * 86400000).toISOString();
const oldEvent: AttendanceEvent = {
	id: 'old', email: 'own@example.com', displayName: 'Own', kind: 'clock_in', occurredAt: olderInstant,
	localDate: olderInstant.slice(0, 10), localTime: olderInstant.slice(11, 16), timeZoneAtEvent: 'UTC', source: 'web', resultPostID: ''
};
let serverSummary: AttendanceSummary = {
	readScope: 'mine', month: now.slice(0, 7), serverTime: now, timeZone: 'UTC', currentUserEmail: oldEvent.email,
	currentMemberID: 'own', isAdmin: false, events: [oldEvent], absences: [], members: [], todayStatus: 'none',
	locations: [{ id: 'Office', name: 'Office', color: '', isDefault: true }], teamViewVisibleToAll: false, teamViewBlocked: false
};
let reads = 0;
let read: () => Promise<AttendanceSummary> = async () => structuredClone(serverSummary);
let toggle: () => Promise<AttendanceWriteResult | void> = async () => ({ outcome: 'saved', removed: true });
mock.module('../../../src/lib/attendance/supabase-current-attendance', () => ({ supabaseCurrentAttendance: () => { reads += 1; return read(); } }));
mock.module('../../../src/routes/attendance/attendance-api', () => ({
	toggleAttendanceOnServer: () => toggle(),
	addAttendanceEvent: async () => {
		serverSummary = { ...serverSummary, events: [{ ...oldEvent, id: 'closed', kind: 'clock_out' }] };
		return { outcome: 'saved' };
	}
}));
mock.module('../../../src/lib/attendance/supabase-attendance', () => ({ attendanceEventFromClock: () => oldEvent }));
mock.module('$lib/i18n/page-text.svelte', () => ({ createPageText: (value: { en: object }) => value.en }));
mock.module('$lib/widget/attendance-lock-screen', () => ({ lockScreenRefusesTheClock: async () => false }));
mock.module('svelte-sonner', () => ({ toast: { success: () => {}, info: () => {}, error: () => {} } }));
const { myAttendanceToday } = await import('../../../src/lib/attendance/my-attendance-today.svelte');

await myAttendanceToday.load();
await myAttendanceToday.load();
const freshShared = reads === 1;
await myAttendanceToday.load(true);
const forceRefreshes = reads === 2;
await myAttendanceToday.clock('clock_out', 'Office');
const undoRefreshes = reads === 3;

let releaseRead = () => {};
read = () => new Promise<AttendanceSummary>((resolve) => { releaseRead = () => resolve(structuredClone(serverSummary)); });
const pendingRead = myAttendanceToday.load(true);
myAttendanceToday.clear();
releaseRead(); await pendingRead;
const lateReadIgnored = myAttendanceToday.summary === null;
read = async () => structuredClone(serverSummary);
await myAttendanceToday.load();
let releaseWrite = () => {};
toggle = () => new Promise<AttendanceWriteResult>((resolve) => { releaseWrite = () => resolve({ outcome: 'saved', removed: true }); });
const pendingWrite = myAttendanceToday.clock('clock_out', 'Office');
myAttendanceToday.clear();
releaseWrite(); await pendingWrite;
const lateWriteIgnored = myAttendanceToday.summary === null && !myAttendanceToday.isSubmitting && myAttendanceToday.clockFailure === '';

await myAttendanceToday.load();
toggle = async () => { throw new Error('second write refused'); };
let partialFailure = false;
try { await myAttendanceToday.closeAndClockIn('23:59', 'Office'); }
catch { partialFailure = true; }
const partialCloseRefreshed = myAttendanceToday.summary?.events[0]?.kind === 'clock_out' && myAttendanceToday.clockInNobodyClosed === undefined;
read = () => new Promise<AttendanceSummary>((resolve) => { releaseRead = () => resolve(structuredClone(serverSummary)); });
const overlappingRead = myAttendanceToday.load(true);
const pendingFlag = myAttendanceToday.isLoading;
toggle = async () => ({ outcome: 'saved', event: { id: 'overlap', personID: 'own', kind: 'clock_out', occurredAt: now, location: null } });
await myAttendanceToday.clock('clock_out', 'Office');
const mutationClearsReadBusy = pendingFlag && !myAttendanceToday.isLoading;
releaseRead(); await overlappingRead;
const oldReadKeepsBusyCleared = !myAttendanceToday.isLoading;
console.log(JSON.stringify({ freshShared, forceRefreshes, undoRefreshes, lateReadIgnored, lateWriteIgnored, partialFailure, partialCloseRefreshed, mutationClearsReadBusy, oldReadKeepsBusyCleared }));
