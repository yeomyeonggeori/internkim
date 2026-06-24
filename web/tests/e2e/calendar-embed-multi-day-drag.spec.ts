import { expect, test } from '@playwright/test';
import {
	dateTimeKeyInSeoul,
	routeCalendarEventUpdates,
	routeCalendarEvents,
	routeDefaultCalendarAPI
} from './calendar-embed-test-utils';
import { expectMultiDayTimedProxy, expectWeekAllDayEventsDoNotOverlap } from './calendar-embed-interaction-assertions';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar multi-day and drag interactions', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('shows multi-day timed events as editable day and week all-day proxies', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'week-multi-day-timed-event',
				title: '주간 멀티',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			},
			{
				id: 'week-multi-day-timed-event-2',
				title: '주간 멀티 2',
				startISO: '2026-06-16T13:00:00+09:00',
				endISO: '2026-06-18T14:15:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		await expectMultiDayTimedProxy(page, 'week-multi-day-timed-event', '주간 멀티', '11:45', '12:30');
		await expectMultiDayTimedProxy(page, 'week-multi-day-timed-event-2', '주간 멀티 2', '13:00', '14:15');
		await expect(page.locator('.df-week-event.df-event-timed[data-event-id="week-multi-day-timed-event"]:visible')).toHaveCount(0);
		await expect(page.locator('.df-week-event.df-event-timed[data-event-id="week-multi-day-timed-event-2"]:visible')).toHaveCount(0);

		await page.locator('.calendar-multi-day-all-day-proxy', { hasText: '주간 멀티' }).first().dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.locator('.df-event-detail-panel')).toHaveCount(0);

		await openCalendarEmbed(page, '일');
		await navigateEmbeddedCalendar(page, '2026-06-17');
		await expect(page.locator('.calendar-multi-day-all-day-proxy[data-event-id="week-multi-day-timed-event::multi-day-proxy"]')).toHaveCount(1);
		await expect(page.locator('.calendar-multi-day-all-day-proxy[data-event-id="week-multi-day-timed-event-2::multi-day-proxy"]')).toHaveCount(1);
		await expect(page.locator('.df-day-event.df-event-timed[data-event-id="week-multi-day-timed-event"]:visible')).toHaveCount(0);
		await expect(page.locator('.df-day-event.df-event-timed[data-event-id="week-multi-day-timed-event-2"]:visible')).toHaveCount(0);
	});

	test('keeps multi-day timed proxies from overlapping week all-day events', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'week-all-day-overlap-event',
				title: '주간 종일',
				startISO: '2026-06-16T00:00:00+09:00',
				endISO: '2026-06-19T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'week-multi-day-overlap-event',
				title: '주간 시간 다일',
				startISO: '2026-06-16T11:00:00+09:00',
				endISO: '2026-06-18T12:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');

		await expectMultiDayTimedProxy(page, 'week-multi-day-overlap-event', '주간 시간 다일', '11:00', '12:00');
		await expectWeekAllDayEventsDoNotOverlap(page, 'week-all-day-overlap-event', 'week-multi-day-overlap-event::multi-day-proxy');
	});

	test('keeps multi-day month drag feedback within the event date span', async ({ page }) => {
		await routeCalendarEventUpdates(page);
		await routeCalendarEvents(page, [
			{
				id: 'month-multi-day-drag-event',
				title: '월간 다일 드래그',
				startISO: '2026-06-10T09:00:00+09:00',
				endISO: '2026-06-12T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-10');
		const dragGeometry = await page.evaluate((eventID) => {
			const eventElement = visibleMonthEvent(eventID);
			if (!eventElement) throw new Error(`Missing visible month event: ${eventID}`);
			const eventRectangle = eventElement.getBoundingClientRect();
			const startCell = document.querySelector<HTMLElement>('.df-month-day-cell[data-date="2026-06-10"]');
			const endCell = document.querySelector<HTMLElement>('.df-month-day-cell[data-date="2026-06-12"]');
			const targetCell = document.querySelector<HTMLElement>('.df-month-day-cell[data-date="2026-06-11"]');
			if (!startCell || !endCell || !targetCell) throw new Error('Missing month drag date cells');
			const startCellRectangle = startCell.getBoundingClientRect();
			const endCellRectangle = endCell.getBoundingClientRect();
			const targetCellRectangle = targetCell.getBoundingClientRect();
			return {
				sourceX: eventRectangle.left + Math.min(42, eventRectangle.width / 2),
				sourceY: eventRectangle.top + eventRectangle.height / 2,
				targetX: targetCellRectangle.left + targetCellRectangle.width / 2,
				targetY: eventRectangle.top + eventRectangle.height / 2,
				maxExpectedWidth: Math.round(endCellRectangle.right - startCellRectangle.left)
			};

			function visibleMonthEvent(targetEventID: string): HTMLElement | null {
				return (
					Array.from(document.querySelectorAll<HTMLElement>(`[data-event-id="${CSS.escape(targetEventID)}"].df-month-event`)).find(
						(element) => {
							const rectangle = element.getBoundingClientRect();
							return rectangle.width > 0 && rectangle.height > 0;
						}
					) ?? null
				);
			}
		}, 'month-multi-day-drag-event');

		await page.mouse.move(dragGeometry.sourceX, dragGeometry.sourceY);
		await page.mouse.down();
		await page.mouse.move(dragGeometry.targetX, dragGeometry.targetY, { steps: 12 });

		await expect
			.poll(async () =>
				page.evaluate((title) => {
					const elements = Array.from(
						document.querySelectorAll<HTMLElement>('.calendar-stage .df-event, .calendar-stage .month-range-preview')
					).filter((element) => {
						const rectangle = element.getBoundingClientRect();
						return rectangle.width > 0 && rectangle.height > 0 && (element.textContent ?? '').includes(title);
					});
					return Math.max(0, ...elements.map((element) => Math.round(element.getBoundingClientRect().width)));
				}, '월간 다일 드래그')
			)
			.toBeLessThanOrEqual(dragGeometry.maxExpectedWidth + 6);

		await page.mouse.up();
	});

	test('moves a timed multi-day month event with direct layer drag', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'month-direct-selected-event',
				title: '월간 기존 선택',
				startISO: '2026-06-16T09:00:00+09:00',
				endISO: '2026-06-16T10:00:00+09:00',
				isAllDay: false
			},
			{
				id: 'month-direct-drag-event',
				title: '월간 직접 이동',
				startISO: '2026-06-10T09:00:00+09:00',
				endISO: '2026-06-12T10:00:00+09:00',
				isAllDay: false
			}
		]);
		const updatedEvents = await routeCalendarEventUpdates(page);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-10');
		await page.locator('.calendar-month-direct-event[data-event-id="month-direct-selected-event"]').click();
		await expect(page.locator('.calendar-month-direct-event[data-event-id="month-direct-selected-event"]')).toHaveClass(
			/internkim-calendar-event-focused/
		);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('.calendar-month-direct-event[data-event-id="month-direct-selected-event"]')).toHaveClass(
			/internkim-calendar-event-focused/
		);
		const dragGeometry = await page.evaluate((eventID) => {
			const eventElement = document.querySelector<HTMLElement>(`.calendar-month-direct-event[data-event-id="${CSS.escape(eventID)}"]`);
			const targetCell = document.querySelector<HTMLElement>('.df-month-day-cell[data-date="2026-06-12"]');
			if (!eventElement || !targetCell) throw new Error('Missing direct month drag target');
			const eventRectangle = eventElement.getBoundingClientRect();
			const targetRectangle = targetCell.getBoundingClientRect();
			return {
				sourceX: eventRectangle.left + Math.min(42, eventRectangle.width / 2),
				sourceY: eventRectangle.top + eventRectangle.height / 2,
				targetX: targetRectangle.left + targetRectangle.width / 2,
				targetY: eventRectangle.top + eventRectangle.height / 2
			};
		}, 'month-direct-drag-event');

		await page.mouse.move(dragGeometry.sourceX, dragGeometry.sourceY);
		await page.mouse.down();
		await page.mouse.move(dragGeometry.sourceX + 12, dragGeometry.sourceY, { steps: 4 });
		await expect(page.locator('.calendar-month-direct-event[data-event-id="month-direct-drag-event"]')).toHaveClass(
			/internkim-calendar-event-focused/
		);
		await expect(page.locator('.calendar-month-direct-event[data-event-id="month-direct-selected-event"]')).not.toHaveClass(
			/internkim-calendar-event-focused/
		);
		await page.mouse.move(dragGeometry.targetX, dragGeometry.targetY, { steps: 12 });
		await page.mouse.up();

		await expect.poll(() => updatedEvents.length).toBe(1);
		await expect(
			page.locator('.calendar-month-direct-event[data-event-id="month-direct-drag-event"].internkim-calendar-event-focused')
		).toHaveCount(0);
		const updatedEvent = updatedEvents[0];
		if (!updatedEvent) throw new Error('Missing month direct drag update payload');
		expect(updatedEvent.eventID).toBe('month-direct-drag-event');
		expect(dateTimeKeyInSeoul(updatedEvent.startISO)).toBe('2026-06-12 09:00');
		expect(dateTimeKeyInSeoul(updatedEvent.endISO)).toBe('2026-06-14 10:00');
		await expect
			.poll(async () =>
				page.evaluate((eventID) => {
					const eventElement = document.querySelector<HTMLElement>(`.calendar-month-direct-event[data-event-id="${CSS.escape(eventID)}"]`);
					const targetCell = document.querySelector<HTMLElement>('.df-month-day-cell[data-date="2026-06-12"]');
					if (!eventElement || !targetCell) return false;
					const eventRectangle = eventElement.getBoundingClientRect();
					const targetRectangle = targetCell.getBoundingClientRect();
					return Math.abs(eventRectangle.left - targetRectangle.left) <= 8;
				}, 'month-direct-drag-event')
			)
			.toBe(true);
	});

});
