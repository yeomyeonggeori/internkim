import { describe, expect, mock, test } from 'bun:test';

const asked: { name: string; input: Record<string, unknown> }[] = [];

class ToolRefused extends Error {
	constructor(
		message: string,
		readonly errorCode: string | undefined,
		readonly status: number
	) {
		super(message);
		this.name = 'ToolRefused';
	}
}

let refuseWith: ToolRefused | null = null;

mock.module('../../../src/lib/public-api-call', () => ({
	ToolRefused,
	invokeTool: async (name: string, input: Record<string, unknown>) => {
		if (refuseWith) throw refuseWith;
		asked.push({ name, input });
		return {};
	}
}));

mock.module('../../../src/lib/attendance/announce-attendance', () => ({
	announceToTheCompany: async () => undefined
}));

mock.module('../../../src/lib/supabase', () => ({
	supabase: () => ({
		from: () => ({
			select: () => ({
				limit: () => ({ single: async () => ({ data: { timezone: 'Asia/Seoul' } }) })
			})
		})
	})
}));

const { cancelSupabaseLeaveRequest, createSupabaseLeaveRequest, leaveSpanAsked } = await import(
	'../../../src/lib/attendance/supabase-leave'
);

describe('a leave the browser files', () => {
	test('names a whole day by its dates and lets the record decide the moments', async () => {
		asked.length = 0;
		refuseWith = null;

		await createSupabaseLeaveRequest({
			leaveTypeID: 'annual',
			unit: 'fullDay',
			startDate: '2026-08-03',
			endDate: '2026-08-05',
			reason: '개인 일정'
		});

		expect(asked).toEqual([
			{
				name: 'leave_request',
				input: {
					kind: 'annual',
					startsAt: '2026-08-03',
					endsAt: '2026-08-05',
					days: 3,
					note: '개인 일정'
				}
			}
		]);
	});

	test('names a part of a day by the moments it covers, which no date can say', async () => {
		asked.length = 0;
		refuseWith = null;

		await createSupabaseLeaveRequest({
			leaveTypeID: 'annual',
			unit: 'quarterDay',
			startDate: '2026-08-03',
			partialPeriod: 'custom',
			startTime: '14:00',
			reason: '병원'
		});

		const input = asked[0].input;
		expect(asked[0].name).toBe('leave_request');
		expect(input.days).toBe(0.25);
		expect(new Date(String(input.startsAt)).toISOString()).toBe('2026-08-03T05:00:00.000Z');
		expect(new Date(String(input.endsAt)).toISOString()).toBe('2026-08-03T07:00:00.000Z');
	});

	// The span follows the unit, never the deduction. A whole day of a kind
	// that deducts nothing still covers whole days, and reading the deduction
	// instead would send it as timestamps the moment a policy stops charging
	// for it.
	test('names a whole day by its dates whatever the leave deducts', async () => {
		for (const days of [0, 0.5, 3]) {
			const span = await leaveSpanAsked({
				leaveTypeID: 'unpaid',
				unit: 'fullDay',
				startDate: '2026-08-03',
				endDate: '2026-08-05',
				days
			} as never);
			expect(span).toEqual({ startsAt: '2026-08-03', endsAt: '2026-08-05' });
		}
	});

	test('names a part of a day by its moments whatever the leave deducts', async () => {
		const span = await leaveSpanAsked({
			leaveTypeID: 'annual',
			unit: 'quarterDay',
			startDate: '2026-08-03',
			partialPeriod: 'custom',
			startTime: '14:00',
			days: 3
		} as never);
		expect(new Date(span.startsAt).toISOString()).toBe('2026-08-03T05:00:00.000Z');
		expect(new Date(span.endsAt).toISOString()).toBe('2026-08-03T07:00:00.000Z');
	});

	test('withdraws by the exact leave id the screen already holds', async () => {
		asked.length = 0;
		refuseWith = null;

		await cancelSupabaseLeaveRequest('leave-1');

		expect(asked).toEqual([{ name: 'leave_delete', input: { leaveHint: 'leave-1' } }]);
	});

	test('turns a refusal into the code the leave screens speak', async () => {
		refuseWith = new ToolRefused('nope', undefined, 403);

		await expect(cancelSupabaseLeaveRequest('leave-1')).rejects.toMatchObject({
			name: 'EmployeeLeaveAPIError',
			code: 'invalidStatus',
			status: 403
		});
		refuseWith = null;
	});
});
