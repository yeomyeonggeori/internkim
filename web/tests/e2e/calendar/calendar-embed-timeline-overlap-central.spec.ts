import { expect, test, type Page } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

test.describe('embedded calendar timeline overlap layout', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await routeCalendarHolidays(page);
	});

	test('splits overlapping day events into side-by-side, individually selectable columns', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{ title: 'Lane A', startISO: '2026-06-08T06:30:00+09:00', endISO: '2026-06-08T12:15:00+09:00' },
			{ title: 'Lane B', startISO: '2026-06-08T06:45:00+09:00', endISO: '2026-06-08T07:45:00+09:00' }
		]);
		try {
			await signInToCalendar(page);
			await openCalendarEmbed(page, '일');

			const first = page.locator(`[data-calendar-event-id="${eventIDs[0]}"]`);
			const second = page.locator(`[data-calendar-event-id="${eventIDs[1]}"]`);
			await expect(first).toBeVisible();
			await expect(second).toBeVisible();

			const [firstBox, secondBox] = await Promise.all([first.boundingBox(), second.boundingBox()]);
			if (!firstBox || !secondBox) throw new Error('Missing overlapping event boxes');
			const horizontallySeparate = firstBox.x + firstBox.width <= secondBox.x + 1 || secondBox.x + secondBox.width <= firstBox.x + 1;
			expect(horizontallySeparate).toBe(true);

			await second.click();
			await expect(second).toHaveAttribute('data-selected', '');
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});
});
