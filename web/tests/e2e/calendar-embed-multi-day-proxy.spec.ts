import { expect, test, type Page } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';

test.describe('embedded calendar multi-day timed event interactions', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-16T12:00:00'));
		await routeCalendarHolidays(page);
	});

	test('shows a multi-day timed event on every day it spans and opens it for editing', async ({ page }) => {
		const [eventID] = await seedCalendarEvents([
			{ title: '주간 멀티', startISO: '2026-06-16T11:45:00+09:00', endISO: '2026-06-18T12:30:00+09:00' }
		]);
		try {
			await openDesktopWeekView(page);

			for (const dateKey of ['2026-06-16', '2026-06-17', '2026-06-18']) {
				await expect(page.locator(`[data-calendar-date="${dateKey}"] [data-calendar-event-id="${eventID}"]`)).toBeVisible();
			}

			const editableChip = page.locator(`[data-calendar-date="2026-06-16"] [data-calendar-event-id="${eventID}"]`);
			await editableChip.scrollIntoViewIfNeeded();
			await editableChip.click();
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();
			await expect(page.getByLabel('제목')).toHaveValue('주간 멀티');
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('does not overlap a multi-day timed event with a genuine all-day event on the same days', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{ title: '주간 종일', startISO: '2026-06-16T00:00:00+09:00', endISO: '2026-06-19T00:00:00+09:00', isAllDay: true },
			{ title: '주간 시간 다일', startISO: '2026-06-16T11:00:00+09:00', endISO: '2026-06-18T12:00:00+09:00' }
		]);
		try {
			await openDesktopWeekView(page);

			const allDayEvent = page.locator(`[data-calendar-event-id="${eventIDs[0]}"]`).first();
			const timedEvent = page.locator(`[data-calendar-date="2026-06-16"] [data-calendar-event-id="${eventIDs[1]}"]`);
			await expect(allDayEvent).toBeVisible();
			await expect(timedEvent).toBeVisible();
			const allDayBox = await allDayEvent.boundingBox();
			const timedBox = await timedEvent.boundingBox();
			expect(allDayBox).not.toBeNull();
			expect(timedBox).not.toBeNull();
			if (!allDayBox || !timedBox) throw new Error('missing bounding boxes');
			const verticallyOverlap = allDayBox.y < timedBox.y + timedBox.height && timedBox.y < allDayBox.y + allDayBox.height;
			expect(verticallyOverlap).toBe(false);
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});
});

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

async function openDesktopWeekView(page: Page): Promise<void> {
	await page.setViewportSize({ width: 1280, height: 900 });
	await signInToCalendar(page);
	await page.goto('/calendar/embed?date=2026-06-16');
	await page.evaluate(() => window.localStorage.setItem('internkim.calendar.view', 'week'));
	await page.reload();
	await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-week/);
}
