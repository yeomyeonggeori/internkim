import { expect, test } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';
import {
	cancelMonthRangeDrag,
	routeCalendarHolidays,
	routeEventUpdate,
	scrollMonthViewBy,
	startDragBetweenCells,
	waitForClientHydration
} from './calendar-draft-popover-test-utils';

test.describe('calendar draft popover anchors', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await routeCalendarHolidays(page);
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await signInToCalendar(page);
	});

	test('keeps the edit popover within the viewport when it opens near the bottom edge', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 560 });
		const [eventID] = await seedCalendarEvents([
			{ title: '하단 편집 일정', startISO: '2026-06-17T00:00:00+09:00', endISO: '2026-06-18T00:00:00+09:00', isAllDay: true }
		]);
		try {
			await page.goto('/calendar/embed');
			await waitForClientHydration(page);
			const chip = page.locator(`[data-calendar-event-id="${eventID}"]`).first();
			await expect(chip).toBeVisible();
			await chip.click();
			const popover = page.locator('.calendar-draft-popover');
			await expect(popover).toBeVisible();

			await expect
				.poll(async () => popover.evaluate((element) => element.getBoundingClientRect().bottom))
				.toBeLessThanOrEqual(560);
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('closes month edit popovers immediately when the calendar scrolls', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 560 });
		const updatedPayloads = await routeEventUpdate(page);
		const [eventID] = await seedCalendarEvents([
			{ title: '스크롤 기준 일정', startISO: '2026-06-10T09:00:00+09:00', endISO: '2026-06-10T10:00:00+09:00' }
		]);
		try {
			await page.goto('/calendar/embed');
			await waitForClientHydration(page);
			const chip = page.locator(`[data-calendar-event-id="${eventID}"]`);
			await expect(chip).toBeVisible();
			await chip.click();
			const popover = page.locator('.calendar-draft-popover');
			await expect(popover).toBeVisible();
			await page.getByLabel('제목').fill('스크롤 저장 일정');

			await scrollMonthViewBy(page, 80);

			await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
			await expect.poll(() => updatedPayloads.length).toBe(1);
			expect(updatedPayloads[0]).toMatchObject({ title: '스크롤 저장 일정' });
		} finally {
			await cleanupCalendarEvents([eventID]);
		}
	});

	test('cancels a dragged month range on pointer cancel without opening a draft', async ({ page }) => {
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await startDragBetweenCells(page, '2026-06-10', '2026-06-12');
		await expect(page.locator('[data-calendar-event-id="calendar-draft-preview"]')).toBeVisible();

		await cancelMonthRangeDrag(page);
		await page.mouse.up();

		await expect(page.locator('[data-calendar-event-id="calendar-draft-preview"]')).toHaveCount(0);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});
});
