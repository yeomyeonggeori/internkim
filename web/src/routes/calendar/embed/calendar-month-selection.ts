export function refreshSelectedMonthDateCell(stageElement: HTMLElement | null, selectedDateKey: string | null): void {
	if (!stageElement) return;
	for (const dateCell of stageElement.querySelectorAll('.df-month-day-cell.month-selected-date')) {
		dateCell.classList.remove('month-selected-date');
	}
	if (!selectedDateKey) return;
	monthDateCellByDateKey(stageElement, selectedDateKey)?.element.classList.add('month-selected-date');
}

export function dateFromDateKey(dateKey: string): Date {
	const [year = '0', month = '1', day = '1'] = dateKey.split('-');
	return new Date(Number(year), Number(month) - 1, Number(day));
}

function monthDateCellByDateKey(stageElement: HTMLElement, dateKey: string): { element: HTMLElement; dateKey: string } | null {
	const escapedDateKey = window.CSS?.escape(dateKey) ?? dateKey;
	const element = stageElement.querySelector<HTMLElement>(`.df-month-day-cell[data-date="${escapedDateKey}"]`);
	if (!element) return null;
	return { element, dateKey };
}
