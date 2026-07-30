import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import { ViewType, type ViewType as CalendarViewType } from '../calendar-view-type';
import { dayFlowEventSelectorForID } from './calendar-dayflow-dom-adapter';
import { eventEndDate, eventStartDate } from './calendar-event-mapping';

export type CalendarMobileTwoDayWeekLayoutContext = {
	stageElement: HTMLElement | null;
	currentDate: Date;
	events: DayFlowEvent[];
	isMobileTwoDayWeekView: boolean;
	localeCode: string;
};

export const mobileTwoDayWeekDayCount = 2;
export const desktopWeekDayCount = 7;
const activeColumnValue = 'true';
const inactiveColumnValue = 'false';
const mobileRangeDateClass = 'calendar-mobile-two-day-week-range-date';
const mobileColumnHeaderClass = 'calendar-mobile-two-day-week-column-header';
const mobileColumnHeaderCellClass = 'calendar-mobile-two-day-week-column-header-cell';
const mobileColumnWeekendHeaderCellClass = 'calendar-mobile-two-day-week-column-header-cell-weekend';
const mobileEventLayoutDataKey = 'calendarMobileTwoDayEventLayout';
const mobileEventHorizontalInset = 4;
const koreanWeekdayLabels = ['일', '월', '화', '수', '목', '금', '토'];

export function calendarWeekNavigationDayCount(isMobileTwoDayWeekView: boolean): number {
	return isMobileTwoDayWeekView ? mobileTwoDayWeekDayCount : desktopWeekDayCount;
}

export function isCalendarMobileTwoDayWeekView(view: CalendarViewType, isMobile: boolean): boolean {
	return view === ViewType.WEEK && isMobile;
}

export function calendarTimelineDisplayDayCount(
	view: CalendarViewType,
	isMobileTwoDayWeekView: boolean
): number {
	if (view === ViewType.WEEK) return calendarWeekNavigationDayCount(isMobileTwoDayWeekView);
	return 1;
}

export function calendarTimelineDisplayStartDate(
	currentDate: Date,
	view: CalendarViewType,
	isMobileTwoDayWeekView: boolean
): Date {
	if (view === ViewType.WEEK && !isMobileTwoDayWeekView) return startOfWeek(currentDate);
	return startOfDay(currentDate);
}

export function calendarMobileTwoDayWeekDateKeyForColumn(currentDate: Date, columnIndex: number): string {
	const date = addDays(startOfWeek(currentDate), columnIndex);
	return dateKey(date);
}

export function syncCalendarMobileTwoDayWeekLayout(context: CalendarMobileTwoDayWeekLayoutContext): void {
	const { stageElement } = context;
	if (!stageElement) return;
	clearCalendarMobileTwoDayWeekLayout(stageElement);
	if (!context.isMobileTwoDayWeekView) return;
	const activeColumnIndexes = mobileTwoDayWeekColumnIndexes(context.currentDate);
	syncCompactHeaderRange(stageElement, activeColumnIndexes);
	syncTwoDayTimelineColumns(stageElement, context.currentDate);
	syncTwoDayColumnHeader(stageElement, context.currentDate, context.localeCode);
	syncTwoDayAllDayEvents(stageElement, context.currentDate, context.events);
	syncTwoDayTimedEvents(stageElement, context.currentDate, context.events);
}

export function clearCalendarMobileTwoDayWeekLayout(stageElement: HTMLElement | null): void {
	if (!stageElement) return;
	for (const element of stageElement.querySelectorAll<HTMLElement>('[data-calendar-mobile-two-day-active]')) {
		delete element.dataset.calendarMobileTwoDayActive;
	}
	clearAllDayEventLayout(stageElement);
	removeTwoDayColumnHeader(stageElement);
	for (const element of stageElement.querySelectorAll<HTMLElement>(`.${mobileRangeDateClass}`)) {
		element.classList.remove(mobileRangeDateClass);
	}
	for (const scroller of mobileWeekScrollers(stageElement)) {
		scroller.scrollLeft = 0;
	}
}

function syncTwoDayAllDayEvents(stageElement: HTMLElement, currentDate: Date, events: DayFlowEvent[]): void {
	const visibleStartDate = mobileTwoDayWeekStartDate(currentDate);
	for (const event of events) {
		if (!event.allDay) continue;
		for (const eventElement of allDayEventElements(stageElement, event.id)) {
			applyAllDayEventLayout(eventElement, visibleStartDate, event);
		}
	}
}

