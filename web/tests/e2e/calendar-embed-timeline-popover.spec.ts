import { expect, test } from '@playwright/test';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import {
	expectPopoverArrowPointsToElement,
	expectPopoverArrowPointsToEventTimeEnd,
	expectPopoverOpensLeftOfElement
} from './calendar-embed-draft-assertions';
import { doubleClickCalendarEvent, navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';
import {
	duplicateDayAnchorPanelEventSelector,
	duplicateDayAnchorTimelineEventSelector,
	routeDuplicateDayAnchorEvent,
	routeRightPanelMixedEvents
} from './calendar-embed-timeline-fixtures';

test.describe('embedded calendar timeline popover anchors', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
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

	test('switches existing day event editors at the compact viewport boundary', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'compact-boundary-event',
				title: 'Compact Boundary Event',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:00:00+09:00',
				isAllDay: false
			}
		]);
		await page.setViewportSize({ width: 767, height: 900 });
		await openCalendarEmbed(page, '일');

		const eventBlock = page.locator(
			'.calendar-stage [data-event-id="compact-boundary-event"].df-day-event:not(.df-right-panel-event-card)'
		);
		await expect(eventBlock).toBeVisible();
		expect(await page.evaluate(() => window.innerWidth)).toBe(767);
		await eventBlock.click();
		await expect(eventBlock).toHaveClass(/internkim-calendar-event-focused/);
		await expect(page.locator('.calendar-mobile-event-editor')).toHaveCount(0);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await eventBlock.dblclick();
		await expect(page.locator('.calendar-mobile-event-editor')).toBeVisible();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await page
			.locator('.calendar-mobile-event-editor .df-mobile-event-drawer-header-action')
			.filter({ hasText: '취소' })
			.click();
		await expect(page.locator('.calendar-mobile-event-editor')).toHaveCount(0);

		await page.setViewportSize({ width: 768, height: 900 });
		await page.reload();
		await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-day/);
		expect(await page.evaluate(() => window.innerWidth)).toBe(768);
		await expect(eventBlock).toBeVisible();
		await eventBlock.dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.locator('.calendar-mobile-event-editor')).toHaveCount(0);
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

	test('anchors week popovers to timed, all-day, and multi-day proxy blocks', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'week-anchor-timed',
				title: 'Week Anchor Timed',
				startISO: '2026-06-16T09:00:00+09:00',
				endISO: '2026-06-16T10:00:00+09:00',
				isAllDay: false
			},
			{
				id: 'week-anchor-all-day',
				title: 'Week Anchor All Day',
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-18T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'week-anchor-proxy',
				title: 'Week Anchor Proxy',
				startISO: '2026-06-18T11:45:00+09:00',
				endISO: '2026-06-20T12:30:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		const anchorSelectors = [
			'.calendar-stage [data-event-id="week-anchor-timed"].df-week-event.df-event-timed',
			'.calendar-stage .df-week-all-day-event-layer [data-event-id="week-anchor-all-day"]',
			'.calendar-stage .calendar-multi-day-all-day-proxy[data-event-id="week-anchor-proxy::multi-day-proxy"]'
		];

		for (const anchorSelector of anchorSelectors) {
			await expect(page.locator(anchorSelector)).toBeVisible();
			await doubleClickCalendarEvent(page, anchorSelector);
			await expect(page.locator('.calendar-draft-popover')).toBeVisible();
			await expectPopoverArrowPointsToElement(page, anchorSelector);
			await page.keyboard.press('Escape');
		}
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
});
