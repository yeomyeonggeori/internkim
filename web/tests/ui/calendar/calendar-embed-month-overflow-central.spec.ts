import { expect, test, type Locator, type Page } from '@playwright/test';
import {
	calendarEmbedPath,
	cleanupCalendarEvents,
	seedCalendarEvents,
	signInToCalendar
} from './calendar-central-test-utils';

test.describe('embedded calendar month overflow', () => {
	test.use({ locale: 'ko-KR' });

	test('collapses overflowing same-day month events behind a more button', async ({ page }) => {
		const eventIDs = await seedCalendarEvents(
			Array.from({ length: 8 }, (_, index) => ({
				title: `Overflow Short ${index}`,
				startISO: `2026-06-17T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-17T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false
			}))
		);

		try {
			await openMonthView(page, '2026-06-17');
			await expect(page.locator(`[data-calendar-event-id="${eventIDs[0]}"]:visible`)).toHaveCount(1);

			const moreButton = monthMoreButton(page, '2026-06-17');
			await expect(moreButton).toBeVisible();
			await expect(moreButton).toContainText(/\+\d+ 더보기/);
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});

	test('opens the day view for that date when the more button is clicked', async ({ page }) => {
		const eventIDs = await seedCalendarEvents(
			Array.from({ length: 8 }, (_, index) => ({
				title: `Keyboard Overflow ${index}`,
				startISO: `2026-06-17T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-17T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false
			}))
		);

		try {
			await openMonthView(page, '2026-06-16');
			const moreButton = monthMoreButton(page, '2026-06-17');
			await expect(moreButton).toBeVisible();
			await moreButton.click();

			await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-day/);
			await expect(page.locator(`[data-calendar-event-id="${eventIDs[0]}"]:visible`)).toHaveCount(1);
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});
});

function monthMoreButton(page: Page, dateKey: string): Locator {
	return page.locator(`[data-calendar-date="${dateKey}"] button:not([data-calendar-event-id])`);
}

async function openMonthView(page: Page, dateKey: string): Promise<void> {
	await page.clock.setFixedTime(new Date(`${dateKey}T12:00:00`));
	await signInToCalendar(page);
	await page.goto(`${calendarEmbedPath}?date=${dateKey}`);
	await page.evaluate(() => {
		window.localStorage.setItem('internkim.calendar.view', 'month');
	});
	await page.reload();
	await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-month/);
	await expect(page.locator(`[data-calendar-date="${dateKey}"]`)).toBeVisible();
}
