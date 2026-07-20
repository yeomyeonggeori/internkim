import { expect, type Locator, type Page } from '@playwright/test';

export const mobileEventHorizontalInset = 4;

type WeekGridMeasurements = {
	allDayCellRects: ElementRect[];
	allDayEventRects: EventRect[];
	allDayLabelText: string;
	allDayBottom: number;
	allDayContentBackground: string;
	allDayContentBackgroundImage: string;
	allDayRightEdges: number[];
	allDayShellBackground: string;
	allDayShellOpacity: string;
	compactHeaderBackground: string;
	compactDateTexts: string[];
	compactHeaderLabelColors: string[];
	customHeaderBackgrounds: string[];
	customHeaderBorderColors: string[];
	customHeaderColors: string[];
	customHeaderTexts: string[];
	desktopHeaderLabels: string[];
	desktopHeaderRightEdges: number[];
	hasCompactHeaderBottomLine: boolean;
	highlightedDateTexts: string[];
	isAllDayLabelPainted: boolean;
	firstVisibleTimeLabel: string;
	rangePillBackgrounds: string[];
	rangePillColors: string[];
	stageBackground: string;
	timeCellBorderColors: string[];
	timeScrollerBackground: string;
	timeScrollerBackgroundImage: string;
	timeCellRects: ElementRect[];
	timedEventRects: EventRect[];
	timeRightEdges: number[];
	timeTop: number;
};

type ElementRect = {
	left: number;
	right: number;
	width: number;
};

type EventRect = ElementRect & {
	id: string;
};

export async function selectedDateKey(page: Page): Promise<string> {
	return page.evaluate(() => document.querySelector<HTMLElement>('.calendar-stage')?.dataset.calendarSelectedDateKey ?? '');
}

export async function activeMobileEditorField(page: Page): Promise<string> {
	return page.evaluate(() => {
		const element = document.activeElement;
		return element instanceof HTMLElement ? (element.dataset.mobileEditorField ?? '') : '';
	});
}

export async function renderedEventIDs(page: Page): Promise<string[]> {
	return page.locator('[data-event-id]').evaluateAll((elements) =>
		elements
			.map((element) => element.getAttribute('data-event-id'))
			.filter((eventID): eventID is string => Boolean(eventID))
	);
}

export async function enableDarkMode(page: Page): Promise<void> {
	await page.evaluate(() => {
		document.documentElement.classList.add('dark');
	});
	await page.evaluate(
		() =>
			new Promise<void>((resolve) => {
				requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
			})
	);
}

export async function verifyMobileEditorControls(page: Page): Promise<void> {
	const editor = dayFlowMobileEditor(page);
	const startDateInput = editor.locator('input[data-mobile-editor-field="startDate"]');
	const startTimeInput = editor.locator('input[data-mobile-editor-field="startTime"]');
	const allDayInput = editor.locator('input[data-mobile-editor-field="allDay"]');
	await startDateInput.click();
	await expect.poll(() => activeMobileEditorField(page)).toBe('startDate');
	await startDateInput.fill('2026-06-02');
	await expect(startDateInput).toHaveValue('2026-06-02');
	await startTimeInput.click();
	await expect.poll(() => activeMobileEditorField(page)).toBe('startTime');
	await startTimeInput.fill('11:15');
	await expect(startTimeInput).toHaveValue('11:15');
	await editor.locator('[data-mobile-editor-switch-field]').click();
	await expect(allDayInput).toBeChecked();
	await expect(startTimeInput).toHaveCount(0);
}

