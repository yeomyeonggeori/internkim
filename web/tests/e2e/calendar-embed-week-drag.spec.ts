import { expect, test } from '@playwright/test';
import {
	dateTimeKeyInSeoul,
	routeCalendarEventUpdates,
	routeCalendarEvents,
	routeDefaultCalendarAPI
} from './calendar-embed-test-utils';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar week drag interactions', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('moves a timed week event with drag without showing native temporary blocks', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'drag-move-event',
				title: '드래그 이동 일정',
				startISO: '2026-06-09T02:00:00+09:00',
				endISO: '2026-06-09T03:00:00+09:00',
				isAllDay: false
			}
		]);
		const updatedEvents = await routeCalendarEventUpdates(page);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-09');
		await expect(page.locator('.df-week-event.df-event-timed[data-event-id="drag-move-event"]')).toBeVisible();

		const dragTarget = await page.evaluate((eventID) => {
			const escapedEventID = CSS.escape(eventID);
			const eventElement = Array.from(
				document.querySelectorAll<HTMLElement>(`.df-week-event.df-event-timed[data-event-id="${escapedEventID}"]`)
			).find((element) => element.offsetParent !== null);
			if (!eventElement) throw new Error(`Missing visible draggable event: ${eventID}`);
			const eventRectangle = eventElement.getBoundingClientRect();
			const sourceX = eventRectangle.left + eventRectangle.width / 2;
			const sourceY = eventRectangle.top + Math.min(12, eventRectangle.height / 2);
			const firstRow = document.querySelector('.df-time-grid-row');
			if (!(firstRow instanceof HTMLElement)) throw new Error('Missing week time grid row');
			const cells = Array.from(firstRow.querySelectorAll<HTMLElement>('.df-week-time-grid-cell'));
			const sourceColumnIndex = cells.findIndex((cell) => {
				const rectangle = cell.getBoundingClientRect();
				return sourceX >= rectangle.left && sourceX <= rectangle.right;
			});
			if (sourceColumnIndex < 0) throw new Error('Missing source column for draggable event');
			const targetCell = cells[sourceColumnIndex + 1];
			if (!targetCell) throw new Error('Missing next-day drop target cell');
			const targetRectangle = targetCell.getBoundingClientRect();
			return {
				sourceX,
				sourceY,
				targetX: targetRectangle.left + targetRectangle.width / 2,
				targetY: sourceY,
				targetColumnIndex: sourceColumnIndex + 1
			};
		}, 'drag-move-event');

		await page.mouse.move(dragTarget.sourceX, dragTarget.sourceY);
		await page.mouse.down();
		await page.mouse.move(dragTarget.targetX, dragTarget.targetY, { steps: 16 });
		await expect(
			page.locator(
				'.calendar-stage .df-drag-indicator-month-pill:visible, .calendar-stage .df-drag-indicator-regular-pill:visible, .calendar-stage .df-drag-indicator-all-day-pill:visible, .calendar-stage .df-drag-indicator-manual-pill:visible'
			)
		).toHaveCount(0);
		await page.mouse.up();

		await expect.poll(() => updatedEvents.length).toBe(1);
		const updatedEvent = updatedEvents[0];
		if (!updatedEvent) throw new Error('Missing drag move update payload');
		expect(updatedEvent.eventID).toBe('drag-move-event');
		expect(dateTimeKeyInSeoul(updatedEvent.startISO)).toBe('2026-06-10 02:00');
		expect(dateTimeKeyInSeoul(updatedEvent.endISO)).toBe('2026-06-10 03:00');
		await expect(page.locator('.df-week-event.df-event-timed[data-event-id="drag-move-event"]')).toHaveCount(1);
		await expect(page.locator('.df-week-event.df-event-timed[data-event-id="drag-move-event"]')).toContainText('드래그 이동 일정');

		const movedPosition = await page.evaluate(
			({ eventID, targetColumnIndex }) => {
				const escapedEventID = CSS.escape(eventID);
				const eventElement = Array.from(
					document.querySelectorAll<HTMLElement>(`.df-week-event.df-event-timed[data-event-id="${escapedEventID}"]`)
				).find((element) => element.offsetParent !== null);
				if (!eventElement) throw new Error(`Missing moved event: ${eventID}`);
				const firstRow = document.querySelector('.df-time-grid-row');
				if (!(firstRow instanceof HTMLElement)) throw new Error('Missing week time grid row');
				const targetCell = firstRow.querySelectorAll<HTMLElement>('.df-week-time-grid-cell')[targetColumnIndex];
				if (!targetCell) throw new Error('Missing target column after drag');
				const eventRectangle = eventElement.getBoundingClientRect();
				const targetRectangle = targetCell.getBoundingClientRect();
				return {
					eventCenterX: eventRectangle.left + eventRectangle.width / 2,
					targetLeft: targetRectangle.left,
					targetRight: targetRectangle.right
				};
			},
			{ eventID: 'drag-move-event', targetColumnIndex: dragTarget.targetColumnIndex }
		);
		expect(movedPosition.eventCenterX).toBeGreaterThan(movedPosition.targetLeft);
		expect(movedPosition.eventCenterX).toBeLessThan(movedPosition.targetRight);
	});

	test('hides DayFlow native drag indicators inside the calendar stage', async ({ page }) => {
		await openCalendarEmbed(page, '월');
		await page.locator('.calendar-stage').evaluate((stageElement) => {
			const indicator = document.createElement('div');
			indicator.className = 'df-drag-indicator-regular-pill df-event';
			indicator.textContent = 'Native drag indicator';
			stageElement.appendChild(indicator);
		});

		await expect(page.locator('.df-drag-indicator-regular-pill')).toHaveCSS('display', 'none');
	});

	test('scopes hidden DayFlow panels to the calendar stage', async ({ page }) => {
		await openCalendarEmbed(page, '월');
		await page.evaluate(() => {
			const outsidePanel = document.createElement('div');
			outsidePanel.id = 'outside-dayflow-panel';
			outsidePanel.className = 'df-event-detail-panel';
			outsidePanel.style.display = 'block';
			document.body.appendChild(outsidePanel);
		});
		await page.locator('.calendar-stage').evaluate((stageElement) => {
			const insidePanel = document.createElement('div');
			insidePanel.id = 'inside-dayflow-panel';
			insidePanel.className = 'df-event-detail-panel';
			insidePanel.style.display = 'block';
			stageElement.appendChild(insidePanel);
		});

		await expect(page.locator('#inside-dayflow-panel')).toHaveCSS('display', 'none');
		await expect(page.locator('#outside-dayflow-panel')).toHaveCSS('display', 'block');
	});
});
