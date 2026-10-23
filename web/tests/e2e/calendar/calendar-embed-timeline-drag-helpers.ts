import { expect, type Page } from '@playwright/test';

export async function createTimelineSlotByClick(page: Page, viewLabel: '일' | '주'): Promise<void> {
	await expect(page.locator('header .calendar-toolbar-title')).toBeVisible();
	await page.waitForSelector(timelineTargetSelector(viewLabel), { state: 'attached' });
	await resetTimelineScrollerTop(page, viewLabel);
	await page.evaluate(({ selector, view }) => {
		const { target, clientX, clientY } = visibleTimelineTarget(selector, view);
		if (!(target instanceof HTMLElement)) throw new Error(`Missing timeline target: ${selector}`);
		target.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));

		function visibleTimelineTarget(targetSelector: string, viewLabel: '일' | '주'): { target: Element | null; clientX: number; clientY: number } {
			const scroller = document.querySelector(viewLabel === '일' ? '.df-day-content-grid' : '.df-week-time-grid-scroller');
			if (!(scroller instanceof HTMLElement)) throw new Error('Missing visible timeline scroller');
			const rectangle = scroller.getBoundingClientRect();
			const timeAxisWidth = 124;
			const gridWidth = Math.max(1, rectangle.width - timeAxisWidth);
			const columnWidth = viewLabel === '일' ? gridWidth : gridWidth / 7;
			const columnIndex = viewLabel === '일' ? 0 : 1;
			const clientX = rectangle.left + timeAxisWidth + columnWidth * columnIndex + columnWidth / 2;
			const clientY = rectangle.top + Math.min(160, rectangle.height / 3);
			const targetElement = document.elementFromPoint(clientX, clientY)?.closest(targetSelector) ?? document.querySelector(targetSelector);
			return { target: targetElement, clientX, clientY };
		}
	}, { selector: timelineTargetSelector(viewLabel), view: viewLabel });
}

export async function createTimelineRangeByDrag(page: Page, viewLabel: '일' | '주'): Promise<void> {
	await startTimelineRangeDrag(page, viewLabel);
	await finishTimelineRangeDrag(page, viewLabel);
}

export async function startTimelineRangeDrag(page: Page, viewLabel: '일' | '주'): Promise<void> {
	await expect(page.locator('header .calendar-toolbar-title')).toBeVisible();
	await page.waitForSelector(timelineTargetSelector(viewLabel), { state: 'attached' });
	await resetTimelineScrollerTop(page, viewLabel);
	await page.evaluate(({ selector, columnIndex, view }) => {
		const { target, clientX: startClientX, clientY: startClientY } = visibleTimelineTarget(selector, view, columnIndex);
		if (!(target instanceof HTMLElement)) throw new Error(`Missing timeline target: ${selector}`);
		const endClientY = startClientY + 140;
		target.dispatchEvent(
			new PointerEvent('pointerdown', {
				bubbles: true,
				cancelable: true,
				button: 0,
				pointerId: 1,
				clientX: startClientX,
				clientY: startClientY
			})
		);
		target.dispatchEvent(
			new MouseEvent('mousedown', {
				bubbles: true,
				cancelable: true,
				button: 0,
				clientX: startClientX,
				clientY: startClientY
			})
		);
		document.dispatchEvent(
			new PointerEvent('pointermove', {
				bubbles: true,
				cancelable: true,
				button: 0,
				pointerId: 1,
				clientX: startClientX,
				clientY: endClientY
			})
		);

		function visibleTimelineTarget(
			targetSelector: string,
			viewLabel: '일' | '주',
			targetColumnIndex: number
		): { target: Element | null; clientX: number; clientY: number } {
			const scroller = document.querySelector(viewLabel === '일' ? '.df-day-content-grid' : '.df-week-time-grid-scroller');
			if (!(scroller instanceof HTMLElement)) throw new Error('Missing visible timeline scroller');
			const rectangle = scroller.getBoundingClientRect();
			const timeAxisWidth = 124;
			const gridWidth = Math.max(1, rectangle.width - timeAxisWidth);
			const columnWidth = viewLabel === '일' ? gridWidth : gridWidth / 7;
			const clientX = rectangle.left + timeAxisWidth + columnWidth * targetColumnIndex + columnWidth / 2;
			const clientY = rectangle.top + Math.min(160, rectangle.height / 3);
			const targetElement = document.elementFromPoint(clientX, clientY)?.closest(targetSelector) ?? document.querySelector(targetSelector);
			return { target: targetElement, clientX, clientY };
		}
	}, { selector: timelineTargetSelector(viewLabel), columnIndex: timelineTargetColumnIndex(viewLabel), view: viewLabel });
}

