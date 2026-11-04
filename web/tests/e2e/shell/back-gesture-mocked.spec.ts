import { expect, test } from '@playwright/test';
import { mockBuzzDisabled } from '../messenger/buzz-test-routes';
import { insideTheIPhoneApp, lastBackGestureChoice } from '../support/iphone-app';

test.beforeEach(async ({ page }) => {
	await mockBuzzDisabled(page);
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'ko' } }));
	await page.setViewportSize({ width: 390, height: 844 });
	await insideTheIPhoneApp(page);
});

test('the iPhone app keeps the back swipe off on main pages and on below them', async ({ page }) => {
	await page.goto('/settings/');
	await expect.poll(() => lastBackGestureChoice(page)).toBe(false);

	await page.goto('/settings/setup');
	await expect.poll(() => lastBackGestureChoice(page)).toBe(true);

	await page.getByRole('link', { name: '업무', exact: true }).click();
	await expect(page).toHaveURL(/\/task\/?$/);
	await expect.poll(() => lastBackGestureChoice(page)).toBe(false);
});
