import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { buildEmployeeLeaveFixture } from '../../../dev-attendance-leave-fixture';
import {
	buildLeaveHistory,
	milliDaysValue
} from '../../../src/routes/attendance/leave/leave-history-model';

let originalState: unknown;

beforeAll(() => {
	originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
});

afterAll(() => {
	if (originalState === undefined) {
		Reflect.deleteProperty(globalThis, '$state');
	} else {
		Reflect.set(globalThis, '$state', originalState);
	}
});

describe('employee leave history model', () => {
	test('lists every request latest first', () => {
		const fixture = buildEmployeeLeaveFixture();
		const history = buildLeaveHistory(fixture.requests);

		expect(history).toHaveLength(fixture.requests.length);
		const timestamps = history.map((item) => item.occurredAt);
		expect([...timestamps].sort((first, second) => second.localeCompare(first))).toEqual(timestamps);
	});

	test('names each item after the request it carries', () => {
		const fixture = buildEmployeeLeaveFixture();
		const history = buildLeaveHistory(fixture.requests);
		const pending = history.find((item) => item.id === 'request:leave-request-pending');

		expect(pending?.request.status).toBe('pending');
		expect(milliDaysValue(13500)).toBe('13.5');
	});
});

describe('leave request draft', () => {
	test('keeps date and partial-time fields separate', async () => {
		const { LeaveRequestDraft } = await import(
			'../../../src/routes/attendance/leave/leave-request-draft.svelte'
		);
		const fixture = buildEmployeeLeaveFixture();
		const draft = new LeaveRequestDraft();

		draft.synchronize(fixture.leaveTypes, '2026-08-03');
		expect(draft.previewRequest()).toEqual({
			leaveTypeID: 'annual',
			unit: 'fullDay',
			startDate: '2026-08-03',
			endDate: '2026-08-03'
		});

		draft.setUnit('quarterDay');
		expect(draft.partialPeriod).toBe('custom');
		expect(draft.previewRequest()).toBe(null);
		draft.startTime = '15:00';

		expect(draft.submission()).toEqual({
			leaveTypeID: 'annual',
			unit: 'quarterDay',
			startDate: '2026-08-03',
			partialPeriod: 'custom',
			startTime: '15:00',
			reason: ''
		});
	});

	test('passes over a leave type nobody may ask for any more', async () => {
		const { LeaveRequestDraft } = await import(
			'../../../src/routes/attendance/leave/leave-request-draft.svelte'
		);
		const fixture = buildEmployeeLeaveFixture();
		const draft = new LeaveRequestDraft();

		draft.leaveTypeID = 'inactive-type';
		draft.synchronize(
			[
				{
					id: 'inactive-type',
					name: '이전 특별 휴가',
					balanceMode: 'none' as const,
					allowedUnits: ['quarterDay' as const],
					includeInSummary: false,
					isActive: false
				},
				...fixture.leaveTypes
			],
			'2026-08-03'
		);

		expect(draft.leaveTypeID).toBe('annual');
	});

	test('fully resets for the next request', async () => {
		const { LeaveRequestDraft } = await import(
			'../../../src/routes/attendance/leave/leave-request-draft.svelte'
		);
		const fixture = buildEmployeeLeaveFixture();
		const draft = new LeaveRequestDraft();

		draft.setUnit('quarterDay');
		draft.startTime = '15:00';
		draft.reason = '개인 일정';
		draft.reset(fixture.leaveTypes, '2026-08-03');

		expect({
			leaveTypeID: draft.leaveTypeID,
			unit: draft.unit,
			startDate: draft.startDate,
			endDate: draft.endDate,
			partialPeriod: draft.partialPeriod,
			startTime: draft.startTime,
			reason: draft.reason
		}).toEqual({
			leaveTypeID: 'annual',
			unit: 'fullDay',
			startDate: '2026-08-03',
			endDate: '2026-08-03',
			partialPeriod: 'morning',
			startTime: '',
			reason: ''
		});
	});
});
