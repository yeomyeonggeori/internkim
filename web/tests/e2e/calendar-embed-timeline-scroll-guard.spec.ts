import { expect, test, type Page } from '@playwright/test';
import { routeCalendarEventUpdates, routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import { dispatchElementScroll, clickCalendarEvent, navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';
import {
	duplicateDayAnchorTimelineEventSelector,
	routeDuplicateDayAnchorEvent
} from './calendar-embed-timeline-fixtures';

test.describe('embedded calendar timeline scroll and pointer guards', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('does not open a day event popover after a moved pointer gesture', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'moved-pointer-day-event',
				title: 'Moved Pointer Day Event',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '일');
		const eventSelector = '.calendar-stage [data-event-id="moved-pointer-day-event"].df-day-event:not(.df-right-panel-event-card)';
		await expect(page.locator(eventSelector)).toBeVisible();

		await dispatchMovedPointerActivation(page, eventSelector);

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('does not open a day event popover when only pointer up reports movement', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'pointer-up-moved-day-event',
				title: 'Pointer Up Moved Day Event',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '일');
		const eventSelector =
			'.calendar-stage [data-event-id="pointer-up-moved-day-event"].df-day-event:not(.df-right-panel-event-card)';
		await expect(page.locator(eventSelector)).toBeVisible();

		await dispatchMovedPointerActivation(page, eventSelector, { dispatchPointerMove: false });

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('does not open a day event popover after a touch scroll gesture', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await routeCalendarEvents(page, [
			{
				id: 'touch-scroll-day-event',
				title: 'Touch Scroll Day Event',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '일');
		const eventSelector = '.calendar-stage [data-event-id="touch-scroll-day-event"].df-day-event:not(.df-right-panel-event-card)';
		await expect(page.locator(eventSelector)).toBeVisible();

		await dispatchMovedPointerActivation(page, eventSelector, { pointerType: 'touch' });

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('dismisses unchanged day edit popovers on calendar scroll without saving or dismissing internal popover scroll', async ({ page }) => {
		const updatedEvents = await routeCalendarEventUpdates(page);
		await routeDuplicateDayAnchorEvent(page);

		await openCalendarEmbed(page, '일');
		await clickCalendarEvent(page, duplicateDayAnchorTimelineEventSelector);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();

		await dispatchElementScroll(page, '.draft-popover-body', 24);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();

		const unexpectedUpdate = page
			.waitForRequest((request) => request.method() === 'PUT' && request.url().includes('/calendar/api/events/'), {
				timeout: 500
			})
			.then(() => true, () => false);
		await dispatchElementScroll(page, '.df-day-content-grid', 120);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(unexpectedUpdate).resolves.toBe(false);
		expect(updatedEvents).toHaveLength(0);
	});

	test('dismisses unchanged week edit popovers on calendar scroll without saving or dismissing internal popover scroll', async ({ page }) => {
		const updatedEvents = await routeCalendarEventUpdates(page);
		await routeCalendarEvents(page, [
			{
				id: 'week-scroll-dismiss-event',
				title: 'Week Scroll Dismiss Event',
				startISO: '2026-06-16T09:00:00+09:00',
				endISO: '2026-06-16T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		const eventSelector = '.calendar-stage [data-event-id="week-scroll-dismiss-event"].df-week-event.df-event-timed';
		await expect(page.locator(eventSelector)).toBeVisible();

		await clickCalendarEvent(page, eventSelector);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();

		await dispatchElementScroll(page, '.draft-popover-body', 24);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();

		const unexpectedUpdate = page
			.waitForRequest((request) => request.method() === 'PUT' && request.url().includes('/calendar/api/events/'), {
				timeout: 500
			})
			.then(() => true, () => false);
		await dispatchElementScroll(page, '.df-week-time-grid-scroller', 120);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(unexpectedUpdate).resolves.toBe(false);
		expect(updatedEvents).toHaveLength(0);
	});

	test('does not open a week all-day event popover after a moved pointer gesture', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'moved-pointer-week-all-day-event',
				title: 'Moved Pointer Week All Day Event',
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-18T00:00:00+09:00',
				isAllDay: true
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		const eventSelector = '.calendar-stage .df-week-all-day-event-layer [data-event-id="moved-pointer-week-all-day-event"]';
		await expect(page.locator(eventSelector)).toBeVisible();

		await dispatchMovedPointerActivation(page, eventSelector, { pointerType: 'touch' });

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});
});

type MovedPointerActivationOptions = {
	dispatchPointerMove?: boolean;
	pointerType?: string;
};

async function dispatchMovedPointerActivation(page: Page, selector: string, options: MovedPointerActivationOptions = {}): Promise<void> {
	await page.evaluate(async ({ dispatchPointerMove, targetSelector, pointerType }) => {
		const target = document.querySelector<HTMLElement>(targetSelector);
		if (!target) throw new Error(`Missing calendar event: ${targetSelector}`);
		target.scrollIntoView({ block: 'center', inline: 'nearest' });
		await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
		const rectangle = target.getBoundingClientRect();
		const startClientX = rectangle.left + Math.min(32, rectangle.width / 2);
		const startClientY = rectangle.top + Math.min(24, rectangle.height / 2);
		const movedClientY = startClientY + 32;
		const pointerID = 25;
		const startPointerEvent: PointerEventInit = {
			bubbles: true,
			cancelable: true,
			button: 0,
			pointerId: pointerID,
			pointerType,
			clientX: startClientX,
			clientY: startClientY
		};
		const movedPointerEvent: PointerEventInit = {
			...startPointerEvent,
			clientY: movedClientY
		};
		const movedMouseEvent: MouseEventInit = {
			bubbles: true,
			cancelable: true,
			button: 0,
			clientX: startClientX,
			clientY: movedClientY
		};
		target.dispatchEvent(new PointerEvent('pointerdown', startPointerEvent));
		if (dispatchPointerMove) document.dispatchEvent(new PointerEvent('pointermove', movedPointerEvent));
		window.dispatchEvent(new PointerEvent('pointerup', movedPointerEvent));
		target.dispatchEvent(new MouseEvent('click', movedMouseEvent));
	}, {
		dispatchPointerMove: options.dispatchPointerMove ?? true,
		targetSelector: selector,
		pointerType: options.pointerType ?? 'mouse'
	});
}