export async function startDayTimelineRangeDragAtHour(page: Page, startHour: number, durationHours: number): Promise<void> {
	await expect(page.locator('header .calendar-toolbar-title')).toBeVisible();
	await page.waitForSelector('.df-day-content-grid-column', { state: 'attached' });
	await resetTimelineScrollerTop(page, '일');
	await page.evaluate(
		({ targetStartHour, targetDurationHours }) => {
			const grid = document.querySelector('.df-day-content-grid-column');
			const rows = document.querySelector('.df-day-content-grid-rows');
			if (!(grid instanceof HTMLElement) || !(rows instanceof HTMLElement)) throw new Error('Missing day timeline grid');
			const gridRectangle = grid.getBoundingClientRect();
			const rowsRectangle = rows.getBoundingClientRect();
			const hourHeight = rowsRectangle.height / 24;
			const startClientX = gridRectangle.left + gridRectangle.width / 2;
			const startClientY = rowsRectangle.top + hourHeight * targetStartHour + 4;
			const endClientY = startClientY + hourHeight * targetDurationHours;
			const target = document.elementFromPoint(startClientX, startClientY)?.closest('.df-day-content-grid-column') ?? grid;
			if (!(target instanceof HTMLElement)) throw new Error('Missing day timeline target');
			target.dispatchEvent(
				new PointerEvent('pointerdown', {
					bubbles: true,
					cancelable: true,
					button: 0,
					pointerId: 3,
					clientX: startClientX,
					clientY: startClientY
				})
			);
			target.dispatchEvent(
				new MouseEvent('mousedown', {
					bubbles: true,
					cancelable: true,
					button: 0,
					clientX: startClientX,
					clientY: startClientY
				})
			);
			document.dispatchEvent(
				new PointerEvent('pointermove', {
					bubbles: true,
					cancelable: true,
					button: 0,
					pointerId: 3,
					clientX: startClientX,
					clientY: endClientY
				})
			);
		},
		{ targetStartHour: startHour, targetDurationHours: durationHours }
	);
}

export async function createDayTimelineRangeByDragAtHour(page: Page, startHour: number, durationHours: number): Promise<void> {
	await startDayTimelineRangeDragAtHour(page, startHour, durationHours);
	await page.evaluate(
		({ targetStartHour, targetDurationHours }) => {
			const grid = document.querySelector('.df-day-content-grid-column');
			const rows = document.querySelector('.df-day-content-grid-rows');
			if (!(grid instanceof HTMLElement) || !(rows instanceof HTMLElement)) throw new Error('Missing day timeline grid');
			const gridRectangle = grid.getBoundingClientRect();
			const rowsRectangle = rows.getBoundingClientRect();
			const hourHeight = rowsRectangle.height / 24;
			const clientX = gridRectangle.left + gridRectangle.width / 2;
			const clientY = rowsRectangle.top + hourHeight * (targetStartHour + targetDurationHours) + 4;
			window.dispatchEvent(
				new PointerEvent('pointerup', {
					bubbles: true,
					cancelable: true,
					button: 0,
					pointerId: 3,
					clientX,
					clientY
				})
			);
		},
		{ targetStartHour: startHour, targetDurationHours: durationHours }
	);
}

export async function finishTimelineRangeDrag(page: Page, viewLabel: '일' | '주'): Promise<void> {
	await page.evaluate(({ selector, columnIndex, view }) => {
		const { target, clientX: startClientX, clientY: startClientY } = visibleTimelineTarget(selector, view, columnIndex);
		if (!(target instanceof HTMLElement)) throw new Error('Missing timeline target');
		const endClientY = startClientY + 220;
		window.dispatchEvent(
			new PointerEvent('pointerup', {
				bubbles: true,
				cancelable: true,
				button: 0,
				pointerId: 1,
				clientX: startClientX,
				clientY: endClientY
			})
		);

		function visibleTimelineTarget(
			targetSelector: string,
			viewLabel: '일' | '주',
			targetColumnIndex: number
		): { target: Element | null; clientX: number; clientY: number } {
			const scroller = document.querySelector(viewLabel === '일' ? '.df-day-content-grid' : '.df-week-time-grid-scroller');
			if (!(scroller instanceof HTMLElement)) throw new Error('Missing visible timeline scroller');
			const rectangle = scroller.getBoundingClientRect();
			const timeAxisWidth = 124;
			const gridWidth = Math.max(1, rectangle.width - timeAxisWidth);
			const columnWidth = viewLabel === '일' ? gridWidth : gridWidth / 7;
			const clientX = rectangle.left + timeAxisWidth + columnWidth * targetColumnIndex + columnWidth / 2;
			const clientY = rectangle.top + Math.min(160, rectangle.height / 3);
			const targetElement = document.elementFromPoint(clientX, clientY)?.closest(targetSelector) ?? document.querySelector(targetSelector);
			return { target: targetElement, clientX, clientY };
		}
	}, { selector: timelineTargetSelector(viewLabel), columnIndex: timelineTargetColumnIndex(viewLabel), view: viewLabel });
}

export function timelineTargetSelector(viewLabel: '일' | '주'): string {
	if (viewLabel === '일') return '.df-day-content-grid-column';
	return '.df-week-time-grid-cell';
}

export function timelineTargetColumnIndex(viewLabel: '일' | '주'): number {
	if (viewLabel === '일') return 0;
	return 1;
}

async function resetTimelineScrollerTop(page: Page, viewLabel: '일' | '주'): Promise<void> {
	await page.evaluate((view) => {
		const scrollerSelector = view === '일' ? '.df-day-content-grid' : '.df-week-time-grid-scroller';
		const scroller = document.querySelector(scrollerSelector);
		if (!(scroller instanceof HTMLElement)) throw new Error(`Missing timeline scroller: ${scrollerSelector}`);
		scroller.scrollTop = 0;
		scroller.dispatchEvent(new Event('scroll', { bubbles: true }));
	}, viewLabel);
	await waitForTimelineScrollSettle(page);
}

async function waitForTimelineScrollSettle(page: Page): Promise<void> {
	await page.evaluate(
		() =>
			new Promise<void>((resolve) => {
				requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
			})
	);
}
