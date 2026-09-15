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
	test('opens on the chosen day with the current time as the start and no end', () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-15', 'member@example.com');

		expect(fixture.addition.isOpen).toBe(true);
		expect(fixture.addition.localDate).toBe('2026-07-15');
		expect(fixture.addition.startTime).toBe('10:30');
		expect(fixture.addition.endTime).toBe('');
		expect(fixture.addition.locationID).toBe('office');
		expect(fixture.addition.outcome).toBe('saved');
	});

	test('opens a past day as a whole workday, ready to submit without touching the times', () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-14', 'member@example.com');

		expect(fixture.addition.startTime).toBe('09:00');
		expect(fixture.addition.endTime).toBe('18:00');
		expect(fixture.addition.isEndTimeMissing).toBe(false);
		expect(fixture.addition.canSubmit).toBe(true);
	});

	test('reaches the administrators once the chosen day falls outside the three days', () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-01', 'member@example.com');

		expect(fixture.addition.startTime).toBe('09:00');
		expect(fixture.addition.outcome).toBe('asked');
	});

	test('refuses to submit a colleague record, and needs no reason for one of its own', () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-15', 'colleague@example.com');
		expect(fixture.addition.outcome).toBe('blocked');
		expect(fixture.addition.canSubmit).toBe(false);

		fixture.addition.email = 'member@example.com';
		fixture.addition.reason = '';
		expect(fixture.addition.canSubmit).toBe(true);
	});

	test('needs at least one of the two moments to submit', () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-15', 'member@example.com');
		fixture.addition.startTime = '';
		fixture.addition.endTime = '';

		expect(fixture.addition.canSubmit).toBe(false);
	});

	test('needs the end on a past day, so no clock-in is left without its clock-out', () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-14', 'member@example.com');
		fixture.addition.startTime = '09:00';
		fixture.addition.endTime = '';

		expect(fixture.addition.isEndTimeMissing).toBe(true);
		expect(fixture.addition.canSubmit).toBe(false);

		fixture.addition.endTime = '18:00';
		expect(fixture.addition.isEndTimeMissing).toBe(false);
		expect(fixture.addition.canSubmit).toBe(true);
	});

	test('lets today go without an end, since the day may still be under way', () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-15', 'member@example.com');
		fixture.addition.startTime = '09:00';
		fixture.addition.endTime = '';

		expect(fixture.addition.isEndTimeMissing).toBe(false);
		expect(fixture.addition.canSubmit).toBe(true);
	});

	test('refuses an end earlier than the start before writing anything', async () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-15', 'member@example.com');
		fixture.addition.startTime = '10:00';
		fixture.addition.endTime = '09:00';

		expect(fixture.addition.isSpanInverted).toBe(true);
		expect(fixture.addition.canSubmit).toBe(false);

		await fixture.addition.submit();

		expect(fixture.additions).toEqual([]);
		expect(fixture.addition.isOpen).toBe(true);
	});

	test('writes a span as two events in order, start then end, with one reason', async () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-15', 'member@example.com');
		fixture.addition.startTime = '09:00';
		fixture.addition.endTime = '18:00';
		fixture.addition.reason = '  구간 전체 누락  ';
		await fixture.addition.submit();

		expect(fixture.additions).toEqual([
			{
				email: 'member@example.com',
				kind: 'clock_in',
				localDate: '2026-07-15',
				localTime: '09:00',
				locationID: 'office',
				reason: '구간 전체 누락'
			},
			{
				email: 'member@example.com',
				kind: 'clock_out',
				localDate: '2026-07-15',
				localTime: '18:00',
				locationID: 'office',
				reason: '구간 전체 누락'
			}
		]);
		expect(fixture.addition.isOpen).toBe(false);
		expect(fixture.addition.completedOutcome).toBe('saved');
	});

	test('writes a single event and no reason when only one side is filled', async () => {
		const fixture = createAdditionFixture();
		fixture.addition.open('2026-07-15', 'member@example.com');
		fixture.addition.startTime = '';
		fixture.addition.endTime = '18:00';
		await fixture.addition.submit();

		expect(fixture.additions).toEqual([
			{
				email: 'member@example.com',
				kind: 'clock_out',
				localDate: '2026-07-15',
				localTime: '18:00',
				locationID: 'office',
				reason: ''
			}
		]);
		expect(fixture.addition.isOpen).toBe(false);
	});

	test('reports asked once either moment of the span asks an administrator', async () => {
		const fixture = createAdditionFixture();
		fixture.results = [{ outcome: 'saved' }, { outcome: 'asked' }];
		fixture.addition.open('2026-07-15', 'member@example.com');
		fixture.addition.startTime = '09:00';
		fixture.addition.endTime = '18:00';
		await fixture.addition.submit();

		expect(fixture.addition.completedOutcome).toBe('asked');
	});

	test('keeps the form open and says why when the record refuses immediately', async () => {
		const fixture = createAdditionFixture();
		fixture.failureAtIndex = 0;
		fixture.addition.open('2026-07-15', 'member@example.com');
		fixture.addition.endTime = '';
		await fixture.addition.submit();

		expect(fixture.addition.isOpen).toBe(true);
		expect(fixture.addition.errorMessage).toBe('cannot clock out without being clocked in');
		expect(fixture.addition.completedOutcome).toBe(null);
		expect(fixture.additions).toEqual([]);
	});

	test('keeps the start it already wrote and says so when the end is refused', async () => {
		const fixture = createAdditionFixture();
		fixture.failureAtIndex = 1;
		fixture.addition.open('2026-07-15', 'member@example.com');
		fixture.addition.startTime = '09:00';
		fixture.addition.endTime = '18:00';
		await fixture.addition.submit();

		expect(fixture.addition.isOpen).toBe(true);
		expect(fixture.addition.completedOutcome).toBe(null);
		expect(fixture.additions).toHaveLength(1);
		expect(fixture.additions[0].kind).toBe('clock_in');
		expect(fixture.addition.errorMessage).toBe(
			'시작 기록은 저장했지만 종료 기록은 저장하지 못했습니다. cannot clock out without being clocked in'
		);
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
		results: [{ outcome: 'saved' }, { outcome: 'saved' }] as AttendanceWriteResult[],
		failureAtIndex: null as number | null,
		failure: new Error('cannot clock out without being clocked in'),
		addition: undefined as unknown as InstanceType<AdditionModule['AttendanceRecordAdditionState']>
	};
	fixture.addition = new AttendanceRecordAdditionState({
		getSummary: () => createSummary(),
		getCurrentServerTime: () => serverTime,
		addEvent: async (request) => {
			const index = additions.length;
			if (fixture.failureAtIndex === index) throw fixture.failure;
			additions.push(request);
			return fixture.results[index] ?? fixture.results.at(-1) ?? { outcome: 'saved' };
		},
		processingFailedMessage: '처리하지 못했습니다.',
		partialSpanFailureMessage: '시작 기록은 저장했지만 종료 기록은 저장하지 못했습니다.'
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
