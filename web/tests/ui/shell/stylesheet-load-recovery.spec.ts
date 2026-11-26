import { expect, test, type Page } from '@playwright/test';

const shippedStylesheet = '**/_app/immutable/assets/*.css';

test.use({ serviceWorkers: 'block' });

async function failStylesheets(page: Page, failuresAllowed: number): Promise<() => number> {
	let documentsThatLostTheirStyles = 0;
	let documentsLoaded = 0;
	page.on('response', (response) => {
		const request = response.request();
		if (request.frame() === page.mainFrame() && request.resourceType() === 'document' && response.ok()) documentsLoaded += 1;
	});
	await page.route(shippedStylesheet, async (route) => {
		if (documentsLoaded > failuresAllowed) return route.continue();
		documentsThatLostTheirStyles = documentsLoaded;
		await route.abort('connectionfailed');
	});
	return () => documentsThatLostTheirStyles;
}

async function stylesheetsThatApplied(page: Page): Promise<{ linked: number; applied: number }> {
	return page.evaluate(() => {
		const links = [...document.querySelectorAll<HTMLLinkElement>('link[rel="stylesheet"][href*="/_app/immutable/"]')];
		return { linked: links.length, applied: links.filter((link) => link.sheet !== null).length };
	});
}

test('a page whose stylesheets failed to arrive loads again and is styled', async ({ page }) => {
	const documentsThatLostTheirStyles = await failStylesheets(page, 1);

	await page.goto('/settings/', { waitUntil: 'domcontentloaded' });

	await expect.poll(async () => (await stylesheetsThatApplied(page)).applied).toBeGreaterThan(0);
	const styles = await stylesheetsThatApplied(page);
	expect(styles.applied).toBe(styles.linked);
	expect(documentsThatLostTheirStyles()).toBe(1);
});

test('a page whose stylesheets never arrive loads again once and then stays', async ({ page }) => {
	const documentsThatLostTheirStyles = await failStylesheets(page, Number.POSITIVE_INFINITY);

	await page.goto('/settings/', { waitUntil: 'domcontentloaded' });

	await expect.poll(documentsThatLostTheirStyles).toBe(2);
	await page.waitForLoadState('load');
	await page.waitForTimeout(1500);
	expect(documentsThatLostTheirStyles()).toBe(2);
});
