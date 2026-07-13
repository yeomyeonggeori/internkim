import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import { expect, test } from './attendance-page-test-fixture';
import { calendarEvent, flowStateFixture, flowTask, routeSourceSeparatedDayContext } from './attendance-team-status-fixtures';
import { selectKorean, todayDateInSeoul } from './attendance-test-helpers';

test.describe('attendance team status details', () => {
	test('opens status day details from a team status cell', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDate.slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({ json: { events: [] } });
		});
		await page.unroute('**/flow/api/state');
		await page.route('**/flow/api/state', async (route) => {
			await route.fulfill({ json: flowStateFixture([]) });
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();

		const dialog = page.getByTestId('team-status-day-detail-dialog');
		const segments = dialog.getByTestId('team-status-day-segment');
		await expect(dialog.locator('[data-slot="sheet-header"]').getByText('김철수', { exact: true })).toBeVisible();
		await expect(dialog.getByRole('heading', { name: new RegExp(todayDate.slice(0, 4)) })).toBeVisible();
		await expect(dialog.getByText('상태')).toHaveCount(0);
		await expect(dialog.getByText('근무 구간')).toHaveCount(0);
		const workHeader = dialog.getByTestId('team-status-work-record-header');
		await expect(workHeader.getByText('근무 기록', { exact: true })).toBeVisible();
		await expect(workHeader.getByLabel(/\d{2}시간 \d{2}분/)).toHaveClass(/text-info/);
		await expect(segments.filter({ hasText: '재택' })).toBeVisible();
		await expect(segments.filter({ hasText: '사무실' })).toBeVisible();
		const activeSegment = segments.filter({ hasText: '외부' });
		await expect(activeSegment).toBeVisible();
		await expect(activeSegment.getByText('근무 중', { exact: true })).toHaveCount(0);
		await expect(activeSegment.getByLabel(/^12:45-/)).toBeVisible();
		const activeTimeRange = activeSegment.locator('[data-slot="time-range-text"]');
		await expect(activeTimeRange).toHaveClass(/font-mono/);
		const timeRangeSeparator = activeTimeRange.locator('[data-slot="time-range-separator"]');
		await expect(timeRangeSeparator).toHaveClass(/mx-0.5/);
		await expect(timeRangeSeparator).toHaveCSS('opacity', '0.4');
		await expect(activeTimeRange.locator('[data-slot="time-range-end"]')).toHaveClass(/text-info/);
		const activeDuration = activeSegment.getByLabel(/\d{2}시간 \d{2}분/);
		await expect(activeDuration).toHaveClass(/text-info/);
		await expect(activeDuration.locator(':scope > span').first()).toHaveClass(/gap-0.5/);
		const durationToneStyles = await activeDuration.evaluate((element) => {
			const leadingZero = element.querySelector<HTMLElement>('[data-slot="duration-digit"].opacity-40');
			const unit = element.querySelector<HTMLElement>('[data-slot="duration-unit"]');
			if (!leadingZero || !unit) throw new Error('Expected a leading zero and duration unit');
			return {
				baseColor: getComputedStyle(element).color,
				leadingZeroColor: getComputedStyle(leadingZero).color,
				leadingZeroOpacity: getComputedStyle(leadingZero).opacity,
				unitColor: getComputedStyle(unit).color,
				unitOpacity: getComputedStyle(unit).opacity
			};
		});
		expect(durationToneStyles.leadingZeroColor).toBe(durationToneStyles.baseColor);
		expect(durationToneStyles.leadingZeroOpacity).toBe('0.4');
		expect(durationToneStyles.unitColor).toBe(durationToneStyles.baseColor);
		expect(durationToneStyles.unitOpacity).toBe('0.6');
		await expect(dialog.getByLabel('08:30-10:20')).toBeVisible();
		await expect(dialog.getByLabel('10:45-12:20')).toBeVisible();
		await expect(dialog.getByTestId('calendar-empty-state')).toHaveAttribute('data-slot', 'empty');
		await expect(dialog.getByTestId('calendar-empty-state').getByText('해당 일정 없음')).toBeVisible();
		await expect(dialog.getByTestId('completed-work-empty-state')).toHaveAttribute('data-slot', 'empty');
		await expect(dialog.getByTestId('completed-work-empty-state').getByText('완료된 업무 없음')).toBeVisible();
	});


	test('shows personal calendar events and completed work in status day details', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
		const targetDate = '2026-06-17';
		const summary = buildAttendanceSummaryFixture('2026-06');
		let calendarRequests = 0;
		let flowRequests = 0;
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			await route.fulfill({ json: summary });
		});
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			calendarRequests += 1;
			await route.fulfill({
				json: {
					events: [
						calendarEvent('personal-calendar-event', '개인 캘린더 일정', targetDate, [
							{ personID: 'kim', name: '김철수', email: 'kim@example.com' }
						]),
						calendarEvent('all-calendar-event', '전체 캘린더 일정', targetDate, [
							{ personID: 'all', name: '전체' }
						]),
						calendarEvent('other-calendar-event', '다른 사람 일정', targetDate, [
							{ personID: 'park', name: '박지민', email: 'park@example.com' }
						])
					]
				}
			});
		});
		await page.unroute('**/flow/api/state');
		await page.route('**/flow/api/state', async (route) => {
			flowRequests += 1;
			await route.fulfill({
				json: flowStateFixture([
					flowTask('completed-personal-task', '월간 현황 팝업 구현', '완료', targetDate, 'kim', ['kim', 'park']),
					flowTask('planned-personal-task', '아직 예정 업무', '예정', targetDate, 'kim', ['kim']),
					flowTask('other-person-task', '다른 사람 완료 업무', '완료', targetDate, 'park', ['park'])
				])
			});
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${targetDate}`).click();

		const dialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(dialog.getByText('근무 기록', { exact: true })).toBeVisible();
		await expect(dialog.getByText('일정', { exact: true })).toBeVisible();
		await expect(dialog.getByText('완료된 업무', { exact: true })).toBeVisible();
		await expect(dialog.getByTestId('team-status-calendar-header').getByTestId('section-count-badge')).toHaveText('1');
		await expect(dialog.getByTestId('team-status-completed-work-header').getByTestId('section-count-badge')).toHaveText('1');
		await expect(dialog.locator('[data-slot="completed-work-section-icon"]')).toHaveClass(/text-muted-foreground/);
		await expect(dialog.getByText('개인 캘린더 일정')).toBeVisible();
		await expect(dialog.getByTestId('team-status-calendar-event-card')).toHaveAttribute('data-slot', 'card');
		const calendarEventItem = dialog.getByTestId('team-status-calendar-event');
		await expect(calendarEventItem.locator('.calendar-event-content')).toBeVisible();
		const calendarTimeRange = calendarEventItem.locator('[data-slot="time-range-text"]');
		await expect(calendarTimeRange).toHaveAccessibleName('10:00-11:00');
		await expect(calendarTimeRange).toHaveClass(/font-mono/);
		await expect(calendarTimeRange.locator('[data-slot="time-value-separator"]')).toHaveCount(2);
		await expect(calendarTimeRange.locator('[data-slot="time-value-separator"]').first()).toHaveCSS('opacity', '0.4');
		await expect(calendarTimeRange.locator('[data-slot="time-range-separator"]')).toHaveClass(/mx-0.5/);
		await expect(calendarTimeRange.locator('[data-slot="time-range-separator"]')).toHaveCSS('opacity', '0.4');
		await expect(dialog.getByText('전체 캘린더 일정')).toHaveCount(0);
		await expect(dialog.getByText('다른 사람 일정')).toHaveCount(0);
		await expect(dialog.getByText('월간 현황 팝업 구현')).toBeVisible();
		await expect(dialog.locator('[data-flow-board-card="completed-personal-task"]')).toBeVisible();
		const taskDateRange = dialog.locator('[data-flow-board-card="completed-personal-task"] [data-slot="flow-task-date-range"]');
		await expect(taskDateRange).toHaveAccessibleName('06/15 - 06/17');
		await expect(taskDateRange).toHaveClass(/font-mono/);
		await expect(taskDateRange.locator('[data-slot="flow-task-date-separator"]')).toHaveCount(2);
		await expect(taskDateRange.locator('[data-slot="flow-task-date-separator"]').first()).toHaveCSS('opacity', '0.4');
		await expect(taskDateRange.locator('[data-slot="flow-task-date-range-separator"]')).toHaveCSS('opacity', '0.4');
		await expect(dialog.getByText('박지민')).toBeVisible();
		await expect(dialog.getByText('아직 예정 업무')).toHaveCount(0);
		await expect(dialog.getByText('다른 사람 완료 업무')).toHaveCount(0);
		await expect(dialog.getByTestId('team-status-section-scroll-fade')).toHaveCount(0);
		expect(calendarRequests).toBe(1);
		expect(flowRequests).toBe(1);
	});


	test('keeps calendar events and completed flow tasks separated by source', async ({ page }) => {
		const targetDate = '2026-06-16';
		const summary = buildAttendanceSummaryFixture('2026-06');
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			await route.fulfill({ json: summary });
		});
		await routeSourceSeparatedDayContext(page, targetDate);

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${targetDate}`).click();

		const dialog = page.getByTestId('team-status-day-detail-dialog');
		const calendarItems = dialog.getByTestId('team-status-calendar-event');
		const completedTaskItems = dialog.getByTestId('team-status-completed-task');
		await expect(calendarItems.filter({ hasText: '완료 업무처럼 보이는 캘린더' })).toBeVisible();
		await expect(calendarItems.filter({ hasText: '캘린더 일정처럼 보이는 완료 업무' })).toHaveCount(0);
		await expect(calendarItems.filter({ hasText: '다른 사람 완료 업무처럼 보이는 캘린더' })).toHaveCount(0);
		await expect(completedTaskItems.filter({ hasText: '캘린더 일정처럼 보이는 완료 업무' })).toBeVisible();
		await expect(completedTaskItems.filter({ hasText: '완료 업무처럼 보이는 캘린더' })).toHaveCount(0);
		await expect(completedTaskItems.filter({ hasText: '캘린더 일정처럼 보이는 예정 업무' })).toHaveCount(0);
		await expect(completedTaskItems.filter({ hasText: '다른 사람 캘린더 일정처럼 보이는 완료 업무' })).toHaveCount(0);
	});


	test('shows context load failures instead of empty states', async ({ page }) => {
		const targetDate = '2026-06-16';
		const summary = buildAttendanceSummaryFixture('2026-06');
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			await route.fulfill({ json: summary });
		});
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({ status: 500, body: 'calendar failed' });
		});
		await page.unroute('**/flow/api/state');
		await page.route('**/flow/api/state', async (route) => {
			await route.fulfill({ status: 500, body: 'flow failed' });
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${targetDate}`).click();

		const dialog = page.getByTestId('team-status-day-detail-dialog');
		await expect(dialog.getByText('일정을 불러오지 못했습니다.')).toBeVisible();
		await expect(dialog.getByText('완료된 업무를 불러오지 못했습니다.')).toBeVisible();
		await expect(dialog.getByText('해당 일정 없음')).toHaveCount(0);
		await expect(dialog.getByText('완료된 업무 없음')).toHaveCount(0);
	});

});
