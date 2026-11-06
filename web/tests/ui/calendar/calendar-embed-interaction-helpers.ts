import { type Page } from '@playwright/test';

export { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-navigation-helpers';
export {
	createDayTimelineRangeByDragAtHour,
	createTimelineRangeByDrag,
	createTimelineSlotByClick,
	finishTimelineRangeDrag,
	startDayTimelineRangeDragAtHour,
	startTimelineRangeDrag,
	timelineTargetColumnIndex,
	timelineTargetSelector
} from './calendar-embed-timeline-drag-helpers';

export async function dismissDraftPopoverFromTimeline(page: Page): Promise<void> {
	await page.evaluate(() => {
		const target = document.querySelector('.df-time-grid-row .df-week-time-grid-cell:nth-child(4), .df-day-content-grid-column');
		if (!(target instanceof HTMLElement)) throw new Error('Missing timeline dismiss target');
		const rectangle = target.getBoundingClientRect();
		const clientX = rectangle.left + rectangle.width / 2;
		const clientY = rectangle.top + 260;
		target.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, cancelable: true, button: 0, pointerId: 7, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
	});
}

export async function dispatchElementScroll(page: Page, selector: string, scrollTop: number): Promise<void> {
	await page.evaluate(
		({ targetSelector, targetScrollTop }) => {
			const target = document.querySelector<HTMLElement>(targetSelector);
			if (!target) throw new Error(`Missing scroll target: ${targetSelector}`);
			target.scrollTop = targetScrollTop;
			target.dispatchEvent(new Event('scroll', { bubbles: true }));
		},
		{ targetSelector: selector, targetScrollTop: scrollTop }
	);
}

export async function clickCalendarEvent(page: Page, selector: string): Promise<void> {
	await page.evaluate(async (targetSelector) => {
		const eventElement = document.querySelector<HTMLElement>(targetSelector);
		if (!eventElement) throw new Error(`Missing calendar event: ${targetSelector}`);
		eventElement.scrollIntoView({ block: 'center', inline: 'nearest' });
		await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
		const target =
			eventElement.querySelector<HTMLElement>('.calendar-dayflow-event-activator') ?? eventElement;
		const rectangle = target.getBoundingClientRect();
		const clientX = rectangle.left + rectangle.width / 2;
		const clientY = rectangle.top + rectangle.height / 2;
		target.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, cancelable: true, button: 0, pointerId: 17, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, cancelable: true, button: 0, pointerId: 17, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
	}, selector);
}
