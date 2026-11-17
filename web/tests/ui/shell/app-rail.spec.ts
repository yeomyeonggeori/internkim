import { expect, type Locator, type Page, test } from '@playwright/test';
import { feedbackFormURL } from '../../../src/lib/components/app-rail-config';
import { createTaskRunsFixture } from '../task/tasks-test-fixtures';

const appRailSelector = '[data-app-rail]';
const profileMenuSelector = '[data-app-rail-profile-menu]';
const maximumProfileMenuRailGap = 8;

const memoryFactsFixture = { personID: 'person-1', layers: [{ scopeType: 'person', scopeID: 'person-1' }, { scopeType: 'workspace' }], index: { embeddingModel: 'google/embeddinggemma-2', current: 0, stale: 0 }, facts: [] };

type RailConversation = {
	id: string;
	name: string;
	kind: 'dm' | 'group';
	unread: { status: 'exact'; value: number };
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
		await page.route('**/memory/api/facts**', async (route) => {
			await route.fulfill({ json: memoryFactsFixture });
		});
	});

	test('keeps the rail width while the profile menu is open', async ({ page }) => {
		await page.setViewportSize({ width: 930, height: 1904 });
		await page.goto('/memory/');

		const railWidthBeforeOpening = (await railMetrics(page)).railWidth;
		await page.getByRole('button', { name: 'tester' }).click();
		await page.mouse.move(500, 500);

		await expect(page.locator(profileMenuSelector)).toBeVisible();
		await expect.poll(async () => railMetrics(page)).toMatchObject({
			menuLeft: expect.any(Number),
			railRight: expect.any(Number),
			railWidth: railWidthBeforeOpening
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
		await expect.poll(async () => railIconSize(appRail.getByRole('link', { name: '기억' }))).toBe(16);
	});

	test('names a collapsed rail icon in a tooltip on hover', async ({ page }) => {
		await page.setViewportSize({ width: 930, height: 1904 });
		await page.goto('/memory/');

		await hoverUntilVisible(page.locator(appRailSelector).getByRole('link', { name: '업무' }), railTooltip(page, '업무'));
	});

	test('shows no tooltip when hovering expands the rail', async ({ page }) => {
		await page.addInitScript(() => localStorage.setItem('internkim.app-rail-mode', 'hover'));
		await page.setViewportSize({ width: 930, height: 1904 });
		await page.goto('/memory/');

		const taskLink = page.locator(appRailSelector).getByRole('link', { name: '업무' });
		await expect(async () => {
			await page.mouse.move(600, 600);
			await taskLink.hover();
			await expect(page.locator(appRailSelector).getByText('업무', { exact: true })).toBeVisible({ timeout: 1_000 });
		}).toPass({ timeout: 15_000 });

		await expect(railTooltip(page, '업무')).toBeHidden();
	});

	test('shows no tooltip on a folded rail icon when the rail expands on hover', async ({ page }) => {
		await page.addInitScript(() => localStorage.setItem('internkim.app-rail-mode', 'hover'));
		await page.setViewportSize({ width: 930, height: 1904 });
		await page.goto('/memory/');

		const taskLink = page.locator(appRailSelector).getByRole('link', { name: '업무' });
		await taskLink.focus();

		await expect(taskLink).toBeFocused();
		await expect(page.locator(appRailSelector).getByText('업무', { exact: true })).toBeHidden();
		await expect(railTooltip(page, '업무')).toBeHidden();
	});

	test('counts unread direct messages on the messenger rail item while another page is open', async ({ page }) => {
		await answerConversations(page, [
			{ id: 'dm-1', name: '박예시', kind: 'dm', unread: { status: 'exact', value: 2 } },
			{ id: 'dm-2', name: '최견본', kind: 'dm', unread: { status: 'exact', value: 1 } },
			{ id: 'channel-1', name: '공지', kind: 'group', unread: { status: 'exact', value: 7 } }
		]);
		await page.setViewportSize({ width: 930, height: 1904 });
		await page.goto('/memory/');

		await expect(railItem(page, '메신저').getByTestId('app-rail-nav-badge')).toHaveText(/^읽지 않은 메시지\s*3$/);
		await expect(railItem(page, '메신저').getByTestId('app-rail-nav-dot')).toHaveCount(0);
		await expect(railItem(page, '업무').getByTestId('app-rail-nav-badge')).toHaveCount(0);
	});

	test('keeps the unread count beside the messenger label on an expanded rail', async ({ page }) => {
		await page.addInitScript(() => localStorage.setItem('internkim.app-rail-mode', 'expanded'));
		await answerConversations(page, [{ id: 'dm-1', name: '박예시', kind: 'dm', unread: { status: 'exact', value: 150 } }]);
		await page.setViewportSize({ width: 1280, height: 900 });
		await page.goto('/memory/');

		const badge = railItem(page, '메신저').getByTestId('app-rail-nav-badge');
		await expect(badge).toHaveText(/^읽지 않은 메시지\s*99\+$/);
		const label = page.locator(appRailSelector).getByText('메신저', { exact: true });
		await expect(label).toBeVisible();
		const labelBox = (await label.boundingBox())!;
		const badgeBox = (await badge.boundingBox())!;
		expect(badgeBox.x).toBeGreaterThan(labelBox.x + labelBox.width);
	});

	test('marks the messenger rail item with a dot when only channels have new posts', async ({ page }) => {
		await answerConversations(page, [
			{ id: 'dm-1', name: '박예시', kind: 'dm', unread: { status: 'exact', value: 0 } },
			{ id: 'channel-1', name: '공지', kind: 'group', unread: { status: 'exact', value: 7 } }
		]);
		await page.setViewportSize({ width: 930, height: 1904 });
		await page.goto('/memory/');

		await expect(railItem(page, '메신저').getByTestId('app-rail-nav-dot')).toHaveText('채널에 새 글 있음');
		await expect(railItem(page, '메신저').getByTestId('app-rail-nav-badge')).toHaveCount(0);
	});

	test('leaves the messenger rail item unmarked when everything is read', async ({ page }) => {
		await answerConversations(page, [
			{ id: 'dm-1', name: '박예시', kind: 'dm', unread: { status: 'exact', value: 0 } },
			{ id: 'channel-1', name: '공지', kind: 'group', unread: { status: 'exact', value: 0 } }
		]);
		await page.setViewportSize({ width: 930, height: 1904 });
		const conversationsRead = page.waitForResponse('**/agent/api/channels');
		await page.goto('/memory/');
		await conversationsRead;

		await expect(railItem(page, '메신저')).toBeVisible();
		await expect(railItem(page, '메신저').getByTestId('app-rail-nav-badge')).toHaveCount(0);
		await expect(railItem(page, '메신저').getByTestId('app-rail-nav-dot')).toHaveCount(0);
	});

	test('uses bottom navigation instead of the rail on mobile', async ({ page }) => {
		await page.setViewportSize({ width: 444, height: 866 });
		await page.goto('/memory/');

		await expect(page.locator(appRailSelector)).toBeHidden();
		const bottomNavigation = mobileBottomNavigation(page);
		await expect(bottomNavigation).toBeVisible();
		await expect(bottomNavigation.getByRole('button', { name: '더보기', exact: true })).toBeVisible();
	});

	test('reaches the messenger from the first tab of the bottom navigation', async ({ page }) => {
		await page.setViewportSize({ width: 444, height: 866 });
		await page.goto('/memory/');

		const firstTab = mobileBottomNavigation(page).getByRole('link').first();
		await expect(firstTab).toHaveAttribute('aria-label', '메신저');
		await expect(firstTab).toHaveAttribute('href', '/messenger');
	});

	test('keeps mail in the more sheet instead of the bottom navigation', async ({ page }) => {
		await page.setViewportSize({ width: 444, height: 866 });
		await page.goto('/memory/');

		const bottomNavigation = mobileBottomNavigation(page);
		await expect(bottomNavigation.getByRole('link', { name: '메일' })).toHaveCount(0);

		await openUntilVisible(
			bottomNavigation.getByRole('button', { name: '더보기', exact: true }),
			page.getByRole('dialog').getByRole('link', { name: '메일' })
		);
	});

	test('opens the channel list from the messenger header on a narrow screen', async ({ page }) => {
		await page.setViewportSize({ width: 444, height: 866 });
		await page.goto('/messenger/');

		const openChannelList = page.getByRole('button', { name: '채널 목록 열기' });
		await expect(openChannelList).toBeVisible();

		await openUntilVisible(openChannelList, page.getByRole('dialog', { name: '채널' }));
	});

	test('keeps the end of every page scrollable clear of the mobile bottom navigation', async ({ page }) => {
		await page.setViewportSize({ width: 444, height: 866 });
		await page.goto('/memory/');

		const bottomNavigation = mobileBottomNavigation(page);
		await expect(bottomNavigation).toBeVisible();

		const metrics = await mobileBottomNavigationOverlayMetrics(page);
		expect(metrics.scrollPaddingBottom).toBeGreaterThanOrEqual(metrics.navigationHeight);
	});

	test('keeps task pagination controls reachable above the mobile overlay', async ({ page }) => {
		await page.setViewportSize({ width: 444, height: 866 });
		await page.route('**/runs/api**', async (route) => {
			await route.fulfill({ json: createTaskRunsFixture() });
		});
		await page.goto('/runs/');

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
			await route.fulfill({ contentType: 'text/html', body: '<title>internkim feedback</title>' });
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

function railTooltip(page: Page, label: string): Locator {
	return page.locator('[data-slot="tooltip-content"]', { hasText: label });
}

async function hoverUntilVisible(trigger: Locator, revealed: Locator): Promise<void> {
	await expect(async () => {
		await trigger.page().mouse.move(600, 600);
		await trigger.hover();
		await expect(revealed).toBeVisible({ timeout: 1_000 });
	}).toPass({ timeout: 15_000 });
}

async function openUntilVisible(trigger: Locator, revealed: Locator): Promise<void> {
	await expect(async () => {
		await trigger.click();
		await expect(revealed).toBeVisible({ timeout: 1_000 });
	}).toPass({ timeout: 15_000 });
}

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
	navigationHeight: number;
	scrollPaddingBottom: number;
}> {
	return page.evaluate(() => {
		const navigation = [...document.querySelectorAll('nav[aria-label]')].find((element) => element.textContent?.includes('더보기'));
		const scrollContainer = document.querySelector('[data-app-shell-scroll]');
		if (!(navigation instanceof HTMLElement)) throw new Error('mobile bottom navigation not found');
		if (!(scrollContainer instanceof HTMLElement)) throw new Error('app shell scroll container not found');

		const scrollContainerStyle = window.getComputedStyle(scrollContainer);
		const navigationRectangle = navigation.getBoundingClientRect();

		return {
			navigationHeight: navigationRectangle.height,
			scrollPaddingBottom: Number.parseFloat(scrollContainerStyle.paddingBottom)
		};
	});
}

function railItem(page: Page, label: string): Locator {
	return page.locator(`${appRailSelector} [data-sidebar="menu-item"]`).filter({ has: page.getByRole('link', { name: label, exact: true }) });
}

async function answerConversations(page: Page, conversations: RailConversation[]): Promise<void> {
	await page.route('**/agent/api/channels', async (route) => {
		await route.fulfill({ json: { conversations } });
	});
}
