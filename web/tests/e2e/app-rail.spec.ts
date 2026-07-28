import { expect, type Locator, type Page, test } from '@playwright/test';
import { feedbackFormURL } from '../../src/lib/components/app-rail-config';
import { createTaskRunsFixture } from './tasks-test-fixtures';

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
	let responseLocale = 'ko';

	test.beforeEach(async ({ page }) => {
		responseLocale = 'ko';
		await page.route('**/admin/api/session', async (route) => {
			await route.fulfill({ json: { email: 'tester@example.com' } });
		});
		await page.route('**/admin/api/locale', async (route) => {
			await route.fulfill({ json: { locale: responseLocale } });
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

	test('marks only the current rail item as active', async ({ page }) => {
		await page.setViewportSize({ width: 930, height: 1904 });
		await page.goto('/memory/');

		const appRail = page.locator(appRailSelector);
		await expect(appRail.getByRole('link', { name: '기억' })).toHaveAttribute('data-active', 'true');
		await expect(appRail.getByRole('link', { name: '업무' })).not.toHaveAttribute('data-active');
		await expect(appRail.getByRole('link', { name: '문의하기' })).not.toHaveAttribute('data-active');
		await expect.poll(async () => railIconSize(appRail.getByRole('link', { name: '기억' }))).toBe(20);
	});

	test('uses bottom navigation instead of the rail on mobile', async ({ page }) => {
		await page.setViewportSize({ width: 444, height: 866 });
		await page.goto('/memory/');

		await expect(page.locator(appRailSelector)).toBeHidden();
		const bottomNavigation = mobileBottomNavigation(page);
		await expect(bottomNavigation).toBeVisible();
		await expect(bottomNavigation.getByRole('button', { name: '더보기', exact: true })).toBeVisible();
	});

	test('treats mobile bottom navigation as a subtle overlay instead of a reserved well', async ({ page }) => {
		await page.setViewportSize({ width: 444, height: 866 });
		await page.goto('/memory/');

		const bottomNavigation = mobileBottomNavigation(page);
		await expect(bottomNavigation).toBeVisible();

		const metrics = await mobileBottomNavigationOverlayMetrics(page);
		expect(metrics.backgroundAlpha).toBeGreaterThanOrEqual(0.72);
		expect(metrics.backgroundAlpha).toBeLessThanOrEqual(0.86);
		expect(metrics.backdropFilter).toContain('blur');
		expect(metrics.scrollPaddingBottom).toBeLessThan(metrics.navigationHeight);
	});

	test('keeps task pagination controls reachable above the mobile overlay', async ({ page }) => {
		await page.setViewportSize({ width: 444, height: 866 });
		await page.route('**/tasks/api/runs**', async (route) => {
			await route.fulfill({ json: createTaskRunsFixture() });
		});
		await page.goto('/tasks/');

		const bottomNavigation = mobileBottomNavigation(page);
		await expect(bottomNavigation).toBeVisible();
		const nextPageButton = page.getByRole('button', { name: '다음' });
		await expect(nextPageButton).toBeVisible();

		await page.locator('[data-app-shell-scroll]').evaluate((element) => {
			if (element instanceof HTMLElement) element.scrollTop = element.scrollHeight;
		});

		await expect.poll(async () => elementOverlap(nextPageButton, bottomNavigation)).toBe(0);
	});

	test('opens the contact form from the rail footer', async ({ page, context }) => {
		await context.route('https://forms.gle/**', async (route) => {
			await route.fulfill({ contentType: 'text/html', body: '<title>InternKim feedback</title>' });
		});
		await page.setViewportSize({ width: 930, height: 1904 });
		await page.goto('/memory/');

		const contactLink = page.getByRole('link', { name: '문의하기' });
		const profileButton = page.getByRole('button', { name: 'tester' });
		await expect(contactLink).toHaveAttribute('href', feedbackFormURL);
		await expect(contactLink).toHaveAttribute('target', '_blank');
		await expect(contactLink).toHaveAttribute('rel', 'noopener noreferrer');
		await expect.poll(async () => verticalGap(contactLink, profileButton)).toBeGreaterThanOrEqual(0);

		const popupPromise = page.waitForEvent('popup');
		await contactLink.click();
		const popup = await popupPromise;

		await expect(popup).toHaveURL(feedbackFormURL);
	});

	test('renders the contact form link in English locale', async ({ page }) => {
		responseLocale = 'en';
		await page.setViewportSize({ width: 930, height: 1904 });
		await page.goto('/memory/');

		const contactLink = page.getByRole('link', { name: 'Contact us' });
		await expect(contactLink).toHaveAttribute('href', feedbackFormURL);
		await expect(contactLink).toHaveAttribute('target', '_blank');
		await expect(contactLink).toHaveAttribute('rel', 'noopener noreferrer');
	});

	test('ignores the sidebar keyboard shortcut in the rail', async ({ page }) => {
		await page.goto('/memory/');
		await page.evaluate(() => {
			document.cookie = 'sidebar:state=; path=/; max-age=0';
		});

		await page.keyboard.press('Control+B');

		await expect.poll(async () => page.evaluate(() => document.cookie.includes('sidebar:state='))).toBe(false);
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

async function verticalGap(upperElement: ReturnType<Page['getByRole']>, lowerElement: ReturnType<Page['getByRole']>): Promise<number> {
	const upperBox = await upperElement.boundingBox();
	const lowerBox = await lowerElement.boundingBox();
	if (!upperBox || !lowerBox) return Number.NEGATIVE_INFINITY;

	return lowerBox.y - (upperBox.y + upperBox.height);
}

async function railIconSize(linkElement: ReturnType<Page['getByRole']>): Promise<number> {
	const iconBox = await linkElement.locator('svg').first().boundingBox();
	if (!iconBox) return 0;

	return Math.round(Math.max(iconBox.width, iconBox.height));
}


async function elementOverlap(firstElement: Locator, secondElement: Locator): Promise<number> {
	const firstBox = await firstElement.boundingBox();
	const secondBox = await secondElement.boundingBox();
	if (!firstBox || !secondBox) return Number.POSITIVE_INFINITY;
	const horizontalOverlap = Math.max(0, Math.min(firstBox.x + firstBox.width, secondBox.x + secondBox.width) - Math.max(firstBox.x, secondBox.x));
	const verticalOverlap = Math.max(0, Math.min(firstBox.y + firstBox.height, secondBox.y + secondBox.height) - Math.max(firstBox.y, secondBox.y));
	return Math.round(horizontalOverlap * verticalOverlap);
}

function mobileBottomNavigation(page: Page): ReturnType<Page['locator']> {
	return page.locator('nav[aria-label]').filter({ has: page.getByRole('button', { name: '더보기', exact: true }) });
}

async function mobileBottomNavigationOverlayMetrics(page: Page): Promise<{
	backgroundAlpha: number;
	backdropFilter: string;
	navigationHeight: number;
	scrollPaddingBottom: number;
}> {
	return page.evaluate(() => {
		const navigation = [...document.querySelectorAll('nav[aria-label]')].find((element) => element.textContent?.includes('더보기'));
		const scrollContainer = document.querySelector('[data-app-shell-scroll]');
		if (!(navigation instanceof HTMLElement)) throw new Error('mobile bottom navigation not found');
		if (!(scrollContainer instanceof HTMLElement)) throw new Error('app shell scroll container not found');

		const navigationStyle = window.getComputedStyle(navigation);
		const scrollContainerStyle = window.getComputedStyle(scrollContainer);
		const navigationRectangle = navigation.getBoundingClientRect();

		return {
			backgroundAlpha: alphaFromCSSColor(navigationStyle.backgroundColor),
			backdropFilter: navigationStyle.backdropFilter,
			navigationHeight: navigationRectangle.height,
			scrollPaddingBottom: Number.parseFloat(scrollContainerStyle.paddingBottom)
		};

		function alphaFromCSSColor(color: string): number {
			const slashAlphaMatch = color.match(/\/\s*([0-9.]+)\s*\)$/);
			if (slashAlphaMatch) return Number.parseFloat(slashAlphaMatch[1] ?? '1');
			const rgbaMatch = color.match(/^rgba?\((.+)\)$/);
			if (!rgbaMatch) return 1;
			const values = (rgbaMatch[1] ?? '').split(',').map((value) => value.trim());
			if (values.length < 4) return 1;
			return Number.parseFloat(values[3] ?? '1');
		}
	});
}