function applyAllDayEventLayout(eventElement: HTMLElement, visibleStartDate: Date, event: DayFlowEvent): void {
	const visibleStartIndex = localDayDiff(visibleStartDate, eventStartDate(event));
	const visibleEndIndex = localDayDiff(visibleStartDate, eventEndDate(event));
	if (visibleEndIndex < 0 || visibleStartIndex >= mobileTwoDayWeekDayCount) {
		eventElement.dataset[mobileEventLayoutDataKey] = activeColumnValue;
		eventElement.style.display = 'none';
		return;
	}
	const startColumnIndex = clamp(visibleStartIndex, 0, mobileTwoDayWeekDayCount - 1);
	const endColumnIndex = clamp(visibleEndIndex, 0, mobileTwoDayWeekDayCount - 1);
	const left = (startColumnIndex / mobileTwoDayWeekDayCount) * 100;
	const width = ((endColumnIndex - startColumnIndex + 1) / mobileTwoDayWeekDayCount) * 100;
	eventElement.style.display = '';
	eventElement.style.left = `calc(${left}% + ${mobileEventHorizontalInset}px)`;
	eventElement.style.width = `calc(${width}% - ${mobileEventHorizontalInset * 2}px)`;
	eventElement.style.right = 'auto';
	eventElement.dataset[mobileEventLayoutDataKey] = activeColumnValue;
}

function clearAllDayEventLayout(stageElement: HTMLElement): void {
	for (const eventElement of stageElement.querySelectorAll<HTMLElement>('[data-calendar-mobile-two-day-event-layout]')) {
		eventElement.style.removeProperty('display');
		eventElement.style.removeProperty('left');
		eventElement.style.removeProperty('right');
		eventElement.style.removeProperty('width');
		delete eventElement.dataset[mobileEventLayoutDataKey];
	}
}

function allDayEventElements(stageElement: HTMLElement, eventID: string): HTMLElement[] {
	return Array.from(
		stageElement.querySelectorAll<HTMLElement>(`.df-week-all-day-event-layer ${dayFlowEventSelectorForID(eventID)}`)
	);
}

function syncTwoDayTimedEvents(stageElement: HTMLElement, currentDate: Date, events: DayFlowEvent[]): void {
	const visibleStartDate = mobileTwoDayWeekStartDate(currentDate);
	const columnCells = activeTimeColumnCells(stageElement);
	for (const event of events) {
		if (event.allDay) continue;
		for (const eventElement of timedEventElements(stageElement, event.id)) {
			applyTimedEventLayout(eventElement, visibleStartDate, event, columnCells);
		}
	}
}

function applyTimedEventLayout(
	eventElement: HTMLElement,
	visibleStartDate: Date,
	event: DayFlowEvent,
	columnCells: HTMLElement[]
): void {
	const visibleStartIndex = localDayDiff(visibleStartDate, eventStartDate(event));
	if (visibleStartIndex < 0 || visibleStartIndex >= mobileTwoDayWeekDayCount) {
		eventElement.dataset[mobileEventLayoutDataKey] = activeColumnValue;
		eventElement.style.display = 'none';
		return;
	}
	const columnCell = columnCells[visibleStartIndex];
	const parentElement = eventElement.offsetParent instanceof HTMLElement ? eventElement.offsetParent : eventElement.parentElement;
	if (!columnCell || !parentElement) {
		eventElement.dataset[mobileEventLayoutDataKey] = activeColumnValue;
		eventElement.style.display = 'none';
		return;
	}
	const columnRectangle = columnCell.getBoundingClientRect();
	const parentRectangle = parentElement.getBoundingClientRect();
	eventElement.style.display = '';
	eventElement.style.left = `${columnRectangle.left - parentRectangle.left + mobileEventHorizontalInset}px`;
	eventElement.style.width = `${Math.max(columnRectangle.width - mobileEventHorizontalInset * 2, 0)}px`;
	eventElement.style.right = 'auto';
	eventElement.dataset[mobileEventLayoutDataKey] = activeColumnValue;
}

function timedEventElements(stageElement: HTMLElement, eventID: string): HTMLElement[] {
	return Array.from(stageElement.querySelectorAll<HTMLElement>(dayFlowEventSelectorForID(eventID))).filter(
		(element) => element.classList.contains('df-week-event') && element.classList.contains('df-event-timed')
	);
}

function activeTimeColumnCells(stageElement: HTMLElement): HTMLElement[] {
	const firstTimeRow = stageElement.querySelector<HTMLElement>('.df-time-grid-row');
	if (!firstTimeRow) return [];
	return Array.from(firstTimeRow.children).filter(
		(element): element is HTMLElement =>
			element instanceof HTMLElement &&
			element.classList.contains('df-week-time-grid-cell') &&
			element.dataset.calendarMobileTwoDayActive === activeColumnValue
	);
}

function syncCompactHeaderRange(stageElement: HTMLElement, activeColumnIndexes: Set<number>): void {
	const dateButtons = Array.from(stageElement.querySelectorAll<HTMLElement>('.df-compact-header-date-button'));
	dateButtons.forEach((dateButton, index) => {
		if (activeColumnIndexes.has(index)) dateButton.classList.add(mobileRangeDateClass);
	});
}

function syncTwoDayTimelineColumns(stageElement: HTMLElement, currentDate: Date): void {
	for (const scroller of mobileWeekScrollers(stageElement)) {
		scroller.scrollLeft = 0;
	}
	syncColumnGroup(stageElement.querySelector<HTMLElement>('.df-week-header'), currentDate);
	syncColumnGroup(stageElement.querySelector<HTMLElement>('.df-week-all-day-row-content'), currentDate);
	for (const row of stageElement.querySelectorAll<HTMLElement>('.df-time-grid-row, .df-week-time-grid-boundary-row')) {
		syncColumnGroup(row, currentDate);
	}
}

