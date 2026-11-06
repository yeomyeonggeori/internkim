import { expect, test, type Page } from '@playwright/test';
import { mockBuzzDisabled } from '../messenger/buzz-test-routes';
import { insideTheIPhoneApp } from '../support/iphone-app';

const launchVideo = '#launch-video';

async function holdTheLaunchVideo(page: Page): Promise<() => void> {
	let release = () => {};
	const released = new Promise<void>((resolve) => {
		release = resolve;
	});
	await page.route('**/splash-logo*.mp4', async (route) => {
		await released;
		await route.continue();
	});
	return release;
}

test.beforeEach(async ({ page }) => {
	await mockBuzzDisabled(page);
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'ko' } }));
	await page.setViewportSize({ width: 390, height: 844 });
});

test('the app covers its first screen with the logo video once, then not again on reload', async ({ page }) => {
	await insideTheIPhoneApp(page);
	const releaseTheVideo = await holdTheLaunchVideo(page);

	await page.goto('/settings/', { waitUntil: 'domcontentloaded' });
	await expect(page.locator(launchVideo)).toBeVisible();
	await expect(page.locator(`${launchVideo} video`)).toHaveAttribute('src', '/splash-logo.mp4');

	releaseTheVideo();
	await expect(page.locator(launchVideo)).toHaveCount(0, { timeout: 7000 });

	await page.reload({ waitUntil: 'domcontentloaded' });
	await expect(page.locator(launchVideo)).toHaveCount(0);
});

test('the app in dark mode plays the logo video on black', async ({ page }) => {
	await page.emulateMedia({ colorScheme: 'dark' });
	await insideTheIPhoneApp(page);
	const releaseTheVideo = await holdTheLaunchVideo(page);

	await page.goto('/settings/', { waitUntil: 'domcontentloaded' });
	await expect(page.locator(`${launchVideo} video`)).toHaveAttribute('src', '/splash-logo-dark.mp4');
	await expect(page.locator(launchVideo)).toHaveCSS('background-color', 'rgb(0, 0, 0)');

	releaseTheVideo();
	await expect(page.locator(launchVideo)).toHaveCount(0, { timeout: 7000 });
});

test('a browser opens straight to the page without the logo video', async ({ page }) => {
	await page.goto('/settings/', { waitUntil: 'domcontentloaded' });
	await expect(page.locator(launchVideo)).toHaveCount(0);
});
