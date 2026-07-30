import { ViewType } from '../calendar-view-type';

export function refreshSelectedMonthDateCell(stageElement: HTMLElement | null, selectedDateKey: string | null): void {
	if (!stageElement) return;
	if (selectedDateKey) stageElement.dataset.calendarSelectedDateKey = selectedDateKey;
	else delete stageElement.dataset.calendarSelectedDateKey;
	for (const dateCell of stageElement.querySelectorAll('.df-month-day-cell.month-selected-date')) {
		dateCell.classList.remove('month-selected-date');
	}
	for (const weekHeader of stageElement.querySelectorAll('.calendar-selected-week-date')) {
		weekHeader.classList.remove('calendar-selected-week-date');
	}
	if (!selectedDateKey) return;
	monthDateCellByDateKey(stageElement, selectedDateKey)?.element.classList.add('month-selected-date');
	weekHeaderByDateKey(stageElement, selectedDateKey)?.classList.add('calendar-selected-week-date');
}

export function scheduleSelectedMonthDateCellRefresh(
	isBrowser: boolean,
	stageElement: HTMLElement | null,
	selectedDateKey: string | null
): void {
	if (!isBrowser) return;
	requestAnimationFrame(() => {
		refreshSelectedMonthDateCell(stageElement, selectedDateKey);
		requestAnimationFrame(() => refreshSelectedMonthDateCell(stageElement, selectedDateKey));
	});
	window.setTimeout(() => refreshSelectedMonthDateCell(stageElement, selectedDateKey), 0);
}

type CalendarSelectedMonthDateContext = {
	getSelectedDateKey: () => string | null;
	getStageElement: () => HTMLElement | null;
	isBrowser: () => boolean;
	setSelectedDateKey: (dateKey: string) => void;
};

export type CalendarSelectedMonthDateActions = {
	getSelectedMonthDateKey: () => string | null;
	refreshSelectedMonthDateCellAfterRender: () => void;
	selectMonthDate: (dateKey: string) => void;
};

export type CalendarMonthKeyboardNavigationOptions = {
	currentView: () => ViewType;
	getSelectedDateKey: () => string | null;
	navigateToDateKey: (dateKey: string) => void;
	clearSelectedEvent: () => void;
};

export function createCalendarSelectedMonthDateActions(
	context: CalendarSelectedMonthDateContext
): CalendarSelectedMonthDateActions {
	function refreshSelectedMonthDateCellAfterRender(): void {
		scheduleSelectedMonthDateCellRefresh(
			context.isBrowser(),
			context.getStageElement(),
			context.getSelectedDateKey()
		);
	}

	function selectMonthDate(dateKey: string): void {
		context.setSelectedDateKey(dateKey);
		refreshSelectedMonthDateCellAfterRender();
	}

	return {
		getSelectedMonthDateKey: context.getSelectedDateKey,
		refreshSelectedMonthDateCellAfterRender,
		selectMonthDate
	};
}

export function dateFromDateKey(dateKey: string): Date {
	const [year = '0', month = '1', day = '1'] = dateKey.split('-');
	return new Date(Number(year), Number(month) - 1, Number(day));
}

export function dateKeyFromWeekHeaderTarget(stageElement: HTMLElement, target: Element, toolbarDate: Date): string | null {
	const weekHeader = target.closest<HTMLElement>('.df-week-header > .df-week-day-cell, .df-week-day-header');
	if (!weekHeader || !stageElement.contains(weekHeader)) return null;
	const headers = weekHeaderElements(stageElement);
	if (headers.length < 7) return null;
	const headerIndex = headers.indexOf(weekHeader);
	if (headerIndex < 0) return null;
	const weekStartDate = new Date(
		toolbarDate.getFullYear(),
		toolbarDate.getMonth(),
		toolbarDate.getDate() - toolbarDate.getDay(),
		12,
		0,
		0,
		0
	);
	return dateKeyFromDate(
		new Date(
			weekStartDate.getFullYear(),
			weekStartDate.getMonth(),
			weekStartDate.getDate() + headerIndex,
			12,
			0,
			0,
			0
		)
	);
}

export function installCalendarMonthKeyboardNavigation(options: CalendarMonthKeyboardNavigationOptions): () => void {
	function handleKeydown(event: KeyboardEvent): void {
		if (options.currentView() !== ViewType.MONTH) return;
		if (isEditableKeyboardTarget(event.target)) return;
		const dayDelta = monthKeyboardDayDelta(event.key);
		if (dayDelta === 0) return;
		const selectedDateKey = options.getSelectedDateKey();
		if (!selectedDateKey) return;
		event.preventDefault();
		event.stopPropagation();
		options.clearSelectedEvent();
		options.navigateToDateKey(shiftedDateKey(selectedDateKey, dayDelta));
	}

	window.addEventListener('keydown', handleKeydown);
	return () => {
		window.removeEventListener('keydown', handleKeydown);
	};
}

function monthDateCellByDateKey(stageElement: HTMLElement, dateKey: string): { element: HTMLElement; dateKey: string } | null {
	const element = Array.from(stageElement.querySelectorAll<HTMLElement>('.df-month-day-cell[data-date]')).find(
		(candidate) => candidate.dataset.date === dateKey
	);
	if (!element) return null;
	return { element, dateKey };
}

function weekHeaderByDateKey(stageElement: HTMLElement, dateKey: string): HTMLElement | null {
	const date = dateFromDateKey(dateKey);
	const headers = weekHeaderElements(stageElement);
	if (headers.length < 7) return null;
	return headers[date.getDay()] ?? null;
}

function weekHeaderElements(stageElement: HTMLElement): HTMLElement[] {
	return Array.from(stageElement.querySelectorAll<HTMLElement>('.df-week-header > .df-week-day-cell, .df-week-day-header'));
}

function monthKeyboardDayDelta(key: string): number {
	if (key === 'ArrowLeft') return -1;
	if (key === 'ArrowRight') return 1;
	if (key === 'ArrowUp') return -7;
	if (key === 'ArrowDown') return 7;
	return 0;
}

function shiftedDateKey(dateKeyValue: string, dayDelta: number): string {
	const date = dateFromDateKey(dateKeyValue);
	const shiftedDate = new Date(date.getFullYear(), date.getMonth(), date.getDate() + dayDelta);
	return dateKeyFromDate(shiftedDate);
}

export function dateKeyFromDate(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

function isEditableKeyboardTarget(target: EventTarget | null): boolean {
	if (!(target instanceof Element)) return false;
	if (target.closest('input, textarea, select, [contenteditable=""], [contenteditable="true"]')) return true;
	return Boolean(target.closest('.calendar-draft-popover'));
}
