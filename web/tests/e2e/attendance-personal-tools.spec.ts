import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import type { AttendanceSummary } from '../../src/routes/attendance/attendance-context.svelte';
import type { Locator, Page } from '@playwright/test';
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

async function openWorkRecordEditor(
	page: Page,
	summary: AttendanceSummary,
	localDate: string
): Promise<Locator> {
	await page.unroute('**/attendance/api/summary**');
	await page.route('**/attendance/api/summary**', async (route) => {
		await route.fulfill({ json: summary });
	});
	await page.goto('/attendance');
	await selectKorean(page);
	await page.getByTestId(`team-status-cell-kim@example.com-${localDate}`).click();

	const detailDialog = page.getByTestId('team-status-day-detail-dialog');
	await detailDialog.getByTestId('work-record-edit-button').click();
	return detailDialog;
}

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
		await expect(detailDialog.getByTestId('team-status-work-record-header').getByTestId('section-count-badge')).toHaveText('3구간');
		const segments = detailDialog.getByTestId('team-status-day-segment');
		await expect(segments).toHaveCount(3);
		await expect(segments.nth(0).getByLabel('08:30-10:20')).toBeVisible();
		await expect(segments.nth(1).getByLabel('10:45-12:20')).toBeVisible();
		await expect(segments.nth(2).getByLabel(/^12:45-/)).toBeVisible();
		await expect(segments.nth(2).locator('[data-slot="time-range-end"]')).toHaveClass(/text-info/);
		await expect(detailDialog.getByTestId('personal-day-detail-panel')).toHaveCount(0);
		await expect(detailDialog.getByText('이벤트', { exact: true })).toHaveCount(0);
		await detailDialog.getByTestId('work-record-edit-button').click();
		await expect(detailDialog.locator('[data-slot="work-segment-edit-fields"]')).toHaveCount(3);
		await expect(detailDialog.getByTestId('work-record-edit-button')).toHaveAttribute('data-state', 'editing');
		await expect(detailDialog.getByTestId('work-record-edit-button')).toHaveAccessibleName('취소');
		await detailDialog.getByTestId('work-record-edit-button').click();
		await expect(detailDialog.locator('[data-slot="work-segment-edit-fields"]')).toHaveCount(0);
		await expect(detailDialog.getByTestId('work-record-edit-button')).toHaveAccessibleName('수정');
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
		await expect(firstDayDialog.getByLabel('22:00:00-24:00:00')).toBeVisible();
		await expect(firstDayDialog.getByText('진행 중')).toHaveCount(0);

		await page.keyboard.press('Escape');
		await page.getByTestId('team-status-cell-kim@example.com-2026-06-02').click();
		const secondDayDialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(secondDayDialog.getByLabel('00:00:00-02:00:00')).toBeVisible();
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
		await expect(firstDayDialog.getByLabel('22:00:00-24:00:00')).toBeVisible();
		await expect(firstDayDialog.getByText('진행 중')).toHaveCount(0);
		await page.keyboard.press('Escape');

		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();
		const secondDayDialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(secondDayDialog.getByLabel('00:00:00-01:00')).toBeVisible();
		await expect(secondDayDialog.getByTestId('team-status-day-segment').getByLabel('01시간 00분')).toBeVisible();
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
		await page.clock.setFixedTime(new Date(`${todayDate}T15:00:00+09:00`));
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
		await detailDialog.getByTestId('work-record-edit-button').click();
		const firstSegment = detailDialog.getByTestId('team-status-day-segment').first();
		const firstSegmentSummary = firstSegment.locator('[data-slot="work-segment-summary"]');
		const firstSegmentMarker = firstSegmentSummary.locator('[data-slot="work-segment-marker"]');
		await expect(firstSegmentSummary.getByText('재택', { exact: true })).toBeVisible();
		await expect(firstSegmentMarker).toHaveCSS('background-color', 'rgb(59, 130, 246)');
		await firstSegment.getByLabel('출근').fill('08:40');
		await firstSegment.getByLabel('퇴근').fill('10:35');
		await expect(firstSegmentSummary.getByLabel('08:40-10:35')).toBeVisible();
		await expect(firstSegmentSummary.getByLabel('01시간 55분')).toBeVisible();
		await firstSegment.getByLabel('장소').click();
		await page.keyboard.press('Home');
		await page.keyboard.press('Enter');
		await expect(firstSegmentSummary.getByText('사무실', { exact: true })).toBeVisible();
		await expect(firstSegmentMarker).toHaveCSS('background-color', 'rgb(34, 197, 94)');
		const editActions = detailDialog.getByTestId('work-record-edit-actions');
		await expect(editActions.getByRole('button', { name: '저장' })).toBeDisabled();
		await editActions.getByLabel('수정 사유').fill('시간 보정');
		await editActions.getByRole('button', { name: '저장' }).click();

		const updatedFirstSegment = detailDialog.getByTestId('team-status-day-segment').first();
		await expect(updatedFirstSegment.getByLabel('08:40-10:35')).toBeVisible();
		await expect(updatedFirstSegment.getByText('사무실', { exact: true })).toBeVisible();
		await expect(detailDialog.getByText('원본 메시지', { exact: true })).toHaveCount(0);
		await expect(detailDialog.getByTestId('personal-day-detail-panel')).toHaveCount(0);
	});

	test('shows a localized error when saving attendance changes fails', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.clock.setFixedTime(new Date(`${todayDate}T15:00:00+09:00`));
		const summary = buildAttendanceSummaryFixture(todayDate.slice(0, 7));
		const serverErrorMessage = 'attendance event time cannot be in future';

		await page.route('**/attendance/api/events/*', async (route) => {
			await route.fulfill({ status: 400, contentType: 'text/plain', body: serverErrorMessage });
		});

		const detailDialog = await openWorkRecordEditor(page, summary, todayDate);
		const firstSegment = detailDialog.getByTestId('team-status-day-segment').first();
		const editActions = detailDialog.getByTestId('work-record-edit-actions');
		await firstSegment.getByLabel('출근').fill('08:40');
		await editActions.getByLabel('수정 사유').fill('저장 실패 검증');
		await editActions.getByRole('button', { name: '저장' }).click();

		await expect(detailDialog.getByText('처리 실패', { exact: true })).toBeVisible();
		await expect(detailDialog.getByText(serverErrorMessage, { exact: true })).toHaveCount(0);
	});

	test('keeps save disabled while an attendance time input is empty', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.clock.setFixedTime(new Date(`${todayDate}T15:00:00+09:00`));
		const summary = buildAttendanceSummaryFixture(todayDate.slice(0, 7));
		const detailDialog = await openWorkRecordEditor(page, summary, todayDate);
		const clockInInput = detailDialog.getByTestId('team-status-day-segment').first().getByLabel('출근');
		const editActions = detailDialog.getByTestId('work-record-edit-actions');
		const saveButton = editActions.getByRole('button', { name: '저장' });

		await clockInInput.fill('');
		await expect(clockInInput).toHaveValue('');
		await editActions.getByLabel('수정 사유').fill('빈 시각 검증');
		await expect(saveButton).toBeDisabled();

		await clockInInput.fill('08:40');
		await expect(saveButton).toBeEnabled();
	});

	test('falls back future attendance input to the current workspace minute', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.clock.setFixedTime(new Date(`${todayDate}T15:00:00+09:00`));
		const baseSummary = buildAttendanceSummaryFixture(todayDate.slice(0, 7));
		const openClockInEvent = baseSummary.events
			.filter(
				(event) =>
					event.email === baseSummary.currentUserEmail && event.localDate === todayDate && event.kind === 'clock_in'
			)
			.at(-1);
		if (!openClockInEvent) throw new Error('Expected an open current user clock-in event fixture');
		const summary: AttendanceSummary = {
			...baseSummary,
			events: [
				...baseSummary.events,
				{
					...openClockInEvent,
					id: 'future-fallback-clock-out',
					kind: 'clock_out',
					occurredAt: `${todayDate}T14:00:00+09:00`,
					localTime: '14:00',
					resultPostID: 'future-fallback-clock-out-post',
					sourceMessage: '외부 일정 종료',
					parsedAs: { ...openClockInEvent.parsedAs, kind: 'clock_out' }
				}
			]
		};
		const patchRequests: Array<{
			pathname: string;
			payload: ReturnType<typeof parseUpdateAttendanceEventRequest>;
		}> = [];

		await page.route('**/attendance/api/events/*', async (route) => {
			const request = route.request();
			if (request.method() === 'PATCH') {
				patchRequests.push({
					pathname: new URL(request.url()).pathname,
					payload: parseUpdateAttendanceEventRequest(request.postData())
				});
			}
			await route.fulfill({ json: { ok: true } });
		});

		const detailDialog = await openWorkRecordEditor(page, summary, todayDate);
		const lastSegment = detailDialog.getByTestId('team-status-day-segment').nth(2);
		const clockOutInput = lastSegment.getByLabel('퇴근');
		const editActions = detailDialog.getByTestId('work-record-edit-actions');

		await expect(lastSegment.locator('input[type="time"]')).toHaveCount(2);
		await expect(clockOutInput).toHaveValue('14:00');
		await expect(clockOutInput).toHaveAttribute('max', '15:00');
		await clockOutInput.fill('16:30');
		await expect(clockOutInput).toHaveValue('15:00');
		await expect(detailDialog.getByText('현재보다 미래 시각으로 근태를 수정할 수 없습니다.')).toHaveCount(0);
		await editActions.getByLabel('수정 사유').fill('미래 시각 보정');
		await editActions.getByRole('button', { name: '저장' }).click();

		expect(patchRequests).toEqual([
			{
				pathname: '/attendance/api/events/future-fallback-clock-out',
				payload: expect.objectContaining({
					localDate: todayDate,
					localTime: '15:00',
					reason: '미래 시각 보정'
				})
			}
		]);
		await expect(detailDialog.getByTestId('work-record-edit-actions')).toHaveCount(0);
	});

	test('limits overnight event times by each event date', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-02T15:00:00+09:00'));
		const summary = buildOvernightAttendanceSummary('2026-06');
		const detailDialog = await openWorkRecordEditor(page, summary, '2026-06-02');
		const segment = detailDialog.getByTestId('team-status-day-segment');
		const clockInInput = segment.getByLabel('출근');
		const clockOutInput = segment.getByLabel('퇴근');

		await expect(clockInInput).toHaveValue('22:00');
		await expect(clockInInput).not.toHaveAttribute('max');
		await expect(clockOutInput).toHaveAttribute('max', '15:00');
	});

	test('refreshes allowed times while the attendance editor stays open', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.clock.install({ time: new Date(`${todayDate}T15:58:00+09:00`) });
		await page.clock.setSystemTime(new Date(`${todayDate}T15:59:00+09:00`));
		const summary = buildAttendanceSummaryFixture(todayDate.slice(0, 7));
		const detailDialog = await openWorkRecordEditor(page, summary, todayDate);
		const clockOutInput = detailDialog
			.getByTestId('team-status-day-segment')
			.nth(1)
			.getByLabel('퇴근');

		await expect(clockOutInput).toHaveAttribute('max', '15:59');
		await page.clock.runFor('01:00');
		await expect(clockOutInput).toHaveAttribute('max', '16:00');
	});

	test('keeps an existing attendance time until the user edits it', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.clock.setFixedTime(new Date(`${todayDate}T15:00:00+09:00`));
		const baseSummary = buildAttendanceSummaryFixture(todayDate.slice(0, 7));
		const futureEvent = baseSummary.events
			.filter(
				(event) =>
					event.email === baseSummary.currentUserEmail && event.localDate === todayDate && event.kind === 'clock_in'
			)
			.at(-1);
		if (!futureEvent) throw new Error('Expected a current user clock-in event fixture');
		const summary = {
			...baseSummary,
			events: baseSummary.events.map((event) =>
				event.id === futureEvent.id ? { ...event, localTime: '16:30:00' } : event
			)
		};

		const detailDialog = await openWorkRecordEditor(page, summary, todayDate);
		const futureSegment = detailDialog.getByTestId('team-status-day-segment').nth(2);
		const clockInInput = futureSegment.getByLabel('출근');
		const editActions = detailDialog.getByTestId('work-record-edit-actions');

		await expect(clockInInput).toHaveValue('16:30');
		await expect(editActions.getByRole('button', { name: '저장' })).toBeDisabled();
		await expect(detailDialog.getByText('현재보다 미래 시각으로 근태를 수정할 수 없습니다.')).toHaveCount(0);
		await clockInInput.fill('16:45');
		await expect(clockInInput).toHaveValue('15:00');
		await editActions.getByLabel('수정 사유').fill('기존 미래 시각 교정');
		await expect(editActions.getByRole('button', { name: '저장' })).toBeEnabled();
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
