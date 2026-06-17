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
	refreshSelectedMonthDateCellAfterRender: () => void;
	selectMonthDate: (dateKey: string) => void;
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
		refreshSelectedMonthDateCellAfterRender,
		selectMonthDate
	};
}

export function dateFromDateKey(dateKey: string): Date {
	const [year = '0', month = '1', day = '1'] = dateKey.split('-');
	return new Date(Number(year), Number(month) - 1, Number(day));
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
	const headers = Array.from(stageElement.querySelectorAll<HTMLElement>('.df-week-header > .df-week-day-cell, .df-week-day-header'));
	if (headers.length < 7) return null;
	return headers[date.getDay()] ?? null;
}
