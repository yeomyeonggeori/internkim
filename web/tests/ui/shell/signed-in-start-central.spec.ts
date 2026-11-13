import { expect, test, type Page } from '@playwright/test';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';
import { exampleCompanySlug } from '../support/central-test-utils';
import { insideTheIPhoneApp } from '../support/iphone-app';

test.use({ locale: 'ko-KR' });

function documentsLoadedBy(page: Page): string[] {
	const documents: string[] = [];
	page.on('request', (request) => {
		if (request.isNavigationRequest() && request.frame() === page.mainFrame()) documents.push(new URL(request.url()).pathname);
	});
	return documents;
}

test('a signed-in member who opens the zone root reaches attendance on the document that was first loaded', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await signInToTheCentralPlane(page, `/${exampleCompanySlug}/task`);
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();

	const documents = documentsLoadedBy(page);
	await page.goto('/');
	await expect(page).toHaveURL(new RegExp(`/${exampleCompanySlug}/attendance/?$`));
	expect(documents).toEqual(['/']);
});

test('the app plays the logo video once, to its end, while a signed-in member is moved to attendance', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await signInToTheCentralPlane(page, `/${exampleCompanySlug}/task`);
	await expect(page.locator('[data-task-ready="true"]')).toBeVisible();

	await insideTheIPhoneApp(page);
	await page.evaluate(() => sessionStorage.clear());
	const documents = documentsLoadedBy(page);
	await page.goto('/');
	const video = page.locator('#launch-video video');
	await expect.poll(() => video.evaluate((playing: HTMLVideoElement) => playing.currentTime), { timeout: 15_000 }).toBeGreaterThan(3);
	await expect(page).toHaveURL(new RegExp(`/${exampleCompanySlug}/attendance/?$`));
	expect(documents).toEqual(['/']);
});
