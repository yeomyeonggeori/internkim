export const dayTaskSelector = {
	dayAllDayRow: '.df-day-content-all-day-row',
	dayAllDayLane: '.df-day-content-all-day-lane',
	dayTimedEvent: '.df-day-event.df-event-timed',
	monthDateCell: '.df-month-day-cell[data-date]',
	monthEvent: '.df-month-event[data-event-id]',
	monthEventContent: '.df-month-segment-event, .df-content-slot',
	monthLayerEvent: '.df-month-week-event-layer-row .df-month-event[data-event-id]',
	monthLayerRow: '.df-month-week-event-layer-row',
	multiDayProxy: '.calendar-multi-day-all-day-proxy',
	weekAllDayCell: '.df-week-all-day-cell, .df-all-day-cell',
	weekAllDayEventLayer: '.df-week-all-day-event-layer',
	weekAllDayRow: '.df-week-all-day-row-content, .df-all-day-row',
	weekTimedEvent: '.df-week-event.df-event-timed'
} as const;

export function dayTaskEventSelectorForID(eventID: string): string {
	const escapedEventID = cssEscape(eventID);
	return `[data-calendar-event-id="${escapedEventID}"], [data-event-id="${escapedEventID}"], [data-event-id^="${escapedEventID}::"]`;
}

export function visibleDayTaskElements(rootElement: HTMLElement, selector: string): HTMLElement[] {
	return Array.from(rootElement.querySelectorAll<HTMLElement>(selector)).filter(isVisibleDayTaskElement);
}

export function isVisibleDayTaskElement(element: HTMLElement): boolean {
	const rectangle = element.getBoundingClientRect();
	const style = window.getComputedStyle(element);
	return rectangle.width > 0 && rectangle.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
}

export function dayTaskMonthEventContentElement(eventElement: HTMLElement): HTMLElement | null {
	return eventElement.querySelector<HTMLElement>(dayTaskSelector.monthEventContent);
}

export function dayTaskWeekAllDayCells(rowElement: HTMLElement): HTMLElement[] {
	return Array.from(rowElement.querySelectorAll<HTMLElement>(dayTaskSelector.weekAllDayCell));
}

export function dayTaskTimedEventElements(stageElement: HTMLElement): HTMLElement[] {
	return Array.from(
		stageElement.querySelectorAll<HTMLElement>(`${dayTaskSelector.dayTimedEvent}, ${dayTaskSelector.weekTimedEvent}`)
	);
}

function cssEscape(value: string): string {
	if (typeof CSS !== 'undefined' && CSS.escape) return CSS.escape(value);
	return value.replaceAll('\\', '\\\\').replace(/["'./:[\],=]/g, (character) => `\\${character}`);
}
