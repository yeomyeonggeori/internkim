import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import type { AttendanceSummary } from '../../src/routes/attendance/attendance-context.svelte';
import { expect, test } from './attendance-page-test-fixture';
import { selectKorean } from './attendance-test-helpers';

test.describe('approved partial leave attendance runtime', () => {
	test('shows leave status and asks before an early return', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-07-28T14:00:00+09:00'));
		const fixture = buildAttendanceSummaryFixture('2026-07');
		const sourceEvent = fixture.events[0];
		const sourceAbsence = fixture.absences[0];
		if (!sourceEvent || !sourceAbsence) throw new Error('Expected attendance fixture records');
		const email = fixture.currentUserEmail;
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const summary: AttendanceSummary = {
				...fixture,
				month: '2026-07',
				serverTime: '2026-07-28T05:00:00Z',
				events: [
					{
						...sourceEvent,
						id: 'leave-runtime-clock-in',
						email,
						kind: 'clock_in',
						occurredAt: '2026-07-28T00:00:00Z',
						localDate: '2026-07-28',
						localTime: '09:00:00',
						canceledAt: undefined
					},
					{
						...sourceEvent,
						id: 'leave-runtime-auto-clock-out',
						email,
						kind: 'clock_out',
						occurredAt: '2026-07-28T04:00:00Z',
						localDate: '2026-07-28',
						localTime: '13:00:00',
						source: 'approved_leave',
						canceledAt: undefined
					},
				],
				absences: [
					{
						...sourceAbsence,
						id: 'leave-runtime-absence',
						rangeID: 'leave-runtime-range',
						email,
						kind: 'leave',
						labelKey: 'leave',
						date: '2026-07-28',
						startDate: '2026-07-28',
						endDate: '2026-07-28',
						startTime: '13:00',
						endTime: '15:00',
						reason: '개인 용무',
						canceledAt: undefined
					}
				],
				activeLeave: {
					requestID: 'leave-runtime-request',
					occurrenceID: 'leave-runtime-occurrence',
					leaveTypeID: 'annual',
					leaveTypeName: '연차',
					startTime: '13:00',
					endTime: '15:00',
					deductionMilliDays: 250,
					startAt: '2026-07-28T13:00:00+09:00',
					endAt: '2026-07-28T15:00:00+09:00'
				},
				todayStatus: 'on_leave'
			};
			await route.fulfill({ json: summary });
		});
		await page.goto('/attendance');
		await selectKorean(page);
		const leaveStatus = page.getByTestId('active-leave-status');
		await expect(leaveStatus).toContainText('휴가 중');
		await expect(leaveStatus).toContainText('13:00–15:00');

		await page.getByRole('button', { name: '출근', exact: true }).click();
		const confirmation = page.getByRole('alertdialog', {
			name: '휴가에서 조기 복귀할까요?'
		});
		await expect(confirmation).toContainText('예정 종료 시각은 15:00입니다.');
		await expect(confirmation.getByRole('button', { name: '출근하기' })).toBeVisible();
	});
});
