import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import { expect, test } from './attendance-page-test-fixture';
import { buildWideTooltipSummary, selectKorean, todayDateInSeoul } from './attendance-test-helpers';

test.describe('attendance team status hover', () => {
	test('shows work segment tooltip in a team status day cell', async ({ page }) => {
		const todayDate = todayDateInSeoul();
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDate.slice(0, 7);
			await route.fulfill({ json: buildWideTooltipSummary(month, todayDate) });
		});

		await page.clock.setFixedTime(new Date(`${todayDate}T15:45:00+09:00`));
		await page.goto('/attendance');
		await selectKorean(page);

		const todayCell = page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`);
		await expect(todayCell).toBeVisible();
		await expect(todayCell.getByText('외부')).toHaveCount(0);
		await expect(todayCell.getByText('근무 중')).toHaveCount(0);
		await expect(todayCell.locator('span[style*="background-color"]')).toHaveCount(3);
		await todayCell.hover();
		const tooltip = page.locator('[data-slot="tooltip-content"]');
		await expect(tooltip.getByText('사무실본관회의실A')).toHaveCount(3);
		await expect(tooltip.getByLabel('09:46-10:46')).toBeVisible();

		await page.getByTestId(`team-status-cell-lee@example.com-${todayDate}`).hover();
		await expect(page.locator('[data-slot="tooltip-content"]')).toHaveCount(1);
	});


	test('shows an employee work time chart in the employee header hover card', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId('team-status-person-header-kim@example.com').hover();

		const hoverCard = page.locator('[data-slot="hover-card-content"]');
		await expect(hoverCard.getByText('근무 시간', { exact: true })).toBeVisible();
		await expect(hoverCard.getByText('내 근무 시간')).toHaveCount(0);
		await expect(hoverCard.getByText('일일 평균')).toBeVisible();
		await expect(hoverCard.getByText('일일 최고')).toBeVisible();
		await expect(hoverCard.getByText('일일 최저')).toBeVisible();

		await page.getByTestId('team-status-person-header-lee@example.com').hover();
		await expect(page.locator('[data-slot="hover-card-content"]')).toHaveCount(1);
	});

});
