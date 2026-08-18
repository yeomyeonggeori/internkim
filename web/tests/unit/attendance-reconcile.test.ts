import { describe, expect, test } from 'bun:test';
import {
	attendanceWorkCalendarFromDevice,
	attendanceWorkModeFromDevice,
	attendanceWorkPolicyFromDevice,
	InvalidAttendanceWorkCalendarError
} from '../../src/lib/server/attendance-work-calendar-reconcile';
import {
	EmptyWindowRefused,
	paired,
	reconcileMember,
	type DeviceAttendance,
	type RecordedAttendance
} from '../../src/lib/server/attendance-reconcile';

describe('attendanceWorkModeFromDevice', () => {
	test('accepts only an actual supported work mode', () => {
		expect(attendanceWorkModeFromDevice('fixed')).toBe('fixed');
		expect(attendanceWorkModeFromDevice('flexible')).toBe('flexible');
		expect(attendanceWorkModeFromDevice('autonomous')).toBe('autonomous');
	});

	test('keeps reconciling when an earlier device has no work mode projection', () => {
		expect(attendanceWorkModeFromDevice(undefined)).toBe(undefined);
	});

	test('rejects a malformed work mode projection', () => {
		expect(() => attendanceWorkModeFromDevice('hybrid')).toThrow('workMode');
		expect(() => attendanceWorkModeFromDevice(null)).toThrow('workMode');
	});
});

describe('attendanceWorkPolicyFromDevice', () => {
	test('accepts one current work policy with editable night hours', () => {
		expect(attendanceWorkPolicyFromDevice(currentPolicy())).toEqual(currentPolicy());
	});

	test('keeps reconciling with an older device that sends no current policy', () => {
		expect(attendanceWorkPolicyFromDevice(undefined)).toBe(undefined);
	});

	test('rejects a malformed current policy', () => {
		expect(() => attendanceWorkPolicyFromDevice({ ...currentPolicy(), nightEndTime: 6 })).toThrow(
			'nightEndTime'
		);
	});
});

describe('attendanceWorkCalendarFromDevice', () => {
	test('accepts the date-level work calendar contract', () => {
		expect(
			attendanceWorkCalendarFromDevice([
				{ date: '2027-01-04', workMode: 'fixed', workingDate: true },
				{ date: '2027-01-05', workMode: 'fixed', workingDate: false }
			], '2027-01-04T00:00:00Z', '2027-01-06T00:00:00Z')
		).toEqual([
			{ date: '2027-01-04', workMode: 'fixed', workingDate: true },
			{ date: '2027-01-05', workMode: 'fixed', workingDate: false }
		]);
	});

	test('keeps reconciling when an earlier device has no work calendar projection', () => {
		expect(
			attendanceWorkCalendarFromDevice(
				undefined,
				'2027-01-04T00:00:00Z',
				'2027-01-06T00:00:00Z'
			)
		).toBe(undefined);
	});

	test('rejects a malformed work calendar projection', () => {
		const from = '2027-01-04T00:00:00Z';
		const to = '2027-01-05T00:00:00Z';
		expect(() => attendanceWorkCalendarFromDevice(null, from, to)).toThrow('workCalendar');
		expect(() =>
			attendanceWorkCalendarFromDevice([
				{ date: '2027-02-30', workMode: 'fixed', workingDate: true }
			], from, to)
		).toThrow('workCalendar');
		expect(() =>
			attendanceWorkCalendarFromDevice([
				{ date: '2027-01-04', workMode: 'hybrid', workingDate: true }
			], from, to)
		).toThrow('workCalendar');
		expect(() =>
			attendanceWorkCalendarFromDevice([
				{ date: '2027-01-04', workMode: 'fixed', workingDate: 'yes' }
			], from, to)
		).toThrow('workCalendar');
		expect(() =>
			attendanceWorkCalendarFromDevice([
				{ date: '2027-01-04', workMode: 'fixed', workingDate: true, holidayName: 'closure' }
			], from, to)
		).toThrow('workCalendar');
	});

	const completeWindow = [
		{ date: '2027-01-01', workMode: 'fixed', workingDate: true },
		{ date: '2027-01-02', workMode: 'fixed', workingDate: true },
		{ date: '2027-01-03', workMode: 'fixed', workingDate: false }
	];
	const invalidCalendars = [
		{ name: 'empty', workCalendar: [] },
		{ name: 'duplicate', workCalendar: [completeWindow[0], completeWindow[0], completeWindow[2]] },
		{ name: 'missing', workCalendar: [completeWindow[0], completeWindow[2]] },
		{ name: 'reversed', workCalendar: [completeWindow[1], completeWindow[0], completeWindow[2]] },
		{
			name: 'out-of-window',
			workCalendar: [
				{ date: '2026-12-31', workMode: 'fixed', workingDate: true },
				completeWindow[0],
				completeWindow[1]
			]
		},
		{
			name: 'oversized',
			workCalendar: [
				...completeWindow,
				{ date: '2027-01-04', workMode: 'fixed', workingDate: true }
			]
		}
	];
	for (const invalid of invalidCalendars) {
		test(`rejects an explicitly ${invalid.name} calendar for the reconciliation window`, () => {
			expect(() =>
				attendanceWorkCalendarFromDevice(
					invalid.workCalendar,
					'2027-01-01T00:00:00Z',
					'2027-01-04T00:00:00Z'
				)
			).toThrow('workCalendar');
		});
	}

	test('accepts an empty calendar for a zero-day parser window', () => {
		expect(
			attendanceWorkCalendarFromDevice(
				[],
				'2027-01-01T12:00:00Z',
				'2027-01-01T12:00:00Z'
			)
		).toEqual([]);
	});

	test('rejects a window over one month before enumerating calendar dates', () => {
		const originalSetUTCDate = Date.prototype.setUTCDate;
		Date.prototype.setUTCDate = function (): number {
			throw new Error('calendar date enumeration started');
		};
		let thrown: unknown;
		try {
			attendanceWorkCalendarFromDevice(
				[],
				'0100-01-01T00:00:00Z',
				'9999-01-01T00:00:00Z'
			);
		} catch (caught) {
			thrown = caught;
		} finally {
			Date.prototype.setUTCDate = originalSetUTCDate;
		}

		expect(thrown).toBeInstanceOf(InvalidAttendanceWorkCalendarError);
	});
});

