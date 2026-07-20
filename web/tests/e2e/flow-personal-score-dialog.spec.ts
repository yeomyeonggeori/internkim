import { expect, test } from '@playwright/test';

test.describe('flow personal score dialog', () => {
	test.beforeEach(async ({ request }) => {
		const response = await request.post('/flow/api/test/reset');
		expect(response.ok()).toBe(true);
	});

	test('serves participant image fallback through the Flow mock server', async ({ request }) => {
		const response = await request.get('/calendar/api/participants/test-person/image');

		expect(response.status()).toBe(404);
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
		await expect(memberScoreCard.getByRole('link')).toHaveCount(0);

		await memberScoreCard.getByRole('button', { name: '내 점수 상세', exact: true }).click();
		const dialog = page.getByRole('dialog');
		await expect(dialog).toBeVisible();
		await expect(dialog.getByRole('heading', { name: '김철수 점수 상세', exact: true })).toBeVisible();
		await expect(dialog.getByText('주간 점수', { exact: true })).toBeVisible();
		await expect(dialog.getByText('월간 점수', { exact: true })).toBeVisible();
		await expect(dialog.getByRole('columnheader', { name: '가중 점수', exact: true })).toHaveCount(2);

		const closeButton = dialog.getByRole('button', { name: '점수 상세 닫기', exact: true });
		await expect(closeButton).toBeVisible();
		await closeButton.click();
		await expect(dialog).toBeHidden();
	});

	test('uses singular English labels for the first previous score periods', async ({ page }) => {
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'en' } });
		});
		await page.addInitScript(() => {
			localStorage.setItem('internkim.locale', 'en');
		});
		await page.goto('/flow/');

		await page.getByRole('button', { name: 'Report', exact: true }).click();
		await page.getByRole('button', { name: 'My score details', exact: true }).click();

		const dialog = page.getByRole('dialog');
		await expect(dialog.getByText('1 week ago', { exact: true })).toBeVisible();
		await expect(dialog.getByText('1 month ago', { exact: true })).toBeVisible();
	});
});
