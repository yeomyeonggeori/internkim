import { expect, test } from '@playwright/test';

test.describe('calendar localization', () => {
	test('updates embedded calendar labels when language changes', async ({ page }) => {
		await page.goto('/calendar/');
		await page.getByLabel('Language').getByRole('button', { name: 'EN', exact: true }).click();

		const calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.getByRole('button', { name: 'Today' })).toBeVisible();
		await expect(calendarFrame.getByText('Sun').first()).toBeVisible();
	});
});
