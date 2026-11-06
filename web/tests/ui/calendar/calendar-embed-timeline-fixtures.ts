import type { Page } from '@playwright/test';
import { routeCalendarEvents } from './calendar-embed-test-utils';

export const duplicateDayAnchorTimelineEventSelector =
	'.calendar-stage [data-event-id="duplicate-day-anchor-event"].df-day-event:not(.df-right-panel-event-card)';
export const duplicateDayAnchorPanelEventSelector =
	'.calendar-stage [data-event-id="duplicate-day-anchor-event"].df-right-panel-event-card';

export async function routeDuplicateDayAnchorEvent(page: Page): Promise<void> {
	await routeCalendarEvents(page, [
		{
			id: 'duplicate-day-anchor-event',
			title: 'Duplicate Day Anchor Event',
			startISO: '2026-06-08T02:00:00+09:00',
			endISO: '2026-06-08T03:00:00+09:00',
			isAllDay: false
		}
	]);
}

export async function routeRightPanelMixedEvents(page: Page): Promise<void> {
	await routeCalendarEvents(page, [
		{
			id: 'right-panel-all-day-a',
			title: 'test 4',
			startISO: '2026-06-08T00:00:00+09:00',
			endISO: '2026-06-09T00:00:00+09:00',
			isAllDay: true
		},
		{
			id: 'right-panel-all-day-b',
			title: 'test 3',
			startISO: '2026-06-08T00:00:00+09:00',
			endISO: '2026-06-09T00:00:00+09:00',
			isAllDay: true
		},
		{
			id: 'right-panel-all-day-c',
			title: 'test 5',
			startISO: '2026-06-08T00:00:00+09:00',
			endISO: '2026-06-09T00:00:00+09:00',
			isAllDay: true
		},
		{
			id: 'right-panel-timed-a',
			title: 'test 1',
			startISO: '2026-06-08T03:00:00+09:00',
			endISO: '2026-06-08T07:00:00+09:00',
			isAllDay: false
		},
		{
			id: 'right-panel-timed-b',
			title: 'test 2',
			startISO: '2026-06-08T04:30:00+09:00',
			endISO: '2026-06-08T07:00:00+09:00',
			isAllDay: false
		}
	]);
}

export async function clearCalendarSelectionFromTestPage(page: Page): Promise<void> {
	await page.evaluate(() => {
		document.body.dispatchEvent(
			new PointerEvent('pointerdown', {
				bubbles: true,
				cancelable: true,
				button: 0,
				pointerId: 77,
				clientX: 4,
				clientY: 4
			})
		);
	});
}
