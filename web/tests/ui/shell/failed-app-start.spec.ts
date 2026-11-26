import { expect, test, type Page } from '@playwright/test';

const partOfTheApp = '**/_app/immutable/chunks/*.js';
const screen = 'body > div';
const notice = '#start-failure';

test.use({ serviceWorkers: 'block' });

async function loseOnePartOfTheApp(page: Page, times: number): Promise<void> {
	let lostPath = '';
	let lost = 0;
	await page.route(partOfTheApp, (route) => {
		const path = new URL(route.request().url()).pathname;
		lostPath ||= path;
		if (path !== lostPath || lost >= times) return route.continue();
		lost += 1;
		return route.abort('connectionreset');
	});
}

test('an app that lost one part on the way loads again by itself and opens', async ({ page }) => {
	await loseOnePartOfTheApp(page, 1);

	await page.goto('/attendance/', { waitUntil: 'domcontentloaded' });

	await expect(page.locator(screen)).toContainText(/\S/);
	await expect(page.locator(notice)).toHaveCount(0);
});

test('an app that loses a part twice says so and opens on another try', async ({ page }) => {
	await loseOnePartOfTheApp(page, 2);

	await page.goto('/attendance/', { waitUntil: 'domcontentloaded' });

	await expect(page.locator(notice)).toBeVisible();
	await page.locator(notice).getByRole('button').click();
	await expect(page.locator(screen)).toContainText(/\S/);
	await expect(page.locator(notice)).toHaveCount(0);
});
