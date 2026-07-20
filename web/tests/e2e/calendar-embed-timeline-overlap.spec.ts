import { expect, test } from '@playwright/test';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import {
	expectTimelineEventLayeredBehindLanes,
	expectTimelineEventNestedInsideLane,
	expectTimelineEventsUseSeparateLanes
} from './calendar-embed-interaction-assertions';
import { expectPopoverArrowPointsToEventTimeEnd } from './calendar-embed-draft-assertions';
import { clickCalendarEvent, openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar timeline overlap layout', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('splits overlapping day events into selectable timeline lanes', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'overlap-lane-event-a',
				title: 'Lane A',
				startISO: '2026-06-08T06:30:00+09:00',
				endISO: '2026-06-08T12:15:00+09:00',
				isAllDay: false
			},
			{
				id: 'overlap-lane-event-b',
				title: 'Lane B',
				startISO: '2026-06-08T06:45:00+09:00',
				endISO: '2026-06-08T07:45:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '일');
		const firstEventSelector = '.calendar-stage [data-event-id="overlap-lane-event-a"].df-day-event:not(.df-right-panel-event-card)';
		const secondEventSelector = '.calendar-stage [data-event-id="overlap-lane-event-b"].df-day-event:not(.df-right-panel-event-card)';
		await expect(page.locator(firstEventSelector)).toBeVisible();
		await expect(page.locator(secondEventSelector)).toBeVisible();

		await expectTimelineEventsUseSeparateLanes(page, firstEventSelector, secondEventSelector);
		await clickCalendarEvent(page, secondEventSelector);

		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expectPopoverArrowPointsToEventTimeEnd(page, secondEventSelector);
	});

	test('layers partially overlapping day events when labels do not collide', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'partial-layer-background-event',
				title: 'Layer Background',
				startISO: '2026-06-08T04:45:00+09:00',
				endISO: '2026-06-08T08:45:00+09:00',
				isAllDay: false
			},
			{
				id: 'partial-layer-lane-event-a',
				title: 'Layer Lane A',
				startISO: '2026-06-08T06:15:00+09:00',
				endISO: '2026-06-08T23:15:00+09:00',
				isAllDay: false
			},
			{
				id: 'partial-layer-lane-event-b',
				title: 'Layer Lane B',
				startISO: '2026-06-08T06:15:00+09:00',
				endISO: '2026-06-08T10:15:00+09:00',
				isAllDay: false
			},
			{
				id: 'partial-layer-lane-event-c',
				title: 'Layer Lane C',
				startISO: '2026-06-08T06:00:00+09:00',
				endISO: '2026-06-08T07:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '일');
		const backgroundSelector = '.calendar-stage [data-event-id="partial-layer-background-event"].df-day-event:not(.df-right-panel-event-card)';
		const laneSelectors = ['partial-layer-lane-event-a', 'partial-layer-lane-event-b', 'partial-layer-lane-event-c'].map(
			(eventID) => `.calendar-stage [data-event-id="${eventID}"].df-day-event:not(.df-right-panel-event-card)`
		);
		await expect(page.locator(backgroundSelector)).toBeVisible();
		for (const selector of laneSelectors) {
			await expect(page.locator(selector)).toBeVisible();
		}

		await expectTimelineEventLayeredBehindLanes(page, backgroundSelector, laneSelectors);
	});

	test('nests contained day events inside a parent lane when labels do not collide', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'contained-parent-event-a',
				title: 'Contained Parent A',
				startISO: '2026-06-08T06:15:00+09:00',
				endISO: '2026-06-08T23:15:00+09:00',
				isAllDay: false
			},
			{
				id: 'contained-parent-event-b',
				title: 'Contained Parent B',
				startISO: '2026-06-08T06:15:00+09:00',
				endISO: '2026-06-08T10:15:00+09:00',
				isAllDay: false
			},
			{
				id: 'contained-child-event',
				title: 'Contained Child',
				startISO: '2026-06-08T07:15:00+09:00',
				endISO: '2026-06-08T08:15:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '일');
		const parentSelector = '.calendar-stage [data-event-id="contained-parent-event-a"].df-day-event:not(.df-right-panel-event-card)';
		const siblingSelector = '.calendar-stage [data-event-id="contained-parent-event-b"].df-day-event:not(.df-right-panel-event-card)';
		const childSelector = '.calendar-stage [data-event-id="contained-child-event"].df-day-event:not(.df-right-panel-event-card)';
		await expect(page.locator(parentSelector)).toBeVisible();
		await expect(page.locator(siblingSelector)).toBeVisible();
		await expect(page.locator(childSelector)).toBeVisible();

		await expectTimelineEventsUseSeparateLanes(page, parentSelector, siblingSelector);
		await expectTimelineEventNestedInsideLane(page, childSelector, parentSelector);
	});
});
