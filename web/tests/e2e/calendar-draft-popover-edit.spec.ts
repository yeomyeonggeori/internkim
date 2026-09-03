import { expect, test } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';
import { routeCalendarHolidays, routeEventDelete, waitForClientHydration } from './calendar-draft-popover-test-utils';

test.describe('calendar draft popover edit mode', () => {
	test.use({ locale: 'ko-KR' });

	test('opens existing events in edit mode and deletes them from the popover', async ({ page }) => {
		await routeCalendarHolidays(page);
		const deletedPayloads = await routeEventDelete(page);
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		const [eventID] = await seedCalendarEvents([
			{
				title: '기존 일정',
				note: '기존 설명',
				startISO: '2026-06-10T09:00:00+09:00',
				endISO: '2026-06-10T10:00:00+09:00'
			}
		]);
		try {
			await signInToCalendar(page);
			await page.goto('/calendar/embed');
			await waitForClientHydration(page);

			const chip = page.locator(`[data-calendar-event-id="${eventID}"]`);
			await expect(chip).toBeVisible();
			await chip.click();
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();
			await expect(page.getByLabel('제목')).toHaveValue('기존 일정');

			await page.getByRole('button', { name: '삭제' }).click();

			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
			await expect(page.locator(`[data-calendar-event-id="${eventID}"]`)).toHaveCount(0);
			await expect(page.locator('.calendar-delete-undo-toast')).toBeVisible();

			await expect.poll(() => deletedPayloads.length, { timeout: 12_000 }).toBe(1);
			expect(deletedPayloads[0]).toMatchObject({ eventHint: eventID });
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});
});
