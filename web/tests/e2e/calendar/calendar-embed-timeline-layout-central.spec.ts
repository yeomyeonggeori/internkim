import { expect, test, type Page } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

test.describe('embedded calendar timeline layout', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await routeCalendarHolidays(page);
	});

	test('renders a 24-row hour grid and an all-day row for the day and week timelines', async ({ page }) => {
		await signInToCalendar(page);

		await openCalendarEmbed(page, '일');
		await expect(page.locator('[role="grid"][aria-label="종일"]')).toBeVisible();
		await expect(page.locator('[data-calendar-date="2026-06-08"]')).toBeVisible();

		await openCalendarEmbed(page, '주');
		await expect(page.locator('[role="grid"][aria-label="종일"]')).toBeVisible();
		await expect(page.locator('[data-calendar-date="2026-06-08"]')).toBeVisible();
	});

	test('shows the day event list in the day view sidebar and hides it in week view', async ({ page }) => {
		const [eventID] = await seedCalendarEvents([
			{ title: '사이드바 일정', startISO: '2026-06-08T09:00:00+09:00', endISO: '2026-06-08T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);

			await openCalendarEmbed(page, '일');
			await expect(page.locator('[data-testid="calendar-event-card"]', { hasText: '사이드바 일정' })).toBeVisible();

			await openCalendarEmbed(page, '주');
			await expect(page.locator('[data-testid="calendar-event-card"]')).toHaveCount(0);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});
});
