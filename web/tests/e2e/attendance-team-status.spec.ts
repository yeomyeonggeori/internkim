import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import { expect, test } from './attendance-page-test-fixture';
import {
	calendarEvent,
	flowStateFixture,
	flowTask,
	overflowTargetDate,
	routeOverflowStatusDay,
	routePersonalDayContext,
	routeSourceSeparatedDayContext
} from './attendance-team-status-fixtures';
import { buildWideTooltipSummary, selectKorean, todayDateInSeoul } from './attendance-test-helpers';

test.describe('attendance team status', () => {
	test('shows registered members even when they have no attendance records', async ({ page }) => {
		const summary = buildAttendanceSummaryFixture('2026-06');
		const registeredMemberWithoutRecords = {
			email: 'no-record@example.com',
			displayName: '무기록',
			mattermostUsername: 'no-record',
		};

		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? summary.month;
			await route.fulfill({
				json: month === summary.month
					? { ...summary, members: [...summary.members, registeredMemberWithoutRecords] }
					: buildAttendanceSummaryFixture(month)
			});
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByPlaceholder('직원 검색').fill(registeredMemberWithoutRecords.displayName);

		await expect(page.getByText(registeredMemberWithoutRecords.displayName)).toBeVisible();
		await expect(page.getByTestId(`team-status-cell-${registeredMemberWithoutRecords.email}-2026-06-16`)).toBeVisible();
	});

	test('marks the today row header with an inverted color treatment', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		const todayDate = todayDateInSeoul();
		const otherDate = todayDate.endsWith('-01') ? `${todayDate.slice(0, 8)}02` : `${todayDate.slice(0, 8)}01`;
		await page.goto('/attendance');
		await selectKorean(page);

		const todayRowHeader = page.getByTestId(`team-status-day-${todayDate}`);
		const otherRowHeader = page.getByTestId(`team-status-day-${otherDate}`);

		await expect(todayRowHeader).toBeVisible();
		const [todayStyle, otherStyle] = await Promise.all([
			todayRowHeader.evaluate((element) => getComputedStyle(element).backgroundColor),
			otherRowHeader.evaluate((element) => getComputedStyle(element).backgroundColor)
		]);
		expect(todayStyle).not.toBe('rgba(0, 0, 0, 0)');
		expect(todayStyle).not.toBe(otherStyle);
	});

	test('keeps the today row visible while scrolling past it in both directions', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-25T09:00:00+09:00'));
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			await route.fulfill({ json: buildAttendanceSummaryFixture('2026-06') });
		});

		await page.goto('/attendance');
		await selectKorean(page);

		const statusTable = page.getByTestId('team-status-table');
		const todayRowHeader = page.getByTestId('team-status-day-2026-06-25');
		await expect(todayRowHeader).toBeVisible();

		await statusTable.evaluate((table) => {
			table.scrollTop = 0;
		});
		const containerBoxNearTop = await statusTable.boundingBox();
		const rowBoxNearTop = await todayRowHeader.boundingBox();
		if (!containerBoxNearTop || !rowBoxNearTop) throw new Error('Expected the status table and today row to be measurable');
		expect(rowBoxNearTop.y).toBeGreaterThanOrEqual(containerBoxNearTop.y - 1);
		expect(rowBoxNearTop.y + rowBoxNearTop.height).toBeLessThanOrEqual(containerBoxNearTop.y + containerBoxNearTop.height + 1);

		await statusTable.evaluate((table) => {
			table.scrollTop = table.scrollHeight;
		});
		const containerBoxNearBottom = await statusTable.boundingBox();
		const rowBoxNearBottom = await todayRowHeader.boundingBox();
		if (!containerBoxNearBottom || !rowBoxNearBottom) throw new Error('Expected the status table and today row to be measurable');
		expect(rowBoxNearBottom.y).toBeGreaterThanOrEqual(containerBoxNearBottom.y - 1);
		expect(rowBoxNearBottom.y + rowBoxNearBottom.height).toBeLessThanOrEqual(containerBoxNearBottom.y + containerBoxNearBottom.height + 1);

		const otherVisibleRowHeader = page.getByTestId('team-status-day-2026-06-30');
		await expect(otherVisibleRowHeader).toBeVisible();
		const otherRowText = await otherVisibleRowHeader.textContent();
		expect(otherRowText?.trim()).toContain('6/30');
	});

	test('shows location bars without a hover tooltip in a team status cell', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		const todayDate = todayDateInSeoul();
		await page.goto('/attendance');
		await selectKorean(page);

		const todayCell = page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`);
		await expect(todayCell.getByText(/\d+시간/)).toBeVisible();
		await expect(todayCell.getByText('외부')).toHaveCount(0);
		await expect(todayCell.getByText('근무 중')).toHaveCount(0);
		await expect(todayCell.locator('span[style*="background-color"]')).toHaveCount(3);
		await todayCell.hover();
		await expect(page.locator('[data-slot="tooltip-content"]')).toHaveCount(0);
	});

	test('opens status day details from a team status cell', async ({ page }) => {
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

		const dialog = page.getByTestId('team-status-day-detail-dialog');
		const segments = dialog.getByTestId('team-status-day-segment');
		await expect(dialog.getByText('김철수')).toBeVisible();
		await expect(dialog.getByText('상태')).toHaveCount(0);
		await expect(dialog.getByText('근무 구간')).toHaveCount(0);
		const workHeader = dialog.getByTestId('team-status-work-record-header');
		await expect(workHeader.getByText('근무 기록', { exact: true })).toBeVisible();
		await expect(workHeader.getByText(/\d+시간/)).toBeVisible();
		await expect(segments.filter({ hasText: '재택' })).toBeVisible();
		await expect(segments.filter({ hasText: '사무실' })).toBeVisible();
		await expect(segments.filter({ hasText: '외부' })).toBeVisible();
		await expect(dialog.getByText('08:30-10:20')).toBeVisible();
		await expect(dialog.getByText('10:45-12:20')).toBeVisible();
		await expect(dialog.getByText('12:45~')).toBeVisible();
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
		await expect(dialog.getByText('캘린더 일정', { exact: true })).toBeVisible();
		await expect(dialog.getByText('완료된 업무', { exact: true })).toBeVisible();
		await expect(dialog.getByText('개인 캘린더 일정')).toBeVisible();
		await expect(dialog.getByText('전체 캘린더 일정')).toHaveCount(0);
		await expect(dialog.getByText('다른 사람 일정')).toHaveCount(0);
		await expect(dialog.getByText('월간 현황 팝업 구현')).toBeVisible();
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
			sheet.getByTestId('personal-day-detail-panel'),
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

		const cardStyle = await sheet.getByTestId('team-status-calendar-event').first().evaluate((element) => {
			const style = getComputedStyle(element);
			return {
				backgroundColor: style.backgroundColor,
				boxShadow: style.boxShadow
			};
		});
		expect(cardStyle.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
		expect(cardStyle.boxShadow).not.toBe('none');
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

	test('opens mobile status day details in a read-only bottom sheet', async ({ page }) => {
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
		await expect(sheet.getByText(`김철수 · ${todayDate}`)).toBeVisible();
		await expect(sheet.getByTestId('team-status-work-record-header').getByText(/\d+시간/)).toBeVisible();
		await expect(segments.filter({ hasText: '재택' })).toBeVisible();
		await expect(segments.filter({ hasText: '사무실' })).toBeVisible();
		await expect(segments.filter({ hasText: '외부' })).toBeVisible();
		await expect(sheet.getByText('08:30-10:20')).toBeVisible();
		await expect(sheet.getByText('10:45-12:20')).toBeVisible();
		await expect(sheet.getByText('12:45~')).toBeVisible();
		await expect(sheet.getByText('개인 캘린더 일정')).toBeVisible();
		await expect(sheet.getByText('월간 현황 팝업 구현')).toBeVisible();
		await expect(sheet.getByRole('button', { name: '수정' })).toHaveCount(0);
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
