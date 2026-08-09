import { describe, expect, test } from 'bun:test';
import {
	EmptyWindowRefused,
	reconcileMember,
	type DeviceAttendance,
	type RecordedAttendance
} from '../../src/lib/server/attendance-reconcile';

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