function syncTwoDayColumnHeader(stageElement: HTMLElement, currentDate: Date, localeCode: string): void {
	const contentWrap = stageElement.querySelector<HTMLElement>('.df-week-all-day-content-wrap');
	if (!contentWrap) return;
	const header = ensureTwoDayColumnHeader(contentWrap);
	const visibleStartDate = mobileTwoDayWeekStartDate(currentDate);
	header.replaceChildren(
		...Array.from({ length: mobileTwoDayWeekDayCount }, (_, index) =>
			createTwoDayColumnHeaderCell(addDays(visibleStartDate, index), localeCode)
		)
	);
}

function ensureTwoDayColumnHeader(contentWrap: HTMLElement): HTMLElement {
	const existingHeader = contentWrap.querySelector<HTMLElement>(`:scope > .${mobileColumnHeaderClass}`);
	if (existingHeader) return existingHeader;
	const header = document.createElement('div');
	header.className = mobileColumnHeaderClass;
	contentWrap.insertBefore(header, contentWrap.firstChild);
	return header;
}

function createTwoDayColumnHeaderCell(date: Date, localeCode: string): HTMLElement {
	const cell = document.createElement('div');
	cell.className = mobileColumnHeaderCellClass;
	if (date.getDay() === 0 || date.getDay() === 6) cell.classList.add(mobileColumnWeekendHeaderCellClass);
	cell.textContent = mobileTwoDayColumnHeaderText(date, localeCode);
	return cell;
}

function mobileTwoDayColumnHeaderText(date: Date, localeCode: string): string {
	if (localeCode.startsWith('ko')) {
		return `${date.getMonth() + 1}월 ${date.getDate()}일 ${koreanWeekdayLabels[date.getDay()]}`;
	}
	return new Intl.DateTimeFormat(localeCode, {
		weekday: 'short',
		month: 'short',
		day: 'numeric'
	}).format(date);
}

function removeTwoDayColumnHeader(stageElement: HTMLElement): void {
	for (const header of stageElement.querySelectorAll<HTMLElement>(`.${mobileColumnHeaderClass}`)) {
		header.remove();
	}
}

function syncColumnGroup(parentElement: HTMLElement | null, currentDate: Date): void {
	if (!parentElement) return;
	const cells = Array.from(parentElement.children).filter(isWeekColumnCell);
	const activeColumnIndexes = mobileTwoDayWeekColumnIndexes(currentDate, cells.length);
	cells.forEach((cell, index) => {
		cell.dataset.calendarMobileTwoDayActive = activeColumnIndexes.has(index) ? activeColumnValue : inactiveColumnValue;
	});
}

function isWeekColumnCell(element: Element): element is HTMLElement {
	return (
		element instanceof HTMLElement &&
		(element.classList.contains('df-week-all-day-cell') ||
			element.classList.contains('df-all-day-cell') ||
			element.classList.contains('df-week-day-cell') ||
			element.classList.contains('df-week-time-grid-cell') ||
			element.classList.contains('df-week-time-grid-boundary-cell'))
	);
}

function mobileWeekScrollers(stageElement: HTMLElement): HTMLElement[] {
	return Array.from(
		stageElement.querySelectorAll<HTMLElement>('.df-week-all-day-content-wrap, .df-week-time-grid-scroller')
	);
}

function mobileTwoDayWeekColumnIndexes(currentDate: Date, columnCount = desktopWeekDayCount): Set<number> {
	const weekStartDate = startOfWeek(currentDate);
	const currentColumnIndex = localDayDiff(weekStartDate, currentDate);
	const lastFirstColumnIndex = Math.max(columnCount - mobileTwoDayWeekDayCount, 0);
	const firstColumnIndex = Math.min(Math.max(currentColumnIndex, 0), lastFirstColumnIndex);
	return new Set([firstColumnIndex, firstColumnIndex + 1]);
}

function mobileTwoDayWeekStartDate(currentDate: Date): Date {
	const currentColumnIndex = localDayDiff(startOfWeek(currentDate), currentDate);
	const lastFirstColumnIndex = desktopWeekDayCount - mobileTwoDayWeekDayCount;
	return addDays(startOfWeek(currentDate), clamp(currentColumnIndex, 0, lastFirstColumnIndex));
}

function clamp(value: number, minimum: number, maximum: number): number {
	return Math.min(Math.max(value, minimum), maximum);
}

function startOfWeek(date: Date): Date {
	const weekStartDate = startOfDay(date);
	weekStartDate.setDate(weekStartDate.getDate() - weekStartDate.getDay());
	return weekStartDate;
}

function startOfDay(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function addDays(date: Date, days: number): Date {
	const nextDate = startOfDay(date);
	nextDate.setDate(nextDate.getDate() + days);
	return nextDate;
}

function localDayDiff(startDate: Date, targetDate: Date): number {
	return Math.round((startOfDay(targetDate).getTime() - startOfDay(startDate).getTime()) / (24 * 60 * 60 * 1000));
}

function dateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}
