import { expect, test } from '@playwright/test';

test('reuses a visited week until the page reloads', async ({ page }) => {
	const summaryRequests = new Map<string, number>();
	await page.route('**/flow/api/summary**', async (route) => {
		const week = new URL(route.request().url()).searchParams.get('week') ?? '';
		summaryRequests.set(week, (summaryRequests.get(week) ?? 0) + 1);
		await route.continue();
	});
	await page.goto('/flow?week=26W23');
	await expect(page.getByRole('button', { name: '날짜로 주차 이동' })).toContainText('6/1 - 6/7');
	await page.getByRole('button', { name: '다음 주', exact: true }).click();
	await page.getByRole('button', { name: '이전 주', exact: true }).click();
	expect(summaryRequests.get('26W23')).toBe(1);
	await page.reload();
	await expect(page.getByRole('button', { name: '날짜로 주차 이동' })).toContainText('6/1 - 6/7');
	expect(summaryRequests.get('26W23')).toBe(2);
});
