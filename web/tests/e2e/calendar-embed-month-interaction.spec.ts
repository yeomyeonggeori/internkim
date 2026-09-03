import { expect, test, type Locator, type Page } from '@playwright/test';
import {
	cleanupCalendarEvents,
	seedCalendarEvents,
	signInToCalendar
} from './calendar-central-test-utils';
import { waitForClientHydration } from './calendar-draft-popover-test-utils';

test.describe('embedded calendar month interactions', () => {
	test.use({ locale: 'ko-KR' });

	test('opens a month event editor when activated from the keyboard', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{
				title: 'Keyboard Selected Month Event',
				startISO: '2026-06-09T00:00:00+09:00',
				endISO: '2026-06-10T00:00:00+09:00',
				isAllDay: true
			}
		]);
		const [eventID] = eventIDs;

		try {
			await openMonthView(page, '2026-06-09');
			const eventBlock = page.locator(`[data-calendar-event-id="${eventID}"]:visible`).first();
			await eventBlock.focus();
			await page.keyboard.press('Enter');

			await expect(page.locator('.calendar-draft-popover')).toBeVisible();
			await expect(page.getByLabel('제목')).toHaveValue('Keyboard Selected Month Event');
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});

	test('opens a month event popover after a single click', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{
				title: 'Single Click Month Event',
				startISO: '2026-06-09T09:00:00+09:00',
				endISO: '2026-06-09T10:00:00+09:00',
				isAllDay: false
			}
		]);
		const [eventID] = eventIDs;

		try {
			await openMonthView(page, '2026-06-09');
			await page.locator(`[data-calendar-event-id="${eventID}"]:visible`).first().click();

			await expect(page.locator('.calendar-draft-popover')).toBeVisible();
			await expect(page.getByLabel('제목')).toHaveValue('Single Click Month Event');
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});

	test('moves the selected month date with arrow keys', async ({ page }) => {
		await openMonthView(page, '2026-06-17');
		await page.locator('[data-calendar-date="2026-06-17"]').click({ position: { x: 40, y: 12 } });
		await expect.poll(async () => selectedMonthDateKey(page)).toBe('2026-06-17');

		await page.keyboard.press('ArrowRight');
		await expect.poll(async () => selectedMonthDateKey(page)).toBe('2026-06-18');

		await page.keyboard.press('ArrowDown');
		await expect.poll(async () => selectedMonthDateKey(page)).toBe('2026-06-25');

		await page.keyboard.press('ArrowLeft');
		await expect.poll(async () => selectedMonthDateKey(page)).toBe('2026-06-24');

		await page.keyboard.press('ArrowUp');
		await expect.poll(async () => selectedMonthDateKey(page)).toBe('2026-06-17');
	});

	test('keeps month scroll position stable when opening an event popover', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{
				title: 'Stable Scroll Event',
				startISO: '2026-06-16T09:00:00+09:00',
				endISO: '2026-06-16T10:00:00+09:00',
				isAllDay: false
			}
		]);
		const [eventID] = eventIDs;

		try {
			await openMonthView(page, '2026-06-16');
			const scroller = monthScroller(page);
			await expect(page.locator(`[data-calendar-event-id="${eventID}"]:visible`)).toHaveCount(1);
			const initialScrollTop = await settledScrollTop(scroller);

			await page.locator(`[data-calendar-event-id="${eventID}"]:visible`).first().click();
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();

			await expect.poll(async () => scroller.evaluate((element) => element.scrollTop)).toBe(initialScrollTop);
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});

	test('closes the event popover when clicking an empty date cell', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{
				title: 'Clear Selection Event',
				startISO: '2026-06-09T09:00:00+09:00',
				endISO: '2026-06-09T10:00:00+09:00',
				isAllDay: false
			}
		]);
		const [eventID] = eventIDs;

		try {
			await openMonthView(page, '2026-06-09');
			const eventChip = page.locator(`[data-calendar-event-id="${eventID}"]:visible`).first();
			await expect(eventChip).toBeVisible();
			await eventChip.click();
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();

			await page.locator('[data-calendar-date="2026-06-11"]').click({ position: { x: 40, y: 44 } });
			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});

	test('scrolls the month view on wheel gestures', async ({ page }) => {
		await openMonthView(page, '2026-06-15');
		const scroller = monthScroller(page);
		const initialScrollTop = await scroller.evaluate((element) => element.scrollTop);

		await scroller.hover();
		await page.mouse.wheel(0, 640);

		await expect.poll(async () => scroller.evaluate((element) => element.scrollTop)).toBeGreaterThan(initialScrollTop);
	});
});

function monthScroller(page: Page): Locator {
	return page.locator('.calendar-stage [role="grid"]');
}

async function settledScrollTop(scroller: Locator): Promise<number> {
	let previousScrollTop = -1;
	let currentScrollTop = await scroller.evaluate((element) => element.scrollTop);
	while (currentScrollTop !== previousScrollTop) {
		previousScrollTop = currentScrollTop;
		await scroller.page().waitForTimeout(150);
		currentScrollTop = await scroller.evaluate((element) => element.scrollTop);
	}
	return currentScrollTop;
}

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
	await waitForClientHydration(page);
}

async function selectedMonthDateKey(page: Page): Promise<string> {
	return page.evaluate(() => document.querySelector('[data-calendar-date][data-selected]')?.getAttribute('data-calendar-date') ?? '');
}
