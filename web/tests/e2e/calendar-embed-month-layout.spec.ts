import { expect, test, type Page } from '@playwright/test';
import {
	cleanupCalendarEvents,
	seedCalendarEvents,
	signInToCalendar
} from './calendar-central-test-utils';

test.describe('embedded calendar month layout', () => {
	test.use({ locale: 'ko-KR' });

	test('keeps single-day month events inside their date cell', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{
				title: 'Single All Day Event',
				startISO: '2026-06-16T00:00:00+09:00',
				endISO: '2026-06-17T00:00:00+09:00',
				isAllDay: true
			},
			{
				title: 'Single Timed Event',
				startISO: '2026-06-16T08:00:00+09:00',
				endISO: '2026-06-16T09:00:00+09:00',
				isAllDay: false
			},
			{
				title: 'Single 24h Timed Event',
				startISO: '2026-06-21T00:00:00+09:00',
				endISO: '2026-06-22T00:00:00+09:00',
				isAllDay: false
			}
		]);
		const [allDayID, timedID, twentyFourHourID] = eventIDs;

		try {
			await openMonthView(page, '2026-06-16');

			await expect(page.locator(`[data-calendar-event-id="${allDayID}"]:visible`)).toHaveCount(1);
			await expect(page.locator(`[data-calendar-event-id="${timedID}"]:visible`)).toHaveCount(1);
			await expect(page.locator(`[data-calendar-event-id="${twentyFourHourID}"]:visible`)).toHaveCount(1);
			await expectEventWithinDateCell(page, allDayID, '2026-06-16');
			await expectEventWithinDateCell(page, timedID, '2026-06-16');
			await expectEventWithinDateCell(page, twentyFourHourID, '2026-06-21');
			await expect(page.locator(`[data-calendar-event-id="${timedID}"]:visible`)).toContainText('Single Timed Event');
			await expect(page.locator(`[data-calendar-event-id="${timedID}"]:visible`)).toContainText('8:00');
			await expect(page.locator(`[data-calendar-event-id="${twentyFourHourID}"]:visible`)).toContainText('Single 24h Timed Event');
			await expect(page.locator(`[data-calendar-event-id="${twentyFourHourID}"]:visible`)).toContainText('12:00');
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});

	test('shows no time label on a multi-day timed month event', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{
				title: 'Month Multi Day Timed',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			}
		]);
		const [multiDayTimedID] = eventIDs;

		try {
			await openMonthView(page, '2026-06-16');

			const chip = page.locator(`[data-calendar-event-id="${multiDayTimedID}"]:visible`).first();
			await expect(chip).toContainText('Month Multi Day Timed');
			await expect(chip).not.toContainText(/\d{1,2}:\d{2}/);
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});

	test('orders month events by day span before start time', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{
				title: 'Later Newer',
				startISO: '2026-06-10T10:00:00+09:00',
				endISO: '2026-06-10T11:00:00+09:00',
				isAllDay: false
			},
			{
				title: 'Early Older',
				startISO: '2026-06-10T08:00:00+09:00',
				endISO: '2026-06-10T09:00:00+09:00',
				isAllDay: false
			},
			{
				title: 'Long Event',
				startISO: '2026-06-10T00:00:00+09:00',
				endISO: '2026-06-13T00:00:00+09:00',
				isAllDay: true
			}
		]);
		const [laterNewerID, earlyOlderID, longEventID] = eventIDs;

		try {
			await openMonthView(page, '2026-06-10');
			await expect(page.locator(`[data-calendar-event-id="${laterNewerID}"]:visible`)).toHaveCount(1);
			await expect(page.locator(`[data-calendar-event-id="${earlyOlderID}"]:visible`)).toHaveCount(1);
			await expect(page.locator(`[data-calendar-event-id="${longEventID}"]:visible`)).toHaveCount(1);

			const eventTopByID = await page.evaluate(
				(ids) =>
					Object.fromEntries(
						ids.map((eventID) => {
							const element = document.querySelector<HTMLElement>(`[data-calendar-event-id="${eventID}"]`);
							if (!element) throw new Error(`Missing month event: ${eventID}`);
							return [eventID, Math.round(element.getBoundingClientRect().top)];
						})
					),
				[longEventID, earlyOlderID, laterNewerID]
			);
			expect(eventTopByID[longEventID]).toBeLessThan(eventTopByID[earlyOlderID]);
			expect(eventTopByID[earlyOlderID]).toBeLessThan(eventTopByID[laterNewerID]);
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});

	test('uses the same month block style for timed and all-day events', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{
				title: 'Style Timed',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:00:00+09:00',
				isAllDay: false
			},
			{
				title: 'Style All Day',
				startISO: '2026-06-09T00:00:00+09:00',
				endISO: '2026-06-10T00:00:00+09:00',
				isAllDay: true
			}
		]);
		const [timedID, allDayID] = eventIDs;

		try {
			await openMonthView(page, '2026-06-08');

			const [timedStyle, allDayStyle] = await Promise.all([
				chipStyle(page, timedID),
				chipStyle(page, allDayID)
			]);
			expect(timedStyle).toEqual(allDayStyle);
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});

	test('centres the server-rendered month on the requested date, not on the server clock', async ({ page }) => {
		await signInToCalendar(page);

		const serverHTML = await (await page.request.get('/calendar/embed?date=2026-06-16')).text();
		const serverWeekStarts = [...serverHTML.matchAll(/data-week-start="([^"]+)"/g)].map((match) => match[1]);
		const weekHoldingTheFirstOfJune = '2026-05-31';

		expect(serverWeekStarts[Math.floor(serverWeekStarts.length / 2)]).toBe(weekHoldingTheFirstOfJune);
	});
});

async function openMonthView(page: Page, dateKey: string): Promise<void> {
	await page.clock.setFixedTime(new Date(`${dateKey}T12:00:00`));
	await signInToCalendar(page);
	await page.goto(`/calendar/embed?date=${dateKey}`);
	await page.evaluate(() => {
		window.localStorage.setItem('internkim.calendar.view', 'month');
	});
	await page.reload();
	await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-month/);
	await expect(page.locator(`[data-calendar-date="${dateKey}"]`)).toBeVisible();
}

async function expectEventWithinDateCell(page: Page, eventID: string, dateKey: string): Promise<void> {
	const isWithinCell = await page.evaluate(
		({ eventID, dateKey }) => {
			const eventElement = document.querySelector<HTMLElement>(`[data-calendar-event-id="${eventID}"]`);
			const cellElement = document.querySelector<HTMLElement>(`[data-calendar-date="${dateKey}"]`);
			if (!eventElement || !cellElement) return false;
			const eventRectangle = eventElement.getBoundingClientRect();
			const cellRectangle = cellElement.getBoundingClientRect();
			return (
				eventRectangle.left >= cellRectangle.left - 1 &&
				eventRectangle.right <= cellRectangle.right + 1 &&
				eventRectangle.top >= cellRectangle.top - 1 &&
				eventRectangle.bottom <= cellRectangle.bottom + 1
			);
		},
		{ eventID, dateKey }
	);
	expect(isWithinCell).toBe(true);
}

async function chipStyle(page: Page, eventID: string): Promise<{ height: string; borderRadius: string }> {
	return page.locator(`[data-calendar-event-id="${eventID}"]:visible`).first().evaluate((element) => {
		const style = window.getComputedStyle(element);
		return { height: style.height, borderRadius: style.borderRadius };
	});
}
