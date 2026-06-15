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
