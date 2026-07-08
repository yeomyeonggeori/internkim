import { expect, test } from './attendance-page-test-fixture';
import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
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
		await expect(page.getByRole('button', { name: '부재 등록' })).toBeVisible();

		const statusTable = page.getByTestId('team-status-table');
		await page.getByPlaceholder('직원 검색').fill('김철수');
		await expect(statusTable.getByText('김철수')).toBeVisible();
		await expect(statusTable.getByText('강민호')).toHaveCount(0);
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
		expect(tabListLayout.width).toBeLessThanOrEqual(220);
		expect(tabListLayout.left).toBeLessThanOrEqual(24);
		await expect(page.getByTestId('mobile-attendance-tools-view')).toBeVisible();
		await expect(page.getByTestId('team-status-grid')).toBeHidden();

		await statusTab.click();

		await expect(statusTab).toHaveAttribute('aria-selected', 'true');
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await expect(page.getByTestId('mobile-attendance-tools-view')).toBeHidden();
		const monthPickerLayout = await mobileMonthPickerLayout(page);
		expect(Math.abs(monthPickerLayout.searchRight - monthPickerLayout.nextButtonRight)).toBeLessThanOrEqual(1);
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
		await expect(toolsView.getByRole('button', { name: '부재 등록' })).toBeVisible();

		await page.getByRole('tab', { name: '팀 현황' }).click();
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();

		const detailSheet = page.getByTestId('team-status-day-detail-sheet');
		const detailPanel = detailSheet.getByTestId('personal-day-detail-panel');
		await expect(detailPanel).toBeVisible();
		await expect(detailPanel.getByTestId('personal-day-event-label')).toHaveCount(5);
		await expect(detailPanel.getByRole('button', { name: '수정' })).toHaveCount(0);
		await detailPanel.getByTestId('personal-day-event-label').first().click();
		await expect(detailPanel.getByRole('button', { name: '수정' })).toBeVisible();
	});

	test('keeps the mobile monthly status table readable while date columns scroll', async ({ page }) => {
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
		await expect(page.getByPlaceholder('직원 검색')).toBeVisible();

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
		await expect(page.getByPlaceholder('Search employees')).toBeVisible();
		await expect(page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`)).toBeVisible();
		const monthPickerLayout = await mobileMonthPickerLayout(page, {
			searchPlaceholder: 'Search employees',
			previousMonthLabel: 'Previous month',
			monthTriggerName: /\w+ \d{4}/,
			nextMonthLabel: 'Next month'
		});
		expect(monthPickerLayout.nextButtonRight).toBeLessThanOrEqual(monthPickerLayout.searchRight + 1);
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
