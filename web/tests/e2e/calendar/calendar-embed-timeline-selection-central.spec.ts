import { expect, test, type Page } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

test.describe('embedded calendar timeline selection', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await routeCalendarHolidays(page);
	});

	test('marks a clicked day event as selected and colors it with the event accent color', async ({ page }) => {
		const [eventID] = await seedCalendarEvents([
			{ title: '선택 확인 일정', startISO: '2026-06-08T09:00:00+09:00', endISO: '2026-06-08T10:00:00+09:00' }
		]);
		try {
			await signInToCalendar(page);
			await openCalendarEmbed(page, '일');

			const eventChip = page.locator(`[data-calendar-event-id="${eventID}"]`);
			await expect(eventChip).toBeVisible();
			await expect(eventChip).not.toHaveAttribute('data-selected', '');

			await eventChip.click();
			await expect(eventChip).toHaveAttribute('data-selected', '');
			await expect(eventChip).toHaveCSS('background-color', 'rgb(59, 130, 246)');
			await expect(eventChip).toHaveCSS('color', 'rgb(255, 255, 255)');
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});
});
