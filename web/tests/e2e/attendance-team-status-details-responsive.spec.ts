import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import { expect, test } from './attendance-page-test-fixture';
import { overflowTargetDate, routeOverflowStatusDay, routePersonalDayContext } from './attendance-team-status-fixtures';
import { selectKorean, todayDateInSeoul } from './attendance-test-helpers';

test.describe('attendance team status responsive details', () => {
	test('uses the desktop side sheet as the only scroll area for long status day details', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
		await routeOverflowStatusDay(page);

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${overflowTargetDate}`).click();

		const sheet = page.getByTestId('team-status-day-detail-dialog');
		await expect(sheet).toBeVisible();
		const sheetMetrics = await sheet.evaluate((element) => ({
			clientHeight: element.clientHeight,
			scrollHeight: element.scrollHeight,
			overflowY: getComputedStyle(element).overflowY
		}));
		expect(sheetMetrics.overflowY).toBe('auto');
		expect(sheetMetrics.scrollHeight).toBeGreaterThan(sheetMetrics.clientHeight);

		const sectionLists = [
			sheet.getByTestId('team-status-work-record-list'),
			sheet.getByTestId('team-status-calendar-event-list'),
			sheet.getByTestId('team-status-completed-task-list')
		];
		for (const sectionList of sectionLists) {
			const metrics = await sectionList.evaluate((element) => ({
				clientHeight: element.clientHeight,
				scrollHeight: element.scrollHeight
			}));
			expect(metrics.scrollHeight).toBeLessThanOrEqual(metrics.clientHeight + 1);
		}

		const workRecordCardStyle = await sheet.getByTestId('team-status-day-segment').first().evaluate((element) => {
			const style = getComputedStyle(element);
			return {
				backgroundColor: style.backgroundColor,
				borderWidth: style.borderWidth,
				boxShadow: style.boxShadow
			};
		});
		expect(workRecordCardStyle.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
		expect(workRecordCardStyle.borderWidth).toBe('1px');
		expect(workRecordCardStyle.boxShadow).not.toBe('none');
		const segmentMarkerStyle = await sheet.locator('[data-slot="work-segment-marker"]').first().evaluate((element) => {
			const style = getComputedStyle(element);
			return {
				backgroundColor: style.backgroundColor,
				height: style.height,
				width: style.width
			};
		});
		expect(segmentMarkerStyle.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
		expect(segmentMarkerStyle.height).toBe('12px');
		expect(segmentMarkerStyle.width).toBe('4px');

		const calendarEventCard = sheet.getByTestId('team-status-calendar-event-card').first();
		const eventCardStyle = await calendarEventCard.evaluate((element) => {
			const style = getComputedStyle(element);
			return {
				paddingBlockStart: style.paddingBlockStart,
				paddingBlockEnd: style.paddingBlockEnd
			};
		});
		const eventContentStyle = await calendarEventCard.locator('[data-slot="card-content"]').evaluate((element) => {
			const style = getComputedStyle(element);
			return {
				paddingInlineStart: style.paddingInlineStart,
				paddingInlineEnd: style.paddingInlineEnd
			};
		});
		const eventRowStyle = await calendarEventCard.getByTestId('team-status-calendar-event').evaluate((element) => {
			const style = getComputedStyle(element);
			return {
				backgroundColor: style.backgroundColor,
				boxShadow: style.boxShadow,
				borderRadius: style.borderRadius,
				paddingBlockStart: style.paddingBlockStart,
				paddingBlockEnd: style.paddingBlockEnd
			};
		});
		expect(eventCardStyle.paddingBlockStart).toBe('0px');
		expect(eventCardStyle.paddingBlockEnd).toBe('0px');
		expect(eventContentStyle.paddingInlineStart).toBe('0px');
		expect(eventContentStyle.paddingInlineEnd).toBe('0px');
		expect(eventRowStyle.backgroundColor).toBe('rgba(0, 0, 0, 0)');
		expect(eventRowStyle.boxShadow).toBe('none');
		expect(eventRowStyle.borderRadius).toBe('0px');
		expect(eventRowStyle.paddingBlockStart).toBe('8px');
		expect(eventRowStyle.paddingBlockEnd).toBe('8px');
	});


	test('uses the bottom sheet as the only mobile scroll area for long status day details', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 640 });
		await routeOverflowStatusDay(page);

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('tab', { name: '팀 현황' }).click();
		await page.getByTestId(`team-status-cell-kim@example.com-${overflowTargetDate}`).click();

		const sheet = page.getByTestId('team-status-day-detail-sheet');
		await expect(sheet).toBeVisible();
		const sheetMetrics = await sheet.evaluate((element) => ({
			clientHeight: element.clientHeight,
			scrollHeight: element.scrollHeight,
			overflowY: getComputedStyle(element).overflowY
		}));
		expect(sheetMetrics.scrollHeight).toBeGreaterThan(sheetMetrics.clientHeight);
		expect(sheetMetrics.overflowY).toBe('auto');

		const sectionLists = [
			sheet.getByTestId('team-status-work-record-list'),
			sheet.getByTestId('team-status-calendar-event-list'),
			sheet.getByTestId('team-status-completed-task-list')
		];
		for (const sectionList of sectionLists) {
			const metrics = await sectionList.evaluate((element) => ({
				clientHeight: element.clientHeight,
				scrollHeight: element.scrollHeight,
				overflowY: getComputedStyle(element).overflowY
			}));
			expect(metrics.scrollHeight).toBe(metrics.clientHeight);
			expect(metrics.overflowY).toBe('visible');
		}
	});


	test('opens mobile status day details in a bottom sheet', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		const todayDate = todayDateInSeoul();
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDate.slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});
		await routePersonalDayContext(page, todayDate);

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('tab', { name: '팀 현황' }).click();
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();

		const sheet = page.getByTestId('team-status-day-detail-sheet');
		const segments = sheet.getByTestId('team-status-day-segment');
		await expect(sheet).toBeVisible();
		await expect(sheet.locator('[data-slot="sheet-header"]').getByText('김철수', { exact: true })).toBeVisible();
		await expect(sheet.getByRole('heading', { name: new RegExp(todayDate.slice(0, 4)) })).toBeVisible();
		await expect(sheet.getByTestId('team-status-work-record-header').getByLabel(/\d{2}시간 \d{2}분/)).toBeVisible();
		await expect(segments.filter({ hasText: '재택' })).toBeVisible();
		await expect(segments.filter({ hasText: '사무실' })).toBeVisible();
		await expect(segments.filter({ hasText: '외부' })).toBeVisible();
		await expect(sheet.getByLabel('08:30-10:20')).toBeVisible();
		await expect(sheet.getByLabel('10:45-12:20')).toBeVisible();
		await expect(segments.filter({ hasText: '외부' }).getByLabel(/^12:45-/)).toBeVisible();
		await expect(segments.filter({ hasText: '외부' }).locator('[data-slot="time-range-end"]')).toHaveClass(/text-info/);
		await expect(sheet.getByText('개인 캘린더 일정')).toBeVisible();
		await expect(sheet.getByText('월간 현황 팝업 구현')).toBeVisible();
		await expect(sheet.getByTestId('work-record-edit-button')).toBeVisible();
		await expect(sheet.getByRole('button', { name: '취소' })).toHaveCount(0);
		await expect(page.getByTestId('team-status-day-detail-dialog')).toHaveCount(0);
	});


	test('shows a cancel action for an own absence in mobile status day details', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		const summary = buildAttendanceSummaryFixture('2026-06');
		const targetAbsence = summary.absences.find(
			(absence) => absence.email === summary.currentUserEmail && absence.rangeID === 'absence-june-kim-leave'
		);
		if (!targetAbsence) throw new Error('Expected a current user team absence fixture');

		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? summary.month;
			await route.fulfill({ json: month === summary.month ? summary : buildAttendanceSummaryFixture(month) });
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('tab', { name: '팀 현황' }).click();
		await page.getByTestId(`team-status-cell-kim@example.com-${targetAbsence.date}`).click();

		const sheet = page.getByTestId('team-status-day-detail-sheet');
		await expect(sheet.getByText('휴가').first()).toBeVisible();
		await expect(sheet.getByText('sample overlap leave').first()).toBeVisible();
		await expect(sheet.getByRole('button', { name: '수정' })).toHaveCount(0);
		await expect(sheet.getByTestId('personal-day-detail-panel').getByRole('button', { name: '취소' })).toBeVisible();
	});
});