function currentPolicy() {
	return {
		workMode: 'fixed',
		workingWeekdays: [1, 2, 3, 4, 5],
		dailyTargetMinutes: 480,
		weeklyTargetMinutes: 2400,
		referenceStartTime: '09:00',
		fixedStartTime: '09:00',
		fixedEndTime: '18:00',
		coreTimeEnabled: false,
		coreStartTime: '',
		coreEndTime: '',
		breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
		nightStartTime: '22:00',
		nightEndTime: '06:00'
	};
}

function onTheDevice(entries: Partial<DeviceAttendance>[]): DeviceAttendance[] {
	return entries.map((entry) => ({
		kind: entry.kind ?? 'clock_in',
		occurredAt: entry.occurredAt ?? '2026-08-05T08:29:00Z',
		location: entry.location ?? ''
	}));
}

function inTheRecord(entries: Partial<RecordedAttendance>[]): RecordedAttendance[] {
	return entries.map((entry, index) => ({
		id: entry.id ?? `row-${index}`,
		kind: entry.kind ?? 'clock_in',
		occurred_at: entry.occurred_at ?? '2026-08-05T08:29:00Z'
	}));
}

describe('reconcileMember', () => {
	test('what the device has and the record lost is added back', () => {
		const plan = reconcileMember('member-1', onTheDevice([{ occurredAt: '2026-08-05T08:29:00Z' }]), []);

		expect(plan.add).toHaveLength(1);
		expect(plan.add[0].member_id).toBe('member-1');
		expect(plan.remove).toEqual([]);
	});

	test('what the record kept after the device corrected it is removed', () => {
		const plan = reconcileMember(
			'member-1',
			onTheDevice([{ occurredAt: '2026-08-07T03:56:00Z' }]),
			inTheRecord([
				{ id: 'kept', occurred_at: '2026-08-07T03:56:00Z' },
				{ id: 'cancelled', occurred_at: '2026-08-05T08:29:00Z' }
			])
		);

		expect(plan.remove).toEqual(['cancelled']);
		expect(plan.add).toEqual([]);
	});

	test('a row already right is neither added nor removed', () => {
		const plan = reconcileMember(
			'member-1',
			onTheDevice([{ occurredAt: '2026-08-05T08:29:00Z' }]),
			inTheRecord([{ id: 'same', occurred_at: '2026-08-05T08:29:00Z' }])
		);

		expect(plan).toEqual({ add: [], remove: [] });
	});

	test('the same second twice on the device is one event, not two', () => {
		const plan = reconcileMember(
			'member-1',
			onTheDevice([{ occurredAt: '2026-08-05T08:29:00Z' }, { occurredAt: '2026-08-05T08:29:00.400Z' }]),
			[]
		);

		expect(plan.add).toHaveLength(1);
	});

	test('an accidental clock-in and the clock-out cancelling it are different events', () => {
		const plan = reconcileMember(
			'member-1',
			onTheDevice([
				{ kind: 'clock_in', occurredAt: '2026-08-05T08:29:00Z' },
				{ kind: 'clock_out', occurredAt: '2026-08-05T08:29:00Z' }
			]),
			[]
		);

		expect(plan.add).toHaveLength(2);
	});

	test('a window the device reports as empty is refused rather than emptied', () => {
		expect(() => reconcileMember('member-1', [], inTheRecord([{}, {}]))).toThrow(EmptyWindowRefused);
	});

	test('an empty window on both sides is nothing to do, not a refusal', () => {
		expect(reconcileMember('member-1', [], [])).toEqual({ add: [], remove: [] });
	});

	test('a plan is only ever about the member it names', () => {
		const plan = reconcileMember('member-2', onTheDevice([{ occurredAt: '2026-08-05T08:29:00Z' }]), []);

		expect(plan.add.every((row) => row.member_id === 'member-2')).toBe(true);
	});
});

