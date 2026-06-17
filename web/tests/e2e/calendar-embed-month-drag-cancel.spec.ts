import { expect, test } from '@playwright/test';
import { routeCalendarEventUpdates, routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar month drag cancellation', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('cancels direct month event drag on pointer cancel without saving a move', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'month-cancel-drag-event',
				title: '월간 취소 이동',
				startISO: '2026-06-10T09:00:00+09:00',
				endISO: '2026-06-12T10:00:00+09:00',
				isAllDay: false
			}
		]);
		const updatedEvents = await routeCalendarEventUpdates(page);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-10');
		const dragGeometry = await page.evaluate((eventID) => {
			const eventElement = document.querySelector<HTMLElement>(`.calendar-month-direct-event[data-event-id="${CSS.escape(eventID)}"]`);
			const targetCell = document.querySelector<HTMLElement>('.df-month-day-cell[data-date="2026-06-12"]');
			if (!eventElement || !targetCell) throw new Error('Missing direct month drag cancel target');
			const eventRectangle = eventElement.getBoundingClientRect();
			const targetRectangle = targetCell.getBoundingClientRect();
			return {
				sourceX: eventRectangle.left + Math.min(42, eventRectangle.width / 2),
				sourceY: eventRectangle.top + eventRectangle.height / 2,
				targetX: targetRectangle.left + targetRectangle.width / 2,
				targetY: eventRectangle.top + eventRectangle.height / 2
			};
		}, 'month-cancel-drag-event');

		await page.evaluate(
			({ sourceX, sourceY, targetX, targetY }) => {
				const eventElement = document.querySelector<HTMLElement>(
					'.calendar-month-direct-event[data-event-id="month-cancel-drag-event"]'
				);
				if (!eventElement) throw new Error('Missing direct month drag cancel event');
				const pointerID = 42;
				eventElement.dispatchEvent(
					new PointerEvent('pointerdown', {
						bubbles: true,
						cancelable: true,
						button: 0,
						buttons: 1,
						clientX: sourceX,
						clientY: sourceY,
						pointerId: pointerID
					})
				);
				window.dispatchEvent(
					new PointerEvent('pointermove', {
						bubbles: true,
						cancelable: true,
						buttons: 1,
						clientX: targetX,
						clientY: targetY,
						pointerId: pointerID
					})
				);
				window.dispatchEvent(
					new PointerEvent('pointercancel', {
						bubbles: true,
						cancelable: true,
						buttons: 0,
						clientX: targetX,
						clientY: targetY,
						pointerId: pointerID
					})
				);
			},
			dragGeometry
		);
		await page.waitForTimeout(100);

		expect(updatedEvents).toHaveLength(0);
		await expect
			.poll(async () =>
				page.evaluate(() => {
					const eventElement = document.querySelector<HTMLElement>(
						'.calendar-month-direct-event[data-event-id="month-cancel-drag-event"]'
					);
					const sourceCell = document.querySelector<HTMLElement>('.df-month-day-cell[data-date="2026-06-10"]');
					if (!eventElement || !sourceCell) return false;
					const eventRectangle = eventElement.getBoundingClientRect();
					const sourceRectangle = sourceCell.getBoundingClientRect();
					return Math.abs(eventRectangle.left - sourceRectangle.left) <= 8;
				})
			)
			.toBe(true);
	});
});
