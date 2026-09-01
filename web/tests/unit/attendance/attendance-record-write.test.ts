import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import type { AttendanceWriteResult } from '../../../src/lib/attendance/attendance-write';
import type { AddAttendanceEventRequest, RemoveAttendanceEventRequest } from '../../../src/routes/attendance/attendance-api';
import type {
	AttendanceEvent,
	AttendanceSummary
} from '../../../src/routes/attendance/attendance-context.svelte';

type AdditionModule = typeof import(
	'../../../src/routes/attendance/team/attendance-record-addition.svelte'
);
type RemovalModule = typeof import(
	'../../../src/routes/attendance/team/attendance-record-removal.svelte'
);

const serverTime = new Date('2026-07-15T10:30:00+09:00');

let originalState: unknown;
let AttendanceRecordAdditionState: AdditionModule['AttendanceRecordAdditionState'];
let AttendanceRecordRemovalState: RemovalModule['AttendanceRecordRemovalState'];

beforeAll(async () => {
	originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	({ AttendanceRecordAdditionState } = await import(
		'../../../src/routes/attendance/team/attendance-record-addition.svelte'
	));
	({ AttendanceRecordRemovalState } = await import(
		'../../../src/routes/attendance/team/attendance-record-removal.svelte'
	));
});

afterAll(() => {
	if (originalState === undefined) {
		Reflect.deleteProperty(globalThis, '$state');
	} else {
		Reflect.set(globalThis, '$state', originalState);
	}
});

describe('adding a work record', () => {
	test('opens on the chosen day with the current time and the first work location', () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-15', 'member@example.com');

		expect(fixture.addition.isOpen).toBe(true);
		expect(fixture.addition.localDate).toBe('2026-07-15');
		expect(fixture.addition.localTime).toBe('10:30');
		expect(fixture.addition.locationID).toBe('office');
		expect(fixture.addition.outcome).toBe('saved');
	});

	test('reaches the administrators once the chosen day falls outside the three days', () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-01', 'member@example.com');

		expect(fixture.addition.localTime).toBe('09:00');
		expect(fixture.addition.outcome).toBe('asked');
	});

	test('refuses to submit a colleague record and an empty reason', () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-15', 'colleague@example.com');
		fixture.addition.reason = '누락';
		expect(fixture.addition.outcome).toBe('blocked');
		expect(fixture.addition.canSubmit).toBe(false);

		fixture.addition.email = 'member@example.com';
		fixture.addition.reason = '   ';
		expect(fixture.addition.canSubmit).toBe(false);
	});

	test('sends the trimmed reason and reports what the record did with it', async () => {
		const fixture = createAdditionFixture();
		fixture.result = { outcome: 'asked' };
		fixture.addition.open('2026-07-01', 'member@example.com');
		fixture.addition.reason = '  깜빡했습니다  ';
		await fixture.addition.submit();

		expect(fixture.additions).toEqual([
			{
				email: 'member@example.com',
				kind: 'clock_in',
				localDate: '2026-07-01',
				localTime: '09:00',
				locationID: 'office',
				reason: '깜빡했습니다'
			}
		]);
		expect(fixture.addition.isOpen).toBe(false);
		expect(fixture.addition.completedOutcome).toBe('asked');
	});

	test('keeps the form open and says why when the record refuses', async () => {
		const fixture = createAdditionFixture();
		fixture.failure = new Error('cannot clock out without being clocked in');
		fixture.addition.open('2026-07-15', 'member@example.com');
		fixture.addition.reason = '누락';
		await fixture.addition.submit();

		expect(fixture.addition.isOpen).toBe(true);
		expect(fixture.addition.errorMessage).toBe('cannot clock out without being clocked in');
		expect(fixture.addition.completedOutcome).toBe(null);
	});
});

describe('removing a work record', () => {
	test('reads the outcome from the record it would remove', () => {
		const fixture = createRemovalFixture();
		fixture.removal.open('clock-in');
		expect(fixture.removal.outcome).toBe('asked');
		fixture.removal.open('clock-out');
		expect(fixture.removal.outcome).toBe('saved');
	});

	test('needs a reason before it will remove anything', async () => {
		const fixture = createRemovalFixture();
		fixture.removal.open('clock-out');
		expect(fixture.removal.canSubmit).toBe(false);
		await fixture.removal.submit();
		expect(fixture.removals).toEqual([]);

		fixture.removal.reason = ' 잘못 찍었습니다 ';
		await fixture.removal.submit();
		expect(fixture.removals).toEqual([{ eventID: 'clock-out', reason: '잘못 찍었습니다' }]);
		expect(fixture.removal.completedOutcome).toBe('saved');
	});
});

function createAdditionFixture() {
	const additions: AddAttendanceEventRequest[] = [];
	const fixture = {
		additions,
		result: { outcome: 'saved' } as AttendanceWriteResult,
		failure: null as Error | null,
		addition: undefined as unknown as InstanceType<AdditionModule['AttendanceRecordAdditionState']>
	};
	fixture.addition = new AttendanceRecordAdditionState({
		getSummary: () => createSummary(),
		getCurrentServerTime: () => serverTime,
		addEvent: async (request) => {
			if (fixture.failure) throw fixture.failure;
			additions.push(request);
			return fixture.result;
		},
		processingFailedMessage: '처리하지 못했습니다.'
	});
	return fixture;
}

function createRemovalFixture() {
	const removals: RemoveAttendanceEventRequest[] = [];
	const fixture = {
		removals,
		removal: undefined as unknown as InstanceType<RemovalModule['AttendanceRecordRemovalState']>
	};
	fixture.removal = new AttendanceRecordRemovalState({
		getSummary: () => createSummary(),
		getCurrentServerTime: () => serverTime,
		removeEvent: async (request) => {
			removals.push(request);
			return { outcome: 'saved' };
		},
		processingFailedMessage: '처리하지 못했습니다.'
	});
	return fixture;
}

function createSummary(): AttendanceSummary {
	return {
		month: '2026-07',
		currentUserEmail: 'member@example.com',
		currentMemberID: 'member-1',
		isAdmin: false,
		timeZone: 'Asia/Seoul',
		backdatedAfterMinutes: 4320,
		events: [
			createEvent('clock-in', 'clock_in', '2026-07-01T09:00:00+09:00'),
			createEvent('clock-out', 'clock_out', '2026-07-15T10:00:00+09:00')
		],
		absences: [],
		members: [],
		todayStatus: 'working',
		locations: [{ id: 'office', name: '사무실', color: '#22c55e', isDefault: true }],
		teamViewVisibleToAll: true,
		teamViewBlocked: false
	};
}

function createEvent(id: string, kind: AttendanceEvent['kind'], occurredAt: string): AttendanceEvent {
	return {
		id,
		mattermostUserID: '',
		mattermostUsername: '',
		email: 'member@example.com',
		displayName: '구성원',
		kind,
		occurredAt,
		localDate: occurredAt.slice(0, 10),
		localTime: occurredAt.slice(11, 16),
		timeZoneAtEvent: 'Asia/Seoul',
		source: 'web',
		resultPostID: ''
	};
}
