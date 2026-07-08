import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import { expect, test } from './attendance-page-test-fixture';
import {
	buildOpenOvernightAttendanceSummary,
	buildOvernightAttendanceSummary,
	currentUserEvent,
	parseUpdateAttendanceEventRequest,
	replaceSummaryEvent,
	selectKorean,
	todayDateInSeoul,
} from './attendance-test-helpers';

test.describe('attendance personal tools', () => {
	test('shows selected personal day details from the team status cell', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		const todayDate = todayDateInSeoul();
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();

		const detailDialog = page.getByTestId('team-status-day-detail-dialog');
		const segments = detailDialog.getByTestId('team-status-day-segment');
		await expect(segments).toHaveCount(3);
		await expect(segments.nth(0)).toContainText('08:30-10:20');
		await expect(segments.nth(1)).toContainText('10:45-12:20');
		await expect(segments.nth(2)).toContainText('12:45~');

		const detailPanel = detailDialog.getByTestId('personal-day-detail-panel');
		const eventLabels = detailPanel.getByTestId('personal-day-event-label');
		await expect(eventLabels.nth(0)).toContainText('출근 08:30');
		await expect(eventLabels.nth(1)).toContainText('퇴근 10:20');
		await expect(eventLabels.nth(2)).toContainText('출근 10:45');
		await expect(eventLabels.nth(3)).toContainText('퇴근 12:20');
		await expect(eventLabels.nth(4)).toContainText('출근 12:45');
	});

	test('shows overnight work as split daily segments', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			await route.fulfill({ json: buildOvernightAttendanceSummary('2026-06') });
		});

		await page.goto('/attendance');
		await selectKorean(page);

		await page.getByTestId('team-status-cell-kim@example.com-2026-06-01').click();
		const firstDayDialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(firstDayDialog.getByText('22:00:00-24:00:00')).toBeVisible();
		await expect(firstDayDialog.getByText('진행 중')).toHaveCount(0);

		await page.keyboard.press('Escape');
		await page.getByTestId('team-status-cell-kim@example.com-2026-06-02').click();
		const secondDayDialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(secondDayDialog.getByText('00:00:00-02:00:00')).toBeVisible();
		await expect(secondDayDialog.getByText('진행 중')).toHaveCount(0);
	});

	test('keeps overnight work in progress after midnight', async ({ page }) => {
		const todayDate = '2026-06-02';
		const previousDate = '2026-06-01';
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			await route.fulfill({ json: buildOpenOvernightAttendanceSummary(todayDate, previousDate) });
		});

		await page.clock.setFixedTime(new Date('2026-06-02T01:00:00+09:00'));
		await page.goto('/attendance');
		await selectKorean(page);

		await expect(page.getByRole('button', { name: '퇴근' })).toBeVisible();
		await page.getByTestId(`team-status-cell-kim@example.com-${previousDate}`).click();
		const firstDayDialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(firstDayDialog.getByText('22:00:00-24:00:00')).toBeVisible();
		await expect(firstDayDialog.getByText('진행 중')).toHaveCount(0);
		await page.keyboard.press('Escape');

		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();
		const secondDayDialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(secondDayDialog.getByText('00:00:00~')).toBeVisible();
		await expect(secondDayDialog.getByTestId('team-status-day-segment').getByText('01시간', { exact: true })).toBeVisible();
	});

	test('registers own absence without sending an email override', async ({ page }) => {
		let requestBody: Record<string, unknown> = {};
		await page.route('**/attendance/api/absences', async (route) => {
			requestBody = JSON.parse(route.request().postData() ?? '{}') as Record<string, unknown>;
			await route.fulfill({
				json: {
					absences: [
						{
							id: 'absence-created',
							email: 'kim@example.com',
							kind: 'leave',
							labelKey: 'leave',
							date: '2026-06-10',
							createdAt: '2026-06-10T09:00:00+09:00'
						}
					]
				}
			});
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('button', { name: '부재 등록' }).click();

		const absenceDialog = page.getByRole('dialog');
		await absenceDialog.getByLabel('시작일').fill('2026-06-10');
		await absenceDialog.getByLabel('종료일').fill('2026-06-10');
		await absenceDialog.getByLabel('사유').fill('family');
		await absenceDialog.getByRole('button', { name: '등록', exact: true }).click();

		await expect(absenceDialog.getByText('부재를 등록했습니다.')).toBeVisible();
		expect(requestBody).toEqual({
			kind: 'leave',
			startDate: '2026-06-10',
			endDate: '2026-06-10',
			reason: 'family'
		});
	});

	test('shows a weekday-only notice when a weekend absence creates no records', async ({ page }) => {
		await page.route('**/attendance/api/absences', async (route) => {
			await route.fulfill({ json: { absences: [] } });
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('button', { name: '부재 등록' }).click();

		const absenceDialog = page.getByRole('dialog');
		await absenceDialog.getByLabel('시작일').fill('2026-06-13');
		await absenceDialog.getByLabel('종료일').fill('2026-06-14');
		await absenceDialog.getByRole('button', { name: '등록', exact: true }).click();

		await expect(absenceDialog.getByText('등록할 평일이 없습니다.')).toBeVisible();
		await expect(absenceDialog.getByText('부재를 등록했습니다.')).toHaveCount(0);
	});

	test('updates an attendance event only after saving an override reason', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		let summary = buildAttendanceSummaryFixture(todayDate.slice(0, 7));
		const targetEvent = currentUserEvent(summary, todayDate, 'clock_in');
		if (!targetEvent) throw new Error('Expected a current user clock-in event fixture');

		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			await route.fulfill({ json: summary });
		});
		await page.route('**/attendance/api/events/*', async (route) => {
			const request = parseUpdateAttendanceEventRequest(route.request().postData());
			const eventID = decodeURIComponent(route.request().url().split('/').at(-1) ?? '');
			summary = replaceSummaryEvent(summary, eventID, request);
			await route.fulfill({ json: { ok: true } });
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();

		const detailDialog = page.getByTestId('team-status-day-detail-dialog');
		const detailPanel = detailDialog.getByTestId('personal-day-detail-panel');
		const eventRow = detailPanel
			.getByTestId('personal-day-event-row')
			.filter({ has: page.getByTestId('personal-day-event-label').filter({ hasText: '출근 08:30' }) });
		await eventRow.getByTestId('personal-day-event-label').click();
		await eventRow.getByRole('button', { name: '수정' }).click();
		await eventRow.getByLabel('시간').fill('08:40');
		await eventRow.getByLabel('장소').click();
		await page.getByRole('option', { name: '사무실', exact: true }).click();
		await eventRow.getByLabel('수정 사유').fill('시간 보정');
		await eventRow.getByRole('button', { name: '저장' }).click();

		const updatedEventRow = detailPanel
			.getByTestId('personal-day-event-row')
			.filter({ has: page.getByTestId('personal-day-event-label').filter({ hasText: '출근 08:40' }) });
		await expect(updatedEventRow.getByTestId('personal-day-event-label')).toContainText('사무실');
		await expect(updatedEventRow.getByText('수정 전:')).toBeVisible();
		await expect(updatedEventRow.getByText('08:30 · 재택')).toBeVisible();
		await expect(updatedEventRow.getByText('수정 후:')).toBeVisible();
		await expect(updatedEventRow.getByText('08:40 · 사무실', { exact: true })).toBeVisible();
		await expect(updatedEventRow.getByText('수정 사유:')).toBeVisible();
		await expect(updatedEventRow.getByText('시간 보정')).toBeVisible();
	});

	test('cancels an own absence from the selected day panel', async ({ page }) => {
		let summary = buildAttendanceSummaryFixture('2026-06');
		const targetAbsence = summary.absences.find(
			(absence) => absence.email === summary.currentUserEmail && absence.kind === 'leave'
		);
		if (!targetAbsence) throw new Error('Expected a current user leave absence fixture');
		let deletedAbsenceID = '';

		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? summary.month;
			await route.fulfill({ json: month === summary.month ? summary : buildAttendanceSummaryFixture(month) });
		});
		await page.route('**/attendance/api/absences/*', async (route) => {
			deletedAbsenceID = decodeURIComponent(route.request().url().split('/').at(-1) ?? '');
			summary = {
				...summary,
				absences: summary.absences.filter((absence) => absence.id !== deletedAbsenceID)
			};
			await route.fulfill({ json: { ok: true } });
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${targetAbsence.date}`).click();

		const detailDialog = page.getByTestId('team-status-day-detail-dialog');
		const detailPanel = detailDialog.getByTestId('personal-day-detail-panel');
		await expect(detailPanel.getByText('휴가')).toBeVisible();
		await detailPanel.getByRole('button', { name: '취소' }).click();

		expect(deletedAbsenceID).toBe(targetAbsence.id);
		await expect(detailPanel.getByText('휴가')).toHaveCount(0);
	});
});