export async function weekGridMeasurements(page: Page): Promise<WeekGridMeasurements> {
	return page.evaluate(() => {
		const isVisibleElement = (element: HTMLElement): boolean => {
			const rectangle = element.getBoundingClientRect();
			const style = window.getComputedStyle(element);
			return rectangle.width > 0 && rectangle.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
		};
		const visibleElements = (selector: string): HTMLElement[] =>
			Array.from(document.querySelectorAll<HTMLElement>(selector)).filter(isVisibleElement);
		const elementRect = (element: HTMLElement): ElementRect => {
			const rectangle = element.getBoundingClientRect();
			return {
				left: Math.round(rectangle.left * 100) / 100,
				right: Math.round(rectangle.right * 100) / 100,
				width: Math.round(rectangle.width * 100) / 100
			};
		};
		const rightEdges = (elements: HTMLElement[]): number[] =>
			elements.map((element) => Math.round(element.getBoundingClientRect().right * 100) / 100);
		const compactDateButtons = visibleElements('.df-compact-header-date-button');
		const highlightedDateButtons = visibleElements('.df-compact-header-date-button.calendar-mobile-two-day-week-range-date');
		const desktopHeaderCells = visibleElements('.df-week-header > .df-week-day-cell');
		const allDayCells = visibleElements('.df-week-all-day-row-content > .df-week-all-day-cell, .df-all-day-row > .df-all-day-cell');
		const firstTimeRow = document.querySelector<HTMLElement>('.df-time-grid-row');
		const timeCells = Array.from(firstTimeRow?.children ?? []).filter(
			(element): element is HTMLElement =>
				element instanceof HTMLElement && element.classList.contains('df-week-time-grid-cell') && isVisibleElement(element)
		);
		const allDayEvents = visibleElements('.df-week-all-day-event-layer [data-event-id]');
		const timedEvents = visibleElements('.df-week-event.df-event-timed[data-event-id]');
		const compactHeader = document.querySelector<HTMLElement>('.df-compact-header');
		const compactHeaderStyle = compactHeader ? window.getComputedStyle(compactHeader) : null;
		const compactHeaderLabels = Array.from(document.querySelectorAll<HTMLElement>('.df-compact-header-label'));
		const rangePills = visibleElements('.df-compact-header-date-button.calendar-mobile-two-day-week-range-date .df-compact-header-date-pill');
		const allDayShell = document.querySelector<HTMLElement>('.df-week-all-day-shell');
		const allDayShellStyle = allDayShell ? window.getComputedStyle(allDayShell) : null;
		const allDayContentWrap = document.querySelector<HTMLElement>('.df-week-all-day-content-wrap');
		const allDayContentWrapStyle = allDayContentWrap ? window.getComputedStyle(allDayContentWrap) : null;
		const timeScroller = document.querySelector<HTMLElement>('.df-week-time-grid-scroller');
		const timeScrollerStyle = timeScroller ? window.getComputedStyle(timeScroller) : null;
		const stage = document.querySelector<HTMLElement>('.calendar-stage');
		const stageStyle = stage ? window.getComputedStyle(stage) : null;
		const allDayLabel = document.querySelector<HTMLElement>('.df-week-all-day-label, .df-all-day-label');
		const allDayLabelRectangle = allDayLabel?.getBoundingClientRect();
		const topElementAtAllDayLabel =
			allDayLabelRectangle && allDayLabelRectangle.width > 0 && allDayLabelRectangle.height > 0
				? document.elementFromPoint(
						allDayLabelRectangle.left + allDayLabelRectangle.width / 2,
						allDayLabelRectangle.top + allDayLabelRectangle.height / 2
					)
				: null;
		const firstVisibleTimeLabel =
			Array.from(document.querySelectorAll<HTMLElement>('.df-week-time-grid-time-column .df-time-label')).find((element) => {
				const rectangle = element.getBoundingClientRect();
				const style = window.getComputedStyle(element);
				return rectangle.width > 0 && rectangle.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			})?.textContent ?? '';
		return {
			allDayCellRects: allDayCells.map(elementRect),
			allDayEventRects: allDayEvents.map((element) => ({
				id: element.dataset.eventId ?? '',
				...elementRect(element)
			})),
			allDayLabelText: allDayLabel?.textContent?.replace(/\s+/g, ' ').trim() ?? '',
			allDayBottom: Math.round((allDayShell?.getBoundingClientRect().bottom ?? 0) * 100) / 100,
			allDayContentBackground: allDayContentWrapStyle?.backgroundColor ?? '',
			allDayContentBackgroundImage: allDayContentWrapStyle?.backgroundImage ?? '',
			allDayRightEdges: rightEdges(allDayCells),
			allDayShellBackground: allDayShellStyle?.backgroundColor ?? '',
			allDayShellOpacity: allDayShellStyle?.opacity ?? '',
			compactHeaderBackground: compactHeaderStyle?.backgroundColor ?? '',
			compactDateTexts: compactDateButtons.map((element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? ''),
			compactHeaderLabelColors: compactHeaderLabels.map((element) => window.getComputedStyle(element).color),
			customHeaderBackgrounds: visibleElements('.calendar-mobile-two-day-week-column-header-cell').map(
				(element) => window.getComputedStyle(element.parentElement ?? element).backgroundColor
			),
			customHeaderBorderColors: visibleElements('.calendar-mobile-two-day-week-column-header-cell').map(
				(element) => window.getComputedStyle(element).borderRightColor
			),
			customHeaderTexts: visibleElements('.calendar-mobile-two-day-week-column-header-cell').map(
				(element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? ''
			),
			customHeaderColors: visibleElements('.calendar-mobile-two-day-week-column-header-cell').map(
				(element) => window.getComputedStyle(element).color
			),
			desktopHeaderLabels: desktopHeaderCells.map((element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? ''),
			desktopHeaderRightEdges: rightEdges(desktopHeaderCells),
			hasCompactHeaderBottomLine:
				Number.parseFloat(compactHeaderStyle?.borderBottomWidth ?? '0') >= 1 &&
				compactHeaderStyle?.borderBottomColor !== 'rgba(0, 0, 0, 0)',
			highlightedDateTexts: highlightedDateButtons.map((element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? ''),
			isAllDayLabelPainted: Boolean(allDayLabel && topElementAtAllDayLabel && allDayLabel.contains(topElementAtAllDayLabel)),
			firstVisibleTimeLabel: firstVisibleTimeLabel.replace(/\s+/g, ' ').trim(),
			rangePillBackgrounds: rangePills.map((element) => window.getComputedStyle(element).backgroundColor),
			rangePillColors: rangePills.map((element) => window.getComputedStyle(element).color),
			stageBackground: stageStyle?.backgroundColor ?? '',
			timeCellBorderColors: timeCells.map((element) => window.getComputedStyle(element).borderRightColor),
			timeScrollerBackground: timeScrollerStyle?.backgroundColor ?? '',
			timeScrollerBackgroundImage: timeScrollerStyle?.backgroundImage ?? '',
			timeCellRects: timeCells.map(elementRect),
			timedEventRects: timedEvents.map((element) => ({
				id: element.dataset.eventId ?? '',
				...elementRect(element)
			})),
			timeRightEdges: rightEdges(timeCells),
			timeTop: Math.round((timeScroller?.getBoundingClientRect().top ?? 0) * 100) / 100
		};
	});
}

export async function clickFirstVisibleTimeCell(page: Page): Promise<void> {
	await clickFirstVisibleCell(page, '.df-time-grid-row .df-week-time-grid-cell', 'time');
}

export async function clickFirstVisibleAllDayCell(page: Page): Promise<void> {
	await clickFirstVisibleCell(page, '.df-week-all-day-cell', 'all-day');
}

async function clickFirstVisibleCell(page: Page, selector: string, cellKind: 'all-day' | 'time'): Promise<void> {
	await page.evaluate(
		({ selector, cellKind }) => {
			const visibleCells = Array.from(document.querySelectorAll(selector)).filter((element): element is HTMLElement => {
				if (!(element instanceof HTMLElement)) return false;
				const rectangle = element.getBoundingClientRect();
				const style = window.getComputedStyle(element);
				return rectangle.width > 0 && rectangle.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			});
			const target = visibleCells[0];
			if (!(target instanceof HTMLElement)) throw new Error(`Missing visible week ${cellKind} cell`);
			const rectangle = target.getBoundingClientRect();
			const clientX = rectangle.left + rectangle.width / 2;
			const clientY = rectangle.top + (cellKind === 'time' ? Math.min(120, rectangle.height / 2) : rectangle.height / 2);
			target.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
			target.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
			target.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		},
		{ selector, cellKind }
	);
}

export async function closeDayFlowMobileEditor(page: Page): Promise<void> {
	await dayFlowMobileEditor(page).getByRole('button', { name: '취소' }).click();
	await expect(dayFlowMobileEditor(page)).toHaveCount(0);
}

export function dayFlowMobileEditor(page: Page): Locator {
	return page.locator('.calendar-mobile-event-editor .df-mobile-event-drawer-panel, .df-dialog-container');
}

export function maximumEdgeDelta(firstEdges: number[], secondEdges: number[]): number {
	if (firstEdges.length !== secondEdges.length) return Number.POSITIVE_INFINITY;
	return Math.max(...firstEdges.map((edge, index) => Math.abs(edge - secondEdges[index])));
}
