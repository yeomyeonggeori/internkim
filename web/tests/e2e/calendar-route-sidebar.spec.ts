import { expect, test, type Page } from '@playwright/test';

test.describe('calendar route sidebar', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarShellAPI(page);
	});

	test('navigates the embedded calendar from the mini month', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/');

		const calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.getByRole('button', { name: '오늘' })).toBeVisible();
		await page.getByRole('button', { name: '다음 달' }).click();
		await page.locator('[data-mini-date-key="2026-07-15"]').click();
		await expect(page.locator('[data-mini-date-key="2026-07-15"]')).toHaveAttribute('aria-pressed', 'true');
		await expect(calendarFrame.getByRole('heading', { name: '2026년 7월' })).toBeVisible();

		await expect
			.poll(async () =>
				page.evaluate(() => window.localStorage.getItem('internkim.calendar.visibleDate')?.startsWith('2026-07'))
			)
			.toBe(true);
	});

	test('renders reference mini month visual states', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/');

		const sundayHeader = page.locator('[data-mini-weekday-index="0"]');
		const weekendDate = page.locator('[data-mini-date-key="2026-06-14"]');
		const eventDotSlot = page.locator('[data-mini-date-key="2026-06-15"] .mini-month-event-dot-slot');
		const emptyDotSlot = page.locator('[data-mini-date-key="2026-06-16"] .mini-month-event-dot-slot');

		await expect(sundayHeader).toHaveAttribute('data-weekend', 'true');
		await expect(sundayHeader).toHaveClass(/text-red-400/);
		await expect(weekendDate).toHaveAttribute('data-weekend', 'true');
		await expect(weekendDate).toHaveClass(/text-red-400/);
		await expect(eventDotSlot).toHaveAttribute('data-has-event', 'true');
		await expect(emptyDotSlot).toHaveAttribute('data-has-event', 'false');

		await page.locator('[data-mini-date-key="2026-06-16"]').click();
		await expect(page.locator('[data-mini-date-key="2026-06-16"]')).toHaveAttribute('data-selected', 'true');
		await expect(page.locator('[data-mini-date-key="2026-06-16"]')).toHaveClass(/bg-transparent/);
		await expect(page.locator('[data-mini-date-key="2026-06-16"]')).toHaveClass(/rounded-none/);
	});

	test('syncs embedded toolbar date and view changes to the sidebar', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/');

		const calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.getByRole('button', { name: '오늘' })).toBeVisible();
		await calendarFrame.getByRole('button', { name: '주', exact: true }).click();

		await expect
			.poll(async () => page.evaluate(() => window.localStorage.getItem('internkim.calendar.view')))
			.toBe('week');
		await expect(page.locator('[data-mini-date-key="2026-06-08"]')).toHaveAttribute('data-week-range', 'middle');

		await calendarFrame.getByRole('button', { name: '다음' }).click();
		await expect(page.locator('[data-mini-date-key="2026-06-15"]')).toHaveAttribute('aria-pressed', 'true');
		await expect(page.locator('[data-mini-date-key="2026-06-15"]')).toHaveAttribute('data-week-range', 'middle');

		await page.reload();
		const reloadedCalendarFrame = page.frameLocator('iframe');
		await expect(reloadedCalendarFrame.getByRole('button', { name: '주', exact: true })).toHaveClass(/active-view/);
		await expect(page.locator('[data-mini-date-key="2026-06-15"]')).toHaveAttribute('aria-pressed', 'true');
		await expect(page.locator('[data-mini-date-key="2026-06-15"]')).toHaveAttribute('data-week-range', 'middle');
	});

	test('keeps subscription copy actions disabled when values are empty', async ({ page }) => {
		await page.goto('/calendar/');
		await page.getByRole('button', { name: '구독 설정' }).click();

		const copyButtons = page.getByRole('button', { name: 'Copy' });
		await expect(copyButtons).toHaveCount(4);
		for (const index of [0, 1, 2, 3]) {
			await expect(copyButtons.nth(index)).toBeDisabled();
		}
	});

	test('does not create horizontal overflow on calendar routes', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/calendar/');
		await expect(page.locator('iframe')).toBeVisible();
		await expectHorizontalOverflow(page, false);

		await page.goto('/calendar/embed');
		await expect(page.getByRole('button', { name: '오늘' })).toBeVisible();
		await expectHorizontalOverflow(page, false);
	});
});

async function routeCalendarShellAPI(page: Page): Promise<void> {
	await page.route('**/admin/api/session', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
	});
	await page.route('**/calendar/api/sync', async (route) => {
		await route.fulfill({
			json: {
				caldavURL: '',
				caldavUsername: '',
				caldavPassword: '',
				icsURL: ''
			}
		});
	});
	await page.route('**/calendar/api/account-status', async (route) => {
		await route.fulfill({ json: { connected: false, needsReauth: false } });
	});
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({
			json: {
				events: [
					{
						id: 'event-2026-06-15',
						title: 'Sidebar event',
						startISO: '2026-06-15T09:00:00.000Z',
						endISO: '2026-06-15T10:00:00.000Z',
						isAllDay: false
					}
				]
			}
		});
	});
	await page.route('**/calendar/api/remote-sync', async (route) => {
		await route.fulfill({ json: { synced: false } });
	});
	await page.route('**/calendar/api/conflicts', async (route) => {
		await route.fulfill({ json: { conflicts: [] } });
	});
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
}

async function expectHorizontalOverflow(page: Page, expected: boolean): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
		)
		.toBe(expected);
}
