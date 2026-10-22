import { expect, test, type Page } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

async function scrollTimelineGrid(page: Page, deltaY: number): Promise<void> {
	await page.locator('[role="grid"]:not([aria-label="종일"])').first().evaluate((element, delta) => {
		element.scrollTop += delta;
		element.dispatchEvent(new Event('scroll', { bubbles: true }));
	}, deltaY);
}

async function scrollDraftPopover(page: Page, deltaY: number): Promise<void> {
	await page.locator('.calendar-draft-popover').evaluate((element, delta) => {
		element.scrollTop += delta;
		element.dispatchEvent(new Event('scroll', { bubbles: true }));
	}, deltaY);
}

test.describe('embedded calendar timeline scroll guards', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await routeCalendarHolidays(page);
	});

	test('dismisses a day event popover on grid scroll without dismissing internal popover scroll', async ({ page }) => {
		const [eventID] = await seedCalendarEvents([
			{ title: '스크롤 확인 일정', startISO: '2026-06-08T09:00:00+09:00', endISO: '2026-06-08T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);
			await openCalendarEmbed(page, '일');

			await page.locator(`[data-calendar-event-id="${eventID}"]`).click();
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();

			await scrollDraftPopover(page, 24);
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();

			await scrollTimelineGrid(page, 120);
			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('dismisses a week event popover on grid scroll without dismissing internal popover scroll', async ({ page }) => {
		const [eventID] = await seedCalendarEvents([
			{ title: '주간 스크롤 일정', startISO: '2026-06-08T09:00:00+09:00', endISO: '2026-06-08T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);
			await openCalendarEmbed(page, '주');

			await page.locator(`[data-calendar-event-id="${eventID}"]`).click();
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();

			await scrollDraftPopover(page, 24);
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();

			await scrollTimelineGrid(page, 120);
			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});
});
