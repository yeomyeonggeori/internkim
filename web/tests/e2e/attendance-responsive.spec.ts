import { expect, test } from './attendance-page-test-fixture';
import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import { createDevAttendanceWorkStatus } from '../../dev-attendance-work-status-mock';
import {
	expectReadableMobileTeamStatusTable,
	measureMobileTeamStatusTable,
	mobileMonthPickerLayout,
	mobileTabListLayout,
	mobileTabStyle,
	withLongMobileDisplayName
} from './attendance-responsive-helpers';
import { buildWideTooltipSummary, selectKorean, todayDateInSeoul } from './attendance-test-helpers';

test.describe('attendance responsive view', () => {
	test('starts on the current month even when a stale selected month was stored', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		const currentMonth = todayDate.slice(0, 7);
		const staleMonth = currentMonth === '2026-06' ? '2026-05' : '2026-06';
		const [year, month] = currentMonth.split('-');
		const currentMonthLabel = `${year}년 ${Number(month)}월`;
		const requestedMonths: Array<string | null> = [];

		await page.addInitScript((storedMonth) => {
			window.localStorage.setItem('attendance.filters', JSON.stringify({ selectedMonth: storedMonth, chartMode: 'month' }));
		}, staleMonth);
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const monthParameter = requestURL.searchParams.get('month');
			requestedMonths.push(monthParameter);
			await route.fulfill({ json: buildAttendanceSummaryFixture(monthParameter ?? currentMonth) });
		});

		await page.goto('/attendance');
		await selectKorean(page);

		expect(requestedMonths[0]).toBeNull();
		await expect(page.getByRole('button', { name: currentMonthLabel }).first()).toBeVisible();
	});

	test('renders the monthly team status table with personal tools in the fixed sidebar', async ({ page }) => {
		await page.goto('/attendance');
		await selectKorean(page);

		await expect(page.getByRole('tab')).toHaveCount(0);
		await expect(page.getByTestId('team-month-calendar')).toHaveCount(0);
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await expect(page.getByTestId('team-status-table')).toBeVisible();
		await expect(page.getByText('월간 근무 현황표')).toBeVisible();
		await expect(page.getByText('내 근무 시간')).toBeVisible();
		await expect(page.getByRole('button', { name: '휴가 등록' })).toBeVisible();

		const statusTable = page.getByTestId('team-status-table');
		await page.getByRole('combobox', { name: '직원 선택' }).click();
		await page.getByPlaceholder('직원 검색').fill('김철수');
		await page.getByRole('option', { name: /김철수/ }).click();
		await expect(statusTable.getByText('김철수')).toBeVisible();
		await expect(statusTable.getByText('강민호')).toHaveCount(0);
	});

	test('updates the personal work standard with the existing chart period controls', async ({ page }) => {
		await page.goto('/attendance');
		await selectKorean(page);

		const workTimeCard = page
			.getByTestId('attendance-sidebar-scroll')
			.locator('[data-slot="card"][aria-label="내 근무 시간"]');
		const standard = workTimeCard.getByTestId('personal-work-standard');
		await expect(workTimeCard.getByText('일일 평균')).toBeVisible();
		await expect(workTimeCard.getByText('일일 최고')).toBeVisible();
		await expect(workTimeCard.getByText('일일 최저')).toBeVisible();
		await expect(standard.getByText('근무 기준')).toBeVisible();
		await expect(standard.getByText('휴가', { exact: true })).toBeVisible();
		await expect(standard.getByText('휴가 인정')).toHaveCount(0);
		await expect(standard.getByText('남은 시간')).toHaveCount(0);
		await expect(standard.getByText('유급 휴가')).toHaveCount(0);
		await expect(standard.getByText('야간 근무', { exact: true })).toBeVisible();
		await expect(standard.getByText('기준 초과', { exact: true })).toBeVisible();
		await expect(standard.getByText('기준 부족', { exact: true })).toBeVisible();
		await expect(standard.getByRole('status')).toHaveCount(0);
		await expect(standard.getByText('실제 근무', { exact: true })).toBeVisible();
		await expect(standard.getByText('실제 근무 01시간 20분')).toHaveCount(0);

		await workTimeCard.getByRole('button', { name: '일별' }).click();
		await expect(standard.getByText(/2026-07-31/)).toBeVisible();
		const capacityBar = standard.getByTestId('work-standard-capacity-bar');
		await expect(capacityBar).toHaveAttribute(
			'aria-label',
			/실제 근무 01시간 20분, 휴가 08시간 00분, 휴가 인정 06시간 40분, 기준 시간 08시간 00분/
		);
		await expect(standard.getByTestId('work-standard-total')).toHaveText('09시간 20분');
		await expect(
			standard.getByTestId('work-standard-uncredited-leave').getByLabel('01시간 20분')
		).toBeVisible();
		await expect(capacityBar).not.toHaveAttribute('aria-label', /전체 시간|남은 시간/);
		await expect(standard.getByTestId('work-standard-target-marker')).toHaveAttribute(
			'style',
			/left:\s*80%/
		);
		await expect(standard.getByTestId('work-standard-progress-track')).toHaveCSS('height', '14px');
		await expect(standard.getByTestId('work-standard-actual-segment')).toHaveClass(/bg-yellow-400/);
		await expect(page.getByTestId('work-standard-bar-tooltip')).toHaveCount(0);
		await workTimeCard.getByRole('button', { name: '주별' }).click();
		await expect(standard.getByText(/2026-07-27–2026-08-02/)).toBeVisible();
		await expect(standard.getByTestId('work-standard-capacity-bar')).toHaveAttribute(
			'aria-label',
			/실제 근무 33시간 20분, 휴가 08시간 00분, 휴가 인정 06시간 40분, 기준 시간 40시간 00분/
		);
		await expect(standard.getByTestId('work-standard-target-marker')).toHaveAttribute('style', /left:\s*80%/);
		await workTimeCard.getByRole('button', { name: '월별' }).click();
		await expect(standard.getByText(/2026-05-01–2026-05-31/)).toBeVisible();
		await expect(standard.getByTestId('work-standard-capacity-bar')).toHaveAttribute(
			'aria-label',
			/실제 근무 161시간 20분, 휴가 08시간 00분, 휴가 인정 06시간 40분, 기준 시간 168시간 00분/
		);
		await expect(standard.getByTestId('work-standard-target-marker')).toHaveAttribute('style', /left:\s*80%/);
	});

	test('distinguishes four work and leave baseline states', async ({ page }) => {
		const scenarios = [
			{ actualMinutes: 360, paidLeaveMinutes: 0, creditedLeaveMinutes: 0, fulfilledMinutes: 360, remainingMinutes: 120, overtimeMinutes: 0, differenceMinutes: -120, total: '06시간 00분', shortfall: '02시간 00분' },
			{ actualMinutes: 300, paidLeaveMinutes: 120, creditedLeaveMinutes: 120, fulfilledMinutes: 420, remainingMinutes: 60, overtimeMinutes: 0, differenceMinutes: -60, total: '07시간 00분', shortfall: '01시간 00분' },
			{ actualMinutes: 540, paidLeaveMinutes: 0, creditedLeaveMinutes: 0, fulfilledMinutes: 480, remainingMinutes: 0, overtimeMinutes: 60, differenceMinutes: 0, total: '09시간 00분', overtime: '01시간 00분' },
			{ actualMinutes: 420, paidLeaveMinutes: 120, creditedLeaveMinutes: 60, fulfilledMinutes: 480, remainingMinutes: 0, overtimeMinutes: 0, differenceMinutes: 0, total: '09시간 00분', uncreditedLeave: '01시간 00분' }
		];
		let scenario = scenarios[0];
		await page.unroute('**/attendance/api/work-status?**');
		await page.route('**/attendance/api/work-status?**', async (route) => {
			const requestURL = new URL(route.request().url());
			const payload = createDevAttendanceWorkStatus(
				'tester@example.com',
				requestURL.searchParams.get('period'),
				requestURL.searchParams.get('anchor')
			);
			payload.personal = payload.personal ? { ...payload.personal, ...scenario, targetMinutes: 480, nightMinutes: 0 } : undefined;
			await route.fulfill({ json: payload });
		});

		for (const currentScenario of scenarios) {
			scenario = currentScenario;
			await page.goto('/attendance');
			await selectKorean(page);
			const standard = page
				.getByTestId('attendance-sidebar-scroll')
				.getByTestId('personal-work-standard');
			await page
				.getByTestId('attendance-sidebar-scroll')
				.getByRole('button', { name: '일별' })
				.click();
			await expect(standard.getByTestId('work-standard-total')).toHaveText(currentScenario.total);
			const overtimeRow = standard.getByText('기준 초과', { exact: true }).locator('..');
			const shortfallRow = standard.getByText('기준 부족', { exact: true }).locator('..');
			await expect(overtimeRow.getByLabel(currentScenario.overtime ?? '00시간 00분')).toBeVisible();
			await expect(shortfallRow.getByLabel(currentScenario.shortfall ?? '00시간 00분')).toBeVisible();
			if (currentScenario.uncreditedLeave) {
				await expect(
					standard
						.getByTestId('work-standard-uncredited-leave')
						.getByLabel(currentScenario.uncreditedLeave)
				).toBeVisible();
			} else {
				await expect(standard.getByTestId('work-standard-uncredited-leave')).toHaveCount(0);
			}
		}
	});

	test('shows the administrator employee work status list and review filters', async ({ page }) => {
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId('employee-work-status-navigation').click();

		const view = page.getByTestId('employee-work-status-view');
		await expect(view.getByText('직원 근무 현황', { exact: true })).toBeVisible();
		await expect(view.getByRole('columnheader', { name: '근무 방식' })).toBeVisible();
		await expect(view.getByRole('columnheader', { name: '기준 충족' })).toBeVisible();
		await expect(view.getByRole('columnheader', { name: '기준 차이' })).toBeVisible();
		await expect(view.getByRole('columnheader', { name: '기준 시간' })).toHaveCount(0);
		await expect(view.getByRole('columnheader', { name: '회사 기준 대비 초과' })).toHaveCount(0);

		await view.getByRole('button', { name: '기록 확인 필요' }).click();
		await expect(view.getByText('최도윤', { exact: true })).toBeVisible();
		await expect(view.getByText('김민지', { exact: true })).toHaveCount(0);

		await view.getByRole('button', { name: '최도윤 상세' }).click();
		await expect(page.getByText('최도윤 · 일별 상세')).toBeVisible();
		await expect(page.getByRole('columnheader', { name: '근무 구간' })).toBeVisible();
		await expect(page.getByRole('columnheader', { name: '휴가 구간' })).toBeVisible();
	});

	test('scrolls desktop sidebar navigation together with personal tools', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 500 });
		await page.goto('/attendance');
		await selectKorean(page);

		const scrollArea = page.getByTestId('attendance-sidebar-scroll');
		const navigation = page.getByTestId('leave-history-navigation');
		const personalTools = page.getByTestId('personal-tools-panel');
		const initialNavigationBox = await navigation.boundingBox();
		const scrollLayout = await scrollArea.evaluate((element) => ({
			clientHeight: element.clientHeight,
			overflowY: window.getComputedStyle(element).overflowY,
			scrollHeight: element.scrollHeight
		}));

		expect(scrollLayout.overflowY).toBe('auto');
		expect(scrollLayout.scrollHeight).toBeGreaterThan(scrollLayout.clientHeight);
		expect(await personalTools.evaluate((element) => window.getComputedStyle(element).overflowY)).toBe('visible');

		await scrollArea.evaluate((element) => element.scrollTo({ top: element.scrollHeight }));
		await expect.poll(() => scrollArea.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);

		const scrolledNavigationBox = await navigation.boundingBox();
		expect(initialNavigationBox).not.toBeNull();
		expect(scrolledNavigationBox).not.toBeNull();
		expect(scrolledNavigationBox!.y).toBeLessThan(initialNavigationBox!.y);
	});

	test('switches between status and tools containers on mobile', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/attendance');
		await selectKorean(page);

		const recordsTab = page.getByRole('tab', { name: '내 기록' });
		const statusTab = page.getByRole('tab', { name: '팀 현황' });

		await expect(recordsTab).toBeVisible();
		await expect(recordsTab).toHaveAttribute('aria-selected', 'true');
		await expect(statusTab).toBeVisible();
		const initialTabStyle = await mobileTabStyle(recordsTab, statusTab);
		expect(initialTabStyle.recordsBackground).not.toBe('rgba(0, 0, 0, 0)');
		expect(initialTabStyle.recordsBackground).not.toBe(initialTabStyle.statusBackground);
		const tabListLayout = await mobileTabListLayout(page.getByRole('tablist'));
		expect(tabListLayout.left).toBeLessThanOrEqual(24);
		expect(tabListLayout.right).toBeLessThanOrEqual(tabListLayout.viewportWidth - 12);
		await expect(page.getByTestId('mobile-attendance-tools-view')).toBeVisible();
		await expect(page.getByTestId('team-status-grid')).toBeHidden();

		await statusTab.click();

		await expect(statusTab).toHaveAttribute('aria-selected', 'true');
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await expect(page.getByTestId('mobile-attendance-tools-view')).toBeHidden();
		const monthPickerLayout = await mobileMonthPickerLayout(page);
		expect(Math.abs(monthPickerLayout.filterRight - monthPickerLayout.nextButtonRight)).toBeLessThanOrEqual(1);
		expect(monthPickerLayout.previousGap).toBeLessThanOrEqual(1);
		expect(monthPickerLayout.nextGap).toBeLessThanOrEqual(1);

		await recordsTab.click();

		await expect(recordsTab).toHaveAttribute('aria-selected', 'true');
		await expect(page.getByTestId('mobile-attendance-tools-view')).toBeVisible();
		await expect(page.getByTestId('team-status-grid')).toBeHidden();
	});

	test('shows personal attendance tools in the mobile records tab', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		const todayDate = todayDateInSeoul();
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDate.slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});
		await page.goto('/attendance');
		await selectKorean(page);

		const toolsView = page.getByTestId('mobile-attendance-tools-view');
		await expect(page.getByRole('tab', { name: '내 기록' })).toHaveAttribute('aria-selected', 'true');
		await expect(toolsView.getByText('내 근무 시간')).toBeVisible();
		await expect(toolsView.getByRole('button', { name: '휴가 등록' })).toBeVisible();
		await expect(toolsView.getByTestId('work-standard-capacity-bar')).toBeVisible();
		await expect(page.getByTestId('work-standard-bar-tooltip')).toHaveCount(0);

		await page.getByRole('tab', { name: '팀 현황' }).click();
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();

		const detailSheet = page.getByTestId('team-status-day-detail-sheet');
		await expect(detailSheet.getByTestId('personal-day-detail-panel')).toHaveCount(0);
		await detailSheet.getByTestId('work-record-edit-button').click();
		await expect(detailSheet.locator('[data-slot="work-segment-edit-fields"]')).toHaveCount(3);
		await expect(detailSheet.getByTestId('work-record-edit-actions')).toBeVisible();
	});

	test('keeps the mobile monthly status table readable while scrolling through dates', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const todayDate = todayDateInSeoul();
			const month = requestURL.searchParams.get('month') ?? todayDate.slice(0, 7);
			const summary = buildWideTooltipSummary(month, todayDate);
			await route.fulfill({ json: withLongMobileDisplayName(summary) });
		});

		await page.setViewportSize({ width: 390, height: 844 });
		const todayDate = todayDateInSeoul();
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('tab', { name: '팀 현황' }).click();

		await expect(page.getByTestId('team-status-table')).toBeVisible();
		await expect(page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`)).toBeVisible();
		await expect(page.getByRole('combobox', { name: '직원 선택' })).toBeVisible();

		const layout = await measureMobileTeamStatusTable(page.getByTestId('team-status-table'), todayDate);

		expectReadableMobileTeamStatusTable(layout);
	});

	test('does not show status cell hover tooltip during mobile table scrolling', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const todayDate = todayDateInSeoul();
			const month = requestURL.searchParams.get('month') ?? todayDate.slice(0, 7);
			await route.fulfill({ json: buildWideTooltipSummary(month, todayDate) });
		});
		await page.setViewportSize({ width: 390, height: 844 });
		const todayDate = todayDateInSeoul();
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('tab', { name: '팀 현황' }).click();

		const statusTable = page.getByTestId('team-status-table');
		const todayCell = page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`);
		await expect(todayCell).not.toHaveAttribute('title');
		await todayCell.hover();
		await expect(page.locator('[data-slot="tooltip-content"]')).toHaveCount(0);

		await statusTable.evaluate((table) => {
			table.scrollLeft = 480;
			table.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, pointerType: 'touch' }));
		});
		await expect(page.locator('[data-slot="tooltip-content"]')).toHaveCount(0);
	});

	test('keeps English mobile controls and monthly status table inside the viewport', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDate.slice(0, 7);
			const summary = buildWideTooltipSummary(month, todayDate);
			await route.fulfill({ json: withLongMobileDisplayName(summary) });
		});
		await page.unroute('**/admin/api/locale');
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'en' } });
		});
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/attendance');

		await expect(page.getByRole('tab', { name: 'My records' })).toHaveAttribute('aria-selected', 'true');
		await page.getByRole('tab', { name: 'Team status' }).click();
		await expect(page.getByText('Monthly work status table')).toBeVisible();
		await expect(page.getByRole('combobox', { name: 'Select employee' })).toBeVisible();
		await expect(page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`)).toBeVisible();
		const monthPickerLayout = await mobileMonthPickerLayout(page, {
			employeeFilterLabel: 'Select employee',
			previousMonthLabel: 'Previous month',
			monthTriggerName: /\w+ \d{4}/,
			nextMonthLabel: 'Next month'
		});
		expect(monthPickerLayout.nextButtonRight).toBeLessThanOrEqual(monthPickerLayout.filterRight + 1);
		expect(monthPickerLayout.previousGap).toBeLessThanOrEqual(1);
		expect(monthPickerLayout.nextGap).toBeLessThanOrEqual(1);

		const layout = await measureMobileTeamStatusTable(page.getByTestId('team-status-table'), todayDate);

		expectReadableMobileTeamStatusTable(layout);
	});

	test('returns to the status table when resizing from mobile tools to desktop', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/attendance');
		await selectKorean(page);

		await page.getByRole('tab', { name: '팀 현황' }).click();
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await page.getByRole('tab', { name: '내 기록' }).click();
		await expect(page.getByTestId('mobile-attendance-tools-view')).toBeVisible();

		await page.setViewportSize({ width: 1280, height: 900 });

		await expect(page.getByRole('tab')).toHaveCount(0);
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await expect(page.getByText('내 근무 시간')).toBeVisible();
	});
});
