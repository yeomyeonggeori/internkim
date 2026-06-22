import { expect, test, type Page } from '@playwright/test';
import { routeCalendarEventUpdates, routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import {
	expectCalendarEventSelectedBlue,
	expectCalendarEventSelectedOnPointerDown,
	expectCalendarEventTitleAndTime,
	expectRightPanelEventCardsShareBlockStyle
} from './calendar-embed-interaction-assertions';
import {
	expectPopoverArrowPointsToElement,
	expectPopoverArrowPointsToEventTimeEnd,
	expectPopoverOpensLeftOfElement
} from './calendar-embed-draft-assertions';
import { dispatchElementScroll, doubleClickCalendarEvent, navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';
import {
	clearCalendarSelectionFromTestPage,
	duplicateDayAnchorPanelEventSelector,
	duplicateDayAnchorTimelineEventSelector,
	routeDuplicateDayAnchorEvent,
	routeRightPanelMixedEvents
} from './calendar-embed-timeline-fixtures';

test.describe('embedded calendar timeline popover and selection behavior', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('uses consistent right panel event cards for all-day and timed day events', async ({ page }) => {
		await routeRightPanelMixedEvents(page);

		await openCalendarEmbed(page, '일');
		const selectors = [
			'right-panel-all-day-a',
			'right-panel-all-day-b',
			'right-panel-all-day-c',
			'right-panel-timed-a',
			'right-panel-timed-b'
		].map((eventID) => `.calendar-stage [data-event-id="${eventID}"].df-right-panel-event-card`);
		for (const selector of selectors) {
			await expect(page.locator(selector)).toBeVisible();
		}

		await expectRightPanelEventCardsShareBlockStyle(page, selectors);
	});

	test('selects the matching right panel all-day card from the all-day row', async ({ page }) => {
		await routeRightPanelMixedEvents(page);

		await openCalendarEmbed(page, '일');
		const allDayRowEventSelector = '.calendar-stage .df-day-content-all-day-lane [data-event-id="right-panel-all-day-a"]';
		const panelEventSelector = '.calendar-stage [data-event-id="right-panel-all-day-a"].df-right-panel-event-card';

		await expect(page.locator(allDayRowEventSelector)).toBeVisible();
		await expect(page.locator(panelEventSelector)).toBeVisible();
		await expectCalendarEventSelectedOnPointerDown(page, allDayRowEventSelector, panelEventSelector);
		await expectCalendarEventSelectedBlue(page, panelEventSelector);
	});

	test('opens right panel all-day event popovers to the left of the clicked card', async ({ page }) => {
		await routeRightPanelMixedEvents(page);

		await openCalendarEmbed(page, '일');
		const panelEventSelector = '.calendar-stage [data-event-id="right-panel-all-day-a"].df-right-panel-event-card';
		await expect(page.locator(panelEventSelector)).toBeVisible();

		await doubleClickCalendarEvent(page, panelEventSelector);

		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expectPopoverOpensLeftOfElement(page, panelEventSelector);
		await expectPopoverArrowPointsToElement(page, panelEventSelector);
	});

	test('opens a timeline event popover only on double click after selecting it', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'double-click-timeline-event',
				title: 'Double Click Timeline Event',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '일');
		const eventBlock = page.locator(
			'.calendar-stage [data-event-id="double-click-timeline-event"].df-day-event:not(.df-right-panel-event-card)'
		);
		await expect(eventBlock).toBeVisible();
		await eventBlock.click();

		await expect(eventBlock).toHaveClass(/internkim-calendar-event-focused/);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);

		await eventBlock.dblclick();

		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.getByLabel('제목')).toHaveValue('Double Click Timeline Event');
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

	test('anchors day multi-day proxy popovers to the clicked proxy block', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'day-proxy-anchor-event',
				title: 'Day Proxy Anchor Event',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '일');
		await navigateEmbeddedCalendar(page, '2026-06-17');
		const proxySelector = '.calendar-stage .calendar-multi-day-all-day-proxy[data-event-id="day-proxy-anchor-event::multi-day-proxy"]';
		await expect(page.locator(proxySelector)).toBeVisible();
		await expect(page.locator('.calendar-stage .df-day-event.df-event-timed[data-event-id="day-proxy-anchor-event"]:visible')).toHaveCount(0);

		await doubleClickCalendarEvent(page, proxySelector);

		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expectPopoverArrowPointsToElement(page, proxySelector);
	});

	test('anchors day edit popovers to the clicked timeline event block when a right panel duplicate exists', async ({ page }) => {
		await routeDuplicateDayAnchorEvent(page);

		await openCalendarEmbed(page, '일');
		const timelineEvent = page.locator(duplicateDayAnchorTimelineEventSelector);
		const panelEvent = page.locator(duplicateDayAnchorPanelEventSelector);
		await expect(timelineEvent).toBeVisible();
		await expect(panelEvent).toBeVisible();

		await doubleClickCalendarEvent(page, duplicateDayAnchorTimelineEventSelector);

		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expectPopoverArrowPointsToEventTimeEnd(page, duplicateDayAnchorTimelineEventSelector);
	});

	test('anchors day edit popovers to the clicked right panel event block when a timeline duplicate exists', async ({ page }) => {
		await routeDuplicateDayAnchorEvent(page);

		await openCalendarEmbed(page, '일');
		const timelineEvent = page.locator(duplicateDayAnchorTimelineEventSelector);
		const panelEvent = page.locator(duplicateDayAnchorPanelEventSelector);
		await expect(timelineEvent).toBeVisible();
		await expect(panelEvent).toBeVisible();

		await doubleClickCalendarEvent(page, duplicateDayAnchorPanelEventSelector);

		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expectPopoverOpensLeftOfElement(page, duplicateDayAnchorPanelEventSelector);
		await expectPopoverArrowPointsToElement(page, duplicateDayAnchorPanelEventSelector);
	});

	test('dismisses day edit popovers on calendar scroll without dismissing internal popover scroll', async ({ page }) => {
		const updatedEvents = await routeCalendarEventUpdates(page);
		await routeDuplicateDayAnchorEvent(page);

		await openCalendarEmbed(page, '일');
		await doubleClickCalendarEvent(page, duplicateDayAnchorTimelineEventSelector);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();

		await dispatchElementScroll(page, '.draft-popover-body', 24);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();

		await dispatchElementScroll(page, '.df-day-content-grid', 120);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect.poll(() => updatedEvents.length).toBe(1);
	});

	test('keeps duplicated day event selection and timed labels aligned', async ({ page }) => {
		await routeDuplicateDayAnchorEvent(page);

		await openCalendarEmbed(page, '일');
		const timelineEvent = page.locator(duplicateDayAnchorTimelineEventSelector);
		const panelEvent = page.locator(duplicateDayAnchorPanelEventSelector);
		await expect(timelineEvent).toBeVisible();
		await expect(panelEvent).toBeVisible();

		await expectCalendarEventSelectedOnPointerDown(page, duplicateDayAnchorTimelineEventSelector, duplicateDayAnchorPanelEventSelector);
		await expectCalendarEventSelectedBlue(page, duplicateDayAnchorTimelineEventSelector);
		await expectCalendarEventSelectedBlue(page, duplicateDayAnchorPanelEventSelector);
		await clearCalendarSelectionFromTestPage(page);
		await expectCalendarEventSelectedOnPointerDown(page, duplicateDayAnchorPanelEventSelector, duplicateDayAnchorTimelineEventSelector);
		await expectCalendarEventSelectedBlue(page, duplicateDayAnchorTimelineEventSelector);
		await expectCalendarEventSelectedBlue(page, duplicateDayAnchorPanelEventSelector);
		await expectCalendarEventTitleAndTime(page, duplicateDayAnchorTimelineEventSelector, 'Duplicate Day Anchor Event', '02:00');
		await expectCalendarEventTitleAndTime(page, duplicateDayAnchorPanelEventSelector, 'Duplicate Day Anchor Event', '02:00');
		await expect(timelineEvent).not.toContainText('03:00');
		await expect(panelEvent).not.toContainText('03:00');
	});
});

type MovedPointerActivationOptions = {
	pointerType?: string;
};

async function dispatchMovedPointerActivation(page: Page, selector: string, options: MovedPointerActivationOptions = {}): Promise<void> {
	await page.evaluate(async ({ targetSelector, pointerType }) => {
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
		document.dispatchEvent(new PointerEvent('pointermove', movedPointerEvent));
		window.dispatchEvent(new PointerEvent('pointerup', movedPointerEvent));
		target.dispatchEvent(new MouseEvent('click', movedMouseEvent));
		target.dispatchEvent(new MouseEvent('dblclick', movedMouseEvent));
	}, { targetSelector: selector, pointerType: options.pointerType ?? 'mouse' });
}
