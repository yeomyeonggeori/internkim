import { expect, type Page, test } from '@playwright/test';

const expandedRailWidth = 224;
const appRailSelector = '[data-app-rail]';
const profileMenuSelector = '[data-app-rail-profile-menu]';
const maximumProfileMenuRailGap = 8;

const memoryGraphFixture = {
	health: { configured: true, reachable: true },
	namespaces: [],
	episodes: [],
	facts: [],
	nodes: [],
	edges: []
};

type RailMetrics = {
	menuLeft: number | null;
	railRight: number | null;
	railWidth: number | null;
};

test.describe('app rail', () => {
	test.beforeEach(async ({ page }) => {
		await page.route('**/admin/api/session', async (route) => {
			await route.fulfill({ json: { email: 'tester@example.com' } });
		});
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: 'ko' } });
		});
		await page.route('**/auth/session**', async (route) => {
			await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
		});
		await page.route('**/memory/api/graph**', async (route) => {
			await route.fulfill({ json: memoryGraphFixture });
		});
	});

	test('keeps the rail expanded while the profile menu is open', async ({ page }) => {
		await page.setViewportSize({ width: 930, height: 1904 });
		await page.goto('/memory/');

		await page.getByRole('button', { name: 'tester' }).click();
		await page.mouse.move(500, 500);

		await expect(page.getByRole('menuitem', { name: '계정' })).toBeVisible();
		await expect.poll(async () => railMetrics(page)).toMatchObject({
			menuLeft: expect.any(Number),
			railRight: expect.any(Number),
			railWidth: expandedRailWidth
		});
		await expect.poll(async () => profileMenuRailGap(page)).toBeLessThanOrEqual(maximumProfileMenuRailGap);
	});
});

async function railMetrics(page: Page): Promise<RailMetrics> {
	return page.evaluate((selectors) => {
		const rail = document.querySelector(selectors.appRailSelector);
		const menu = document.querySelector(selectors.profileMenuSelector);
		const railRectangle = rail?.getBoundingClientRect() ?? null;
		const menuRectangle = menu?.getBoundingClientRect() ?? null;

		return {
			menuLeft: menuRectangle?.left ?? null,
			railRight: railRectangle?.right ?? null,
			railWidth: railRectangle?.width ?? null
		};
	}, { appRailSelector, profileMenuSelector });
}

async function profileMenuRailGap(page: Page): Promise<number> {
	const metrics = await railMetrics(page);
	if (metrics.menuLeft === null || metrics.railRight === null) return Number.MAX_SAFE_INTEGER;

	return Math.abs(metrics.menuLeft - metrics.railRight);
}
