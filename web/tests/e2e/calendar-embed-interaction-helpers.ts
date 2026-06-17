import { expect, type Page } from '@playwright/test';

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

export async function createTimelineSlotByDoubleClick(page: Page, viewLabel: '일' | '주'): Promise<void> {
	await expect(page.locator('header').getByRole('button', { name: '오늘' })).toBeVisible();
	await page.waitForSelector(timelineTargetSelector(viewLabel), { state: 'attached' });
	await page.evaluate(({ selector, view }) => {
		const { target, clientX, clientY } = visibleTimelineTarget(selector, view);
		if (!(target instanceof HTMLElement)) throw new Error(`Missing timeline target: ${selector}`);
		target.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('dblclick', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));

		function visibleTimelineTarget(targetSelector: string, viewLabel: '일' | '주'): { target: Element | null; clientX: number; clientY: number } {
			const scroller = document.querySelector(viewLabel === '일' ? '.df-day-content-grid' : '.df-week-time-grid-scroller');
			if (!(scroller instanceof HTMLElement)) throw new Error('Missing visible timeline scroller');
			scroller.scrollTop = 0;
			scroller.dispatchEvent(new Event('scroll', { bubbles: true }));
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
	await expect(page.locator('header').getByRole('button', { name: '오늘' })).toBeVisible();
	await page.waitForSelector(timelineTargetSelector(viewLabel), { state: 'attached' });
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
			scroller.scrollTop = 0;
			scroller.dispatchEvent(new Event('scroll', { bubbles: true }));
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

export async function openCalendarEmbed(page: Page, viewLabel: '일' | '주' | '월'): Promise<void> {
	await page.goto('/calendar/embed?date=2026-06-08');
	await page.evaluate((view) => {
		window.localStorage.setItem('internkim.calendar.view', view);
	}, calendarViewStorageValue(viewLabel));
	await page.reload();
	const button = page.locator('.calendar-view-switcher').getByRole('button', { name: viewLabel, exact: true });
	await expect(button).toHaveClass(/active-view/);
}

export async function navigateEmbeddedCalendar(page: Page, dateKey: string): Promise<void> {
	await page.waitForSelector('.calendar-stage', { state: 'attached' });
	await page.evaluate(
		() =>
			new Promise<void>((resolve) => {
				requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
			})
	);
	await expect
		.poll(async () =>
			page.evaluate((selectedDateKey) => {
				const selectedDate = new Date(`${selectedDateKey}T12:00:00`);
				const selectedDateValue = selectedDate.toISOString();
				window.localStorage.setItem('internkim.calendar.visibleDate', selectedDateValue);
				window.dispatchEvent(
					new StorageEvent('storage', {
						key: 'internkim.calendar.visibleDate',
						newValue: selectedDateValue
					})
				);
				const channel = new BroadcastChannel('internkim-calendar');
				channel.postMessage({ type: 'calendar-navigate', dateKey: selectedDateKey });
				channel.close();
				window.postMessage({ type: 'calendar-navigate', dateKey: selectedDateKey }, window.location.origin);
				return document.querySelector<HTMLElement>('.calendar-stage')?.dataset.calendarSelectedDateKey ?? '';
			}, dateKey)
		)
		.toBe(dateKey);
}

export function calendarViewStorageValue(viewLabel: '일' | '주' | '월'): 'day' | 'week' | 'month' {
	if (viewLabel === '일') return 'day';
	if (viewLabel === '주') return 'week';
	return 'month';
}

export function timelineTargetSelector(viewLabel: '일' | '주'): string {
	if (viewLabel === '일') return '.df-day-content-grid-column';
	return '.df-week-time-grid-cell';
}

export function timelineTargetColumnIndex(viewLabel: '일' | '주'): number {
	if (viewLabel === '일') return 0;
	return 1;
}
