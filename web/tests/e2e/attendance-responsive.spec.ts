import type { Locator } from '@playwright/test';
import { expect, test } from './attendance-page-test-fixture';
import { buildWideTooltipSummary, selectKorean, todayDateInSeoul } from './attendance-test-helpers';

type MobileTeamStatusTableLayout = {
	canScrollDates: boolean;
	initialEmployeeLeft: number;
	initialEmployeeRowLeft: number;
	initialTargetLeft: number;
	scrolledEmployeeLeft: number;
	scrolledEmployeeRowLeft: number;
	scrolledTargetLeft: number;
	tableLeft: number;
	overflowingLabels: string[];
	overflowingEmployeeDetails: string[];
	pageOverflows: boolean;
};

test.describe('attendance responsive view', () => {
	test('renders the monthly team status table with personal tools in the fixed sidebar', async ({ page }) => {
		await page.goto('/attendance');
		await selectKorean(page);

		await expect(page.getByRole('tab')).toHaveCount(0);
		await expect(page.getByTestId('team-month-calendar')).toHaveCount(0);
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await expect(page.getByTestId('team-status-table')).toBeVisible();
		await expect(page.getByText('월간 근무 현황표')).toBeVisible();
		await expect(page.getByText('내 근무 시간')).toBeVisible();
		await expect(page.getByTestId('personal-month-calendar-grid')).toBeVisible();
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

		const statusTab = page.getByRole('tab', { name: '현황' });
		const toolsTab = page.getByRole('tab', { name: '내 도구' });

		await expect(statusTab).toBeVisible();
		await expect(statusTab).toHaveAttribute('aria-selected', 'true');
		await expect(toolsTab).toBeVisible();
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await expect(page.getByTestId('mobile-attendance-tools-view')).toBeHidden();

		await toolsTab.click();

		await expect(toolsTab).toHaveAttribute('aria-selected', 'true');
		await expect(page.getByTestId('mobile-attendance-tools-view')).toBeVisible();
		await expect(page.getByTestId('team-status-grid')).toBeHidden();

		await statusTab.click();

		await expect(statusTab).toHaveAttribute('aria-selected', 'true');
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await expect(page.getByTestId('mobile-attendance-tools-view')).toBeHidden();
	});

	test('keeps the mobile monthly status table readable while date columns scroll', async ({ page }) => {
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

		await expect(page.getByTestId('team-status-table')).toBeVisible();
		await expect(page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`)).toBeVisible();
		await expect(page.getByPlaceholder('직원 검색')).toBeVisible();

		const layout = await measureMobileTeamStatusTable(page.getByTestId('team-status-table'), todayDate);

		expectReadableMobileTeamStatusTable(layout);
	});

	test('keeps English mobile controls and monthly status table inside the viewport', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDate.slice(0, 7);
			await route.fulfill({ json: buildWideTooltipSummary(month, todayDate) });
		});
		await page.unroute('**/admin/api/locale');
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'en' } });
		});
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/attendance');

		await expect(page.getByText('Monthly work status table')).toBeVisible();
		await expect(page.getByPlaceholder('Search employees')).toBeVisible();
		await expect(page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`)).toBeVisible();

		const layout = await measureMobileTeamStatusTable(page.getByTestId('team-status-table'), todayDate);

		expectReadableMobileTeamStatusTable(layout);
	});

	test('returns to the status table when resizing from mobile tools to desktop', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/attendance');
		await selectKorean(page);

		await page.getByRole('tab', { name: '내 도구' }).click();
		await expect(page.getByTestId('mobile-attendance-tools-view')).toBeVisible();

		await page.setViewportSize({ width: 1280, height: 900 });

		await expect(page.getByRole('tab')).toHaveCount(0);
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await expect(page.getByText('내 근무 시간')).toBeVisible();
	});
});

async function measureMobileTeamStatusTable(
	statusTable: Locator,
	targetDate: string
): Promise<MobileTeamStatusTableLayout> {
	return statusTable.evaluate((tableElement, date) => {
		const table = tableElement as HTMLElement;
		const employeeHeader = table.querySelector<HTMLElement>('[role="columnheader"]');
		const employeeRowHeader = table.querySelector<HTMLElement>('[role="rowheader"]');
		const targetCell = table.querySelector<HTMLElement>(`[data-testid="team-status-cell-kim@example.com-${date}"]`);
		if (!employeeHeader || !employeeRowHeader || !targetCell) {
			throw new Error('Missing monthly attendance table cells');
		}

		table.scrollLeft = 0;
		const initialEmployeeLeft = Math.round(employeeHeader.getBoundingClientRect().left);
		const initialEmployeeRowLeft = Math.round(employeeRowHeader.getBoundingClientRect().left);
		const initialTargetLeft = Math.round(targetCell.getBoundingClientRect().left);
		const tableLeft = Math.round(table.getBoundingClientRect().left);
		table.scrollLeft = 480;
		const scrolledEmployeeLeft = Math.round(employeeHeader.getBoundingClientRect().left);
		const scrolledEmployeeRowLeft = Math.round(employeeRowHeader.getBoundingClientRect().left);
		const scrolledTargetLeft = Math.round(targetCell.getBoundingClientRect().left);
		const overflowingLabels = Array.from(
			table.querySelectorAll<HTMLButtonElement>('button[data-testid^="team-status-cell-"]')
		)
			.map((button) => button.firstElementChild)
			.filter((label): label is HTMLElement => label instanceof HTMLElement)
			.filter((label) => label.scrollWidth > label.clientWidth + 1)
			.map((label) => label.textContent?.trim() ?? '');
		const overflowingEmployeeDetails = Array.from(table.querySelectorAll<HTMLElement>('[role="rowheader"] span'))
			.filter((label) => label.scrollWidth > label.clientWidth + 1)
			.map((label) => label.textContent?.trim() ?? '');

		return {
			canScrollDates: table.scrollWidth > table.clientWidth,
			initialEmployeeLeft,
			initialEmployeeRowLeft,
			initialTargetLeft,
			scrolledEmployeeLeft,
			scrolledEmployeeRowLeft,
			scrolledTargetLeft,
			tableLeft,
			overflowingLabels,
			overflowingEmployeeDetails,
			pageOverflows: document.documentElement.scrollWidth > window.innerWidth
		};
	}, targetDate);
}

function expectReadableMobileTeamStatusTable(layout: MobileTeamStatusTableLayout): void {
	expect(layout.canScrollDates).toBe(true);
	expect(Math.abs(layout.scrolledEmployeeLeft - layout.tableLeft)).toBeLessThanOrEqual(1);
	expect(Math.abs(layout.scrolledEmployeeRowLeft - layout.tableLeft)).toBeLessThanOrEqual(1);
	expect(Math.abs(layout.scrolledEmployeeRowLeft - layout.initialEmployeeRowLeft)).toBeLessThanOrEqual(1);
	expect(layout.scrolledTargetLeft).toBeLessThan(layout.initialTargetLeft);
	expect(layout.overflowingLabels).toEqual([]);
	expect(layout.overflowingEmployeeDetails).toEqual([]);
	expect(layout.pageOverflows).toBe(false);
}
