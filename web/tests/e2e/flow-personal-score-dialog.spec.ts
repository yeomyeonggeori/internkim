import { expect, test } from '@playwright/test';

test.describe('flow personal score dialog', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/flow/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('opens only the current user score detail from the member score card header', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto('/flow/');

		await page.getByRole('button', { name: '보고', exact: true }).click();
		await expect(page.getByRole('dialog')).toHaveCount(0);
		await expect(page.getByRole('columnheader', { name: '가중 점수', exact: true })).toHaveCount(0);

		const memberScoreCard = page.locator('[data-slot="card"]').filter({
			has: page.getByText('구성원 점수', { exact: true })
		});
		await expect(memberScoreCard.getByRole('button', { name: '내 점수 상세', exact: true })).toBeVisible();
		await expect(memberScoreCard.getByRole('button')).toHaveCount(1);

		await memberScoreCard.getByRole('button', { name: '내 점수 상세', exact: true }).click();
		const dialog = page.getByRole('dialog');
		await expect(dialog).toBeVisible();
		await expect(dialog.getByRole('heading', { name: /점수 상세$/ })).toBeVisible();
		await expect(dialog.getByText('주간 점수', { exact: true })).toBeVisible();
		await expect(dialog.getByText('월간 점수', { exact: true })).toBeVisible();
		await expect(dialog.getByRole('columnheader', { name: '가중 점수', exact: true })).toHaveCount(2);

		await page.keyboard.press('Escape');
		await expect(dialog).toBeHidden();
	});
});
