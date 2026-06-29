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
		const segments = dialog.getByTestId('team-status-day-segment');
		await expect(dialog.getByText('김철수')).toBeVisible();
		await expect(dialog.getByText('상태')).toHaveCount(0);
		await expect(dialog.getByText('3시간 25분')).toBeVisible();
		await expect(segments.filter({ hasText: '재택' })).toBeVisible();
		await expect(segments.filter({ hasText: '사무실' })).toBeVisible();
		await expect(segments.filter({ hasText: '외부' })).toBeVisible();
		await expect(dialog.getByText('08:30-10:20')).toBeVisible();
		await expect(dialog.getByText('10:45-12:20')).toBeVisible();
		await expect(dialog.getByText('12:45~')).toBeVisible();
	});

	test('opens mobile status day details in a read-only bottom sheet', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.unroute('**/attendance/api/summary**');
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? todayDateInSeoul().slice(0, 7);
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});

		const todayDate = todayDateInSeoul();
		await page.goto('/attendance');
		await selectKorean(page);
		await page.getByRole('tab', { name: '팀 현황' }).click();
		await page.getByTestId(`team-status-cell-kim@example.com-${todayDate}`).click();

		const sheet = page.getByTestId('team-status-day-detail-sheet');
		const segments = sheet.getByTestId('team-status-day-segment');
		await expect(sheet).toBeVisible();
		await expect(sheet.getByText(`김철수 · ${todayDate}`)).toBeVisible();
		await expect(sheet.getByText('3시간 25분')).toBeVisible();
		await expect(segments.filter({ hasText: '재택' })).toBeVisible();
		await expect(segments.filter({ hasText: '사무실' })).toBeVisible();
		await expect(segments.filter({ hasText: '외부' })).toBeVisible();
		await expect(sheet.getByText('08:30-10:20')).toBeVisible();
		await expect(sheet.getByText('10:45-12:20')).toBeVisible();
		await expect(sheet.getByText('12:45~')).toBeVisible();
		await expect(sheet.getByRole('button', { name: '수정' })).toHaveCount(0);
		await expect(sheet.getByRole('button', { name: '취소' })).toHaveCount(0);
		await expect(page.getByTestId('team-status-day-detail-dialog')).toHaveCount(0);
	});

	test('opens mobile status absence details without actions', async ({ page }) => {
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
		await expect(sheet.getByText('휴가')).toBeVisible();
		await expect(sheet.getByText('sample overlap leave')).toBeVisible();
		await expect(sheet.getByRole('button', { name: '수정' })).toHaveCount(0);
		await expect(sheet.getByRole('button', { name: '취소' })).toHaveCount(0);
	});
});
