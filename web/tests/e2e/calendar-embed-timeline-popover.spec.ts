import { expect, test, type Page } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';

test.describe('embedded calendar timeline popover anchors', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await routeCalendarHolidays(page);
	});

	test('opens a timeline event popover after a single click', async ({ page }) => {
		const [eventID] = await seedCalendarEvents([
			{ title: 'Single Click Timeline Event', startISO: '2026-06-08T09:00:00+09:00', endISO: '2026-06-08T10:00:00+09:00' }
		]);
		try {
			await openDesktopDayView(page);
			const eventChip = page.locator(`[data-calendar-event-id="${eventID}"]`);
			await expect(eventChip).toBeVisible();

			await eventChip.click();

			await expect(eventChip).toHaveAttribute('data-selected', '');
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();
			await expect(page.getByLabel('제목')).toHaveValue('Single Click Timeline Event');
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('opens popovers for timed, all-day, and multi-day week events', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{ title: 'Week Anchor Timed', startISO: '2026-06-08T09:00:00+09:00', endISO: '2026-06-08T10:00:00+09:00' },
			{
				title: 'Week Anchor All Day',
				startISO: '2026-06-09T00:00:00+09:00',
				endISO: '2026-06-10T00:00:00+09:00',
				isAllDay: true
			},
			{ title: 'Week Anchor Multi Day', startISO: '2026-06-08T11:45:00+09:00', endISO: '2026-06-10T12:30:00+09:00' }
		]);
		try {
			await openDesktopWeekView(page);

			for (const eventID of eventIDs) {
				const eventChip = page.locator(`[data-calendar-event-id="${eventID}"]`).first();
				await expect(eventChip).toBeVisible();
				await eventChip.scrollIntoViewIfNeeded();
				await eventChip.click();
				await expect(page.locator('.calendar-draft-popover'), `popover for ${eventID}`).toBeVisible();
				await page.keyboard.press('Escape');
				await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
			}
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

async function openDesktopDayView(page: Page): Promise<void> {
	await page.setViewportSize({ width: 1280, height: 900 });
	await signInToCalendar(page);
	await page.goto('/calendar/embed?date=2026-06-08');
	await page.evaluate(() => window.localStorage.setItem('internkim.calendar.view', 'day'));
	await page.reload();
	await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-day/);
}

async function openDesktopWeekView(page: Page): Promise<void> {
	await page.setViewportSize({ width: 1280, height: 900 });
	await signInToCalendar(page);
	await page.goto('/calendar/embed?date=2026-06-08');
	await page.evaluate(() => window.localStorage.setItem('internkim.calendar.view', 'week'));
	await page.reload();
	await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-week/);
}
