import { describe, expect, mock, test } from 'bun:test';

const previewed: unknown[] = [];
const created: unknown[] = [];

mock.module('../../../src/lib/attendance/supabase-leave', () => ({
	cancelSupabaseLeaveRequest: async () => undefined,
	createSupabaseLeaveRequest: async (request: unknown) => void created.push(request),
	supabaseEmployeeLeave: async () => ({ requests: [] }),
	supabaseLeavePreview: async (request: unknown) => {
		previewed.push(request);
		return { occurrences: [], excludedDates: [], totalDeductionMilliDays: 0 };
	}
}));

const { createEmployeeLeaveRequest, previewEmployeeLeave } = await import(
	'../../../src/routes/attendance/leave/employee-leave-api'
);

describe('employee leave API', () => {
	test('drops the optional fields a request left empty', async () => {
		previewed.length = 0;

		await previewEmployeeLeave({
			leaveTypeID: ' annual ',
			unit: 'fullDay',
			startDate: ' 2026-08-03 ',
			endDate: '',
			startTime: '   '
		});

		expect(previewed[0]).toEqual({
			leaveTypeID: 'annual',
			unit: 'fullDay',
			startDate: '2026-08-03',
			endDate: undefined,
			partialPeriod: undefined,
			startTime: undefined
		});
	});

	test('records a request under the trimmed reason it was asked with', async () => {
		created.length = 0;

		await createEmployeeLeaveRequest({
			leaveTypeID: 'annual',
			unit: 'quarterDay',
			startDate: '2026-08-03',
			partialPeriod: 'custom',
			startTime: '15:00',
			reason: '  개인 일정  '
		});

		expect(created[0]).toEqual({
			leaveTypeID: 'annual',
			unit: 'quarterDay',
			startDate: '2026-08-03',
			endDate: undefined,
			partialPeriod: 'custom',
			startTime: '15:00',
			reason: '개인 일정'
		});
	});
});
