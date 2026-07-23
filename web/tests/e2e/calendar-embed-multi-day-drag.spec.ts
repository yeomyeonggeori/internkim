import { expect, test, type Page } from '@playwright/test';
import {
	dateTimeKeyInSeoul,
	routeCalendarEventUpdates,
	routeCalendarEvents,
	routeDefaultCalendarAPI
} from './calendar-embed-test-utils';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar multi-day month drag interactions', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
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
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await page.keyboard.press('Escape');
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

	test('requires a touch long press before moving a direct month event', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'month-touch-long-press-event',
				title: '월간 롱프레스 이동',
				startISO: '2026-06-10T09:00:00+09:00',
				endISO: '2026-06-10T10:00:00+09:00',
				isAllDay: false
			}
		]);
		const updatedEvents = await routeCalendarEventUpdates(page);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-10');
		const eventBlock = page.locator(
			'.calendar-month-direct-event[data-event-id="month-touch-long-press-event"]'
		);
		const dragGeometry = await monthTouchDragGeometry(page, 'month-touch-long-press-event', '2026-06-12');

		await eventBlock.dispatchEvent('pointerdown', {
			button: 0,
			buttons: 1,
			clientX: dragGeometry.sourceX,
			clientY: dragGeometry.sourceY,
			pointerId: 71,
			pointerType: 'touch'
		});
		await page.waitForTimeout(150);
		await dispatchTouchPointerEnd(page, dragGeometry, 71);
		await page.waitForTimeout(100);

		expect(updatedEvents).toHaveLength(0);
		await expect(eventBlock).not.toHaveClass(/calendar-month-event-drag-ready/);

		await eventBlock.dispatchEvent('pointerdown', {
			button: 0,
			buttons: 1,
			clientX: dragGeometry.sourceX,
			clientY: dragGeometry.sourceY,
			pointerId: 72,
			pointerType: 'touch'
		});
		await page.waitForTimeout(550);
		await expect(eventBlock).toHaveClass(/calendar-month-event-drag-ready/);
		await dispatchTouchPointerEnd(page, dragGeometry, 72);

		await expect.poll(() => updatedEvents.length).toBe(1);
		const updatedEvent = updatedEvents[0];
		if (!updatedEvent) throw new Error('Missing month touch drag update payload');
		expect(updatedEvent.eventID).toBe('month-touch-long-press-event');
		expect(dateTimeKeyInSeoul(updatedEvent.startISO)).toBe('2026-06-12 09:00');
	});
});

async function monthTouchDragGeometry(
	page: Page,
	eventID: string,
	targetDateKey: string
): Promise<{ sourceX: number; sourceY: number; targetX: number; targetY: number }> {
	return page.evaluate(
		({ eventID, targetDateKey }) => {
			const eventElement = document.querySelector<HTMLElement>(
				`.calendar-month-direct-event[data-event-id="${CSS.escape(eventID)}"]`
			);
			const targetCell = document.querySelector<HTMLElement>(
				`.df-month-day-cell[data-date="${CSS.escape(targetDateKey)}"]`
			);
			if (!eventElement || !targetCell) throw new Error('Missing month touch drag geometry');
			const eventRectangle = eventElement.getBoundingClientRect();
			const targetRectangle = targetCell.getBoundingClientRect();
			return {
				sourceX: eventRectangle.left + Math.min(42, eventRectangle.width / 2),
				sourceY: eventRectangle.top + eventRectangle.height / 2,
				targetX: targetRectangle.left + targetRectangle.width / 2,
				targetY: eventRectangle.top + eventRectangle.height / 2
			};
		},
		{ eventID, targetDateKey }
	);
}

async function dispatchTouchPointerEnd(
	page: Page,
	dragGeometry: { targetX: number; targetY: number },
	pointerID: number
): Promise<void> {
	await page.evaluate(
		({ targetX, targetY, pointerID }) => {
			window.dispatchEvent(
				new PointerEvent('pointermove', {
					bubbles: true,
					cancelable: true,
					buttons: 1,
					clientX: targetX,
					clientY: targetY,
					pointerId: pointerID,
					pointerType: 'touch'
				})
			);
			window.dispatchEvent(
				new PointerEvent('pointerup', {
					bubbles: true,
					cancelable: true,
					buttons: 0,
					clientX: targetX,
					clientY: targetY,
					pointerId: pointerID,
					pointerType: 'touch'
				})
			);
		},
		{ targetX: dragGeometry.targetX, targetY: dragGeometry.targetY, pointerID }
	);
}
