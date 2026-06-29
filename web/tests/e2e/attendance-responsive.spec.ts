import { expect, test } from './attendance-page-test-fixture';
import { selectKorean } from './attendance-test-helpers';

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
