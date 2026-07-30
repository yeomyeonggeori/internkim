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
		await expect(firstDayDialog.getByLabel('22:00-24:00')).toBeVisible();
		await expect(firstDayDialog.getByText('진행 중')).toHaveCount(0);

		await page.keyboard.press('Escape');
		await page.getByTestId('team-status-cell-kim@example.com-2026-06-02').click();
		const secondDayDialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(secondDayDialog.getByLabel('00:00-02:00')).toBeVisible();
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
		await expect(firstDayDialog.getByLabel('22:00-24:00')).toBeVisible();
		await expect(firstDayDialog.getByText('진행 중')).toHaveCount(0);
		await page.keyboard.press('Escape');

		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();
		const secondDayDialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(secondDayDialog.getByLabel('00:00-01:00')).toBeVisible();
		await expect(secondDayDialog.getByTestId('team-status-day-segment').getByLabel('01시간 00분')).toBeVisible();
	});

	test('updates an attendance event only after saving an override reason', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		const currentTime = new Date(`${todayDate}T15:00:00+09:00`);
		await page.clock.setFixedTime(currentTime);
		let summary = buildAttendanceSummaryFixture(todayDate.slice(0, 7), currentTime);
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
		const currentTime = new Date(`${todayDate}T15:00:00+09:00`);
		await page.clock.setFixedTime(currentTime);
		const summary = buildAttendanceSummaryFixture(todayDate.slice(0, 7), currentTime);
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
		const currentTime = new Date(`${todayDate}T15:00:00+09:00`);
		await page.clock.setFixedTime(currentTime);
		const summary = buildAttendanceSummaryFixture(todayDate.slice(0, 7), currentTime);
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

	test('uses server time when the browser clock is ahead', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.clock.setFixedTime(new Date(`${todayDate}T16:00:00+09:00`));
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
			serverTime: `${todayDate}T15:00:00+09:00`,
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
		await clockOutInput.fill('17:30');
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

	test('uses server time when the browser clock is behind', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.clock.setFixedTime(new Date(`${todayDate}T14:00:00+09:00`));
		const summary = {
			...buildAttendanceSummaryFixture(todayDate.slice(0, 7)),
			serverTime: `${todayDate}T15:00:00+09:00`
		};
		const detailDialog = await openWorkRecordEditor(page, summary, todayDate);
		const clockOutInput = detailDialog
			.getByTestId('team-status-day-segment')
			.nth(1)
			.getByLabel('퇴근');

		await expect(clockOutInput).toHaveAttribute('max', '15:00');
		await clockOutInput.fill('14:30');
		await expect(clockOutInput).toHaveValue('14:30');
	});

	test('resynchronizes server time after the browser clock changes and regains focus', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.clock.install({ time: new Date(`${todayDate}T14:00:00+09:00`) });
		let summary = {
			...buildAttendanceSummaryFixture(todayDate.slice(0, 7)),
			serverTime: `${todayDate}T15:00:00+09:00`
		};
		let summaryRequestCount = 0;

		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			summaryRequestCount += 1;
			await route.fulfill({ json: summary });
		});
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();

		const detailDialog = page.getByTestId('team-status-day-detail-dialog');
		await detailDialog.getByTestId('work-record-edit-button').click();
		const clockOutInput = detailDialog
			.getByTestId('team-status-day-segment')
			.nth(1)
			.getByLabel('퇴근');
		await expect(clockOutInput).toHaveAttribute('max', '15:00');

		await page.clock.setSystemTime(new Date(`${todayDate}T18:00:00+09:00`));
		await expect(clockOutInput).toHaveAttribute('max', '15:00');

		const initialRequestCount = summaryRequestCount;
		summary = { ...summary, serverTime: `${todayDate}T16:00:00+09:00` };
		await page.evaluate(() => window.dispatchEvent(new Event('focus')));

		await expect.poll(() => summaryRequestCount).toBe(initialRequestCount + 1);
		await expect(clockOutInput).toHaveAttribute('max', '16:00');
	});

	test('closes attendance editing when a refreshed server clock is invalid', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		const currentTime = new Date(`${todayDate}T15:00:00+09:00`);
		let summary = buildAttendanceSummaryFixture(todayDate.slice(0, 7), currentTime);
		let summaryRequestCount = 0;

		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			summaryRequestCount += 1;
			await route.fulfill({ json: summary });
		});
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();

		const detailDialog = page.getByTestId('team-status-day-detail-dialog');
		const editButton = detailDialog.getByTestId('work-record-edit-button');
		await editButton.click();
		const editActions = detailDialog.getByTestId('work-record-edit-actions');
		await detailDialog.getByTestId('team-status-day-segment').first().getByLabel('출근').fill('08:40');
		await editActions.getByLabel('수정 사유').fill('서버 시각 무효화 검증');
		await expect(editActions.getByRole('button', { name: '저장' })).toBeEnabled();
		await expect(editButton).toHaveAccessibleName('취소');
		await expect(editButton).toBeEnabled();

		const initialRequestCount = summaryRequestCount;
		summary = { ...summary, serverTime: 'invalid' };
		await page.evaluate(() => window.dispatchEvent(new Event('focus')));

		await expect.poll(() => summaryRequestCount).toBe(initialRequestCount + 1);
		await expect(detailDialog.getByTestId('work-record-edit-actions')).toHaveCount(0);
		await expect(editButton).toHaveAccessibleName('수정');
		await expect(editButton).toBeDisabled();
	});

	test('recovers attendance editing after a legacy server is replaced', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		const currentTime = new Date(`${todayDate}T15:00:00+09:00`);
		const currentSummary = buildAttendanceSummaryFixture(todayDate.slice(0, 7), currentTime);
		const { serverTime: _serverTime, ...legacySummary } = currentSummary;
		let summary: Omit<AttendanceSummary, 'serverTime'> | AttendanceSummary = legacySummary;
		let summaryRequestCount = 0;

		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			summaryRequestCount += 1;
			await route.fulfill({ json: summary });
		});
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();

		const editButton = page.getByTestId('team-status-day-detail-dialog').getByTestId('work-record-edit-button');
		await expect(editButton).toBeDisabled();

		const initialRequestCount = summaryRequestCount;
		summary = currentSummary;
		await page.evaluate(() => window.dispatchEvent(new Event('focus')));

		await expect.poll(() => summaryRequestCount).toBe(initialRequestCount + 1);
		await expect(editButton).toBeEnabled();
	});

	test('limits overnight event times by each event date', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-02T15:00:00+09:00'));
		const summary = {
			...buildOvernightAttendanceSummary('2026-06'),
			serverTime: '2026-06-02T15:00:00+09:00'
		};
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
		const summary = buildAttendanceSummaryFixture(
			todayDate.slice(0, 7),
			new Date(`${todayDate}T15:59:00+09:00`)
		);
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
		const currentTime = new Date(`${todayDate}T15:00:00+09:00`);
		await page.clock.setFixedTime(currentTime);
		const baseSummary = buildAttendanceSummaryFixture(todayDate.slice(0, 7), currentTime);
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

});
