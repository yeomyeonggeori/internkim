import { expect, test } from '@playwright/test';
import { buildAttendanceSummaryFixture } from '../fixtures/attendance-summary';

test.describe('attendance', () => {
	test.beforeEach(async ({ page }) => {
		await page.route('**/attendance/api/summary**', async (route) => {
			const requestURL = new URL(route.request().url());
			const month = requestURL.searchParams.get('month') ?? '2026-05';
			await route.fulfill({ json: buildAttendanceSummaryFixture(month) });
		});
	});

	test('renders team and personal tabs with fixture summary', async ({ page }) => {
		await page.goto('/attendance');
		await page.getByLabel('Language').getByRole('button', { name: 'KO', exact: true }).click();
		await expect(page.getByRole('tab', { name: '팀' })).toBeVisible();
		await expect(page.getByRole('tab', { name: '개인' })).toBeVisible();
		await page.getByRole('tab', { name: '개인' }).click();
		await expect(page.getByText('내 근무 시간')).toBeVisible();
		await page.getByRole('tab', { name: '팀' }).click();
		await expect(page.locator('text=출석률').first()).toBeVisible();
	});

	test('keeps the selected person tab after refreshing attendance', async ({ page }) => {
		await page.goto('/attendance');
		await page.getByLabel('Language').getByRole('button', { name: 'KO', exact: true }).click();
		await page.getByRole('button', { name: /김철수/ }).first().click();
		await expect(page.getByText('내 근무 시간')).toBeVisible();

		await page.getByRole('button', { name: '출결 새로고침' }).click();

		await expect(page.getByRole('tab', { name: '개인' })).toHaveAttribute('data-state', 'active');
		await expect(page.getByText('내 근무 시간')).toBeVisible();
	});
});
