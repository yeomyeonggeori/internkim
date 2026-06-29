import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import { expect, test } from './attendance-page-test-fixture';
import { buildWideTooltipSummary, selectKorean, todayDateInSeoul } from './attendance-test-helpers';

test.describe('attendance team status', () => {
	test('shows location bars and segment tooltip in a team status cell', async ({ page }) => {
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
		await expect(todayCell.getByText('외부')).toBeVisible();
		await expect(todayCell.getByText('근무 중')).toHaveCount(0);
		await todayCell.hover();
		await expect(page.getByText('08:30-10:20')).toBeVisible();
		await expect(page.getByText('1시간 50분')).toBeVisible();
		await expect(page.getByText('10:45-12:20')).toBeVisible();
		await expect(page.getByText('1시간 35분')).toBeVisible();
		await expect(page.getByText('12:45~')).toBeVisible();
		await expect(page.getByText('진행 중')).toBeVisible();
	});

	test('sizes team status tooltip columns to fit long segment labels', async ({ page }) => {
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildWideTooltipSummary(month, todayDateInSeoul()) });
		});

		const todayDate = todayDateInSeoul();
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).hover();

		const tooltip = page.locator('[data-slot="tooltip-content"]').filter({ hasText: '사무실본관회의실A' });
		await expect(tooltip).toBeVisible();

		const layout = await tooltip.evaluate((content) => {
			const rows = Array.from(content.children).filter((child): child is HTMLElement => {
				return child instanceof HTMLElement && child.querySelectorAll('span').length >= 4;
			});
			const overflowingCells = rows.flatMap((row) => {
				return Array.from(row.querySelectorAll('span'))
					.filter((span) => (span.textContent ?? '').trim().length > 0)
					.filter((span) => span.scrollWidth > span.clientWidth + 1)
					.map((span) => (span.textContent ?? '').trim());
			});
			const columnLefts = rows.map((row) => {
				const spans = Array.from(row.querySelectorAll('span'));
				return spans.slice(1, 4).map((span) => Math.round(span.getBoundingClientRect().left));
			});
			const maximumColumnDrift = columnLefts[0]
				? Math.max(
						...columnLefts.flatMap((lefts) =>
							lefts.map((left, index) => Math.abs(left - (columnLefts[0]?.[index] ?? left)))
						)
					)
				: 0;
			return { overflowingCells, maximumColumnDrift };
		});
		expect(layout.overflowingCells).toEqual([]);
		expect(layout.maximumColumnDrift).toBeLessThanOrEqual(1);
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
		await expect(dialog.getByText('김철수')).toBeVisible();
		await expect(dialog.getByText('상태')).toHaveCount(0);
		await expect(dialog.getByText('3시간 25분')).toBeVisible();
		await expect(dialog.getByText('재택')).toBeVisible();
		await expect(dialog.getByText('사무실')).toBeVisible();
		await expect(dialog.getByText('외부')).toBeVisible();
		await expect(dialog.getByText('08:30-10:20')).toBeVisible();
		await expect(dialog.getByText('10:45-12:20')).toBeVisible();
		await expect(dialog.getByText('12:45~')).toBeVisible();
	});
});