describe('where a clock happened', () => {
	test('a clock-out carries no location, because the record refuses one', () => {
		const plan = reconcileMember(
			'member-1',
			onTheDevice([{ kind: 'clock_out', occurredAt: '2026-08-05T18:00:00Z', location: '본사' }]),
			[]
		);

		expect(plan.add[0].location).toBeNull();
	});

	test('a clock-in with nowhere named stores nothing rather than an empty string', () => {
		const plan = reconcileMember('member-1', onTheDevice([{ kind: 'clock_in', location: '  ' }]), []);

		expect(plan.add[0].location).toBeNull();
	});

	test('a clock-in somewhere keeps where', () => {
		const plan = reconcileMember('member-1', onTheDevice([{ kind: 'clock_in', location: '본사' }]), []);

		expect(plan.add[0].location).toBe('본사');
	});
});

describe('paired', () => {
	test('a day at the same place that nobody clocked out of is dropped', () => {
		const kept = paired(
			onTheDevice([
				{ kind: 'clock_in', occurredAt: '2026-06-18T02:17:00Z', location: '사무실' },
				{ kind: 'clock_in', occurredAt: '2026-06-19T02:36:00Z', location: '사무실' },
				{ kind: 'clock_out', occurredAt: '2026-06-19T09:00:00Z' }
			])
		);

		expect(kept.map((event) => event.occurredAt)).toEqual(['2026-06-19T02:36:00Z', '2026-06-19T09:00:00Z']);
	});

	test('the same click twice is one arrival', () => {
		const kept = paired(
			onTheDevice([
				{ kind: 'clock_in', occurredAt: '2026-06-19T02:31:00Z', location: '사무실' },
				{ kind: 'clock_in', occurredAt: '2026-06-19T02:31:30Z', location: '사무실' }
			])
		);

		expect(kept.map((event) => event.occurredAt)).toEqual(['2026-06-19T02:31:30Z']);
	});

	test('moving from one place to another is two days of work, and both are kept', () => {
		const kept = paired(
			onTheDevice([
				{ kind: 'clock_in', occurredAt: '2026-06-18T02:00:00Z', location: '사무실' },
				{ kind: 'clock_in', occurredAt: '2026-06-19T02:00:00Z', location: '재택' }
			])
		);

		expect(kept).toHaveLength(2);
	});

	test('the last clock-in stays, because somebody may be at work right now', () => {
		const kept = paired(onTheDevice([{ kind: 'clock_in', occurredAt: '2026-08-09T02:00:00Z' }]));

		expect(kept).toHaveLength(1);
	});

	test('a day that was clocked out of is untouched', () => {
		const day = onTheDevice([
			{ kind: 'clock_in', occurredAt: '2026-06-18T02:00:00Z' },
			{ kind: 'clock_out', occurredAt: '2026-06-18T09:00:00Z' }
		]);

		expect(paired(day)).toHaveLength(2);
	});

	test('events out of order are read in order before pairing', () => {
		const kept = paired(
			onTheDevice([
				{ kind: 'clock_out', occurredAt: '2026-06-19T09:00:00Z' },
				{ kind: 'clock_in', occurredAt: '2026-06-18T02:00:00Z' },
				{ kind: 'clock_in', occurredAt: '2026-06-19T02:00:00Z' }
			])
		);

		expect(kept.map((event) => event.occurredAt)).toEqual(['2026-06-19T02:00:00Z', '2026-06-19T09:00:00Z']);
	});
});
