import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import { eventEndDate, eventStartDate } from './calendar-event-mapping';

export type MiniCalendarWeekdayLabels = readonly [string, string, string, string, string, string, string];

export type DayFlowMiniCalendarEnhancementContext = {
	stageElement: HTMLElement | null;
	currentDate: Date;
	events: DayFlowEvent[];
	localeCode: string;
	weekdayLabels: MiniCalendarWeekdayLabels;
	previousLabel: string;
	nextLabel: string;
	selectDateKey: (dateKey: string) => void;
};

const sundayFirstWeekdayIndexes = [0, 1, 2, 3, 4, 5, 6];

export function enhanceDayFlowMiniCalendar(context: DayFlowMiniCalendarEnhancementContext): void {
	if (!context.stageElement) return;
	removeRightPanelCalendarHeader(context.stageElement);
	const month = new Date(context.currentDate.getFullYear(), context.currentDate.getMonth(), 1);
	const firstDate = miniCalendarGridStartDate(month);
	const eventDateKeys = new Set(context.events.flatMap(eventDateKeysFromDayFlowEvent));
	const todayDateKey = dateKey(new Date());
	const selectedDateKey = dateKey(context.currentDate);
	const dayButtons = context.stageElement.querySelectorAll<HTMLElement>('.df-mini-calendar-day');
	dayButtons.forEach((button, index) => {
		const buttonDate = new Date(firstDate);
		buttonDate.setDate(firstDate.getDate() + index);
		enhanceMiniCalendarDay(button, buttonDate, month, selectedDateKey, todayDateKey, eventDateKeys, context.selectDateKey);
	});
	enhanceMiniCalendarWeekdayHeaders(context.stageElement, context.weekdayLabels);
	enhanceMiniCalendarMonthLabel(context.stageElement, month, context.localeCode);
	enhanceMiniCalendarNavigationButtons(context.stageElement, context.currentDate, context.previousLabel, context.nextLabel, context.selectDateKey);
}

export function installDayFlowMiniCalendarDateSelection(selectDateKey: (dateKey: string) => void): () => void {
	const handleClick = (event: MouseEvent): void => {
		const target =
			event.target instanceof Element
				? event.target.closest<HTMLElement>('.df-mini-calendar-day[data-mini-date-key], .df-mini-calendar-nav-btn[data-mini-date-key]')
				: null;
		if (!target) return;
		event.preventDefault();
		event.stopImmediatePropagation();
		const selectedDateKey = target.dataset.miniDateKey;
		if (!selectedDateKey) return;
		selectDateKey(selectedDateKey);
	};
	document.addEventListener('click', handleClick, { capture: true });
	return () => {
		document.removeEventListener('click', handleClick, { capture: true });
	};
}

function removeRightPanelCalendarHeader(stageElement: HTMLElement): void {
	stageElement.querySelector('.df-right-panel-calendar-header')?.remove();
}

function enhanceMiniCalendarDay(
	button: HTMLElement,
	buttonDate: Date,
	month: Date,
	selectedDateKey: string,
	todayDateKey: string,
	eventDateKeys: Set<string>,
	selectDateKey: (dateKey: string) => void
): void {
	const buttonDateKey = dateKey(buttonDate);
	button.setAttribute('type', 'button');
	button.dataset.miniDateKey = buttonDateKey;
	button.dataset.selected = String(buttonDateKey === selectedDateKey);
	button.dataset.today = String(buttonDateKey === todayDateKey);
	button.dataset.weekend = String(isWeekendDate(buttonDate));
	button.dataset.otherMonth = String(buttonDate.getMonth() !== month.getMonth());
	button.setAttribute('aria-pressed', String(buttonDateKey === selectedDateKey));
	button.textContent = String(buttonDate.getDate());
	ensureEventDotSlot(button).dataset.hasEvent = String(eventDateKeys.has(buttonDateKey));
	installMiniCalendarDateSelection(button, selectDateKey);
}

function installMiniCalendarDateSelection(button: HTMLElement, selectDateKey: (dateKey: string) => void): void {
	if (button.dataset.dateSelectionInstalled === 'true') return;
	button.dataset.dateSelectionInstalled = 'true';
	button.addEventListener(
		'click',
		(event) => {
			event.preventDefault();
			event.stopImmediatePropagation();
			const selectedDateKey = button.dataset.miniDateKey;
			if (!selectedDateKey) return;
			selectDateKey(selectedDateKey);
		},
		{ capture: true }
	);
	button.addEventListener('keydown', (event) => {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		const selectedDateKey = button.dataset.miniDateKey;
		if (!selectedDateKey) return;
		selectDateKey(selectedDateKey);
	});
}

function ensureEventDotSlot(button: HTMLElement): HTMLElement {
	const existingSlot = button.querySelector<HTMLElement>('.mini-month-event-dot-slot');
	if (existingSlot) return existingSlot;
	const slot = document.createElement('span');
	slot.className = 'mini-month-event-dot-slot';
	button.appendChild(slot);
	return slot;
}

function enhanceMiniCalendarWeekdayHeaders(stageElement: HTMLElement, weekdayLabels: MiniCalendarWeekdayLabels): void {
	const headers = Array.from(stageElement.querySelectorAll<HTMLElement>('.df-mini-calendar-header'));
	headers.forEach((header, index) => {
		const weekdayIndex = sundayFirstWeekdayIndexes[index];
		if (weekdayIndex === undefined) return;
		header.textContent = weekdayLabels[weekdayIndex];
		header.dataset.miniWeekdayIndex = String(weekdayIndex);
		header.dataset.weekend = String(weekdayIndex === 0 || weekdayIndex === 6);
	});
}

function enhanceMiniCalendarMonthLabel(stageElement: HTMLElement, month: Date, localeCode: string): void {
	const label = stageElement.querySelector<HTMLElement>('.df-mini-calendar-month-label');
	if (!label) return;
	label.textContent = month.toLocaleDateString(localeCode, {
		year: 'numeric',
		month: 'long'
	});
	label.setAttribute('role', 'button');
	label.setAttribute('tabindex', '0');
	label.setAttribute('aria-haspopup', 'dialog');
}

function enhanceMiniCalendarNavigationButtons(
	stageElement: HTMLElement,
	currentDate: Date,
	previousLabel: string,
	nextLabel: string,
	selectDateKey: (dateKey: string) => void
): void {
	const buttons = stageElement.querySelectorAll<HTMLButtonElement>('.df-mini-calendar-header-nav .df-mini-calendar-nav-btn');
	buttons.forEach((button, index) => {
		button.setAttribute('aria-label', index === 0 ? previousLabel : nextLabel);
		installMiniCalendarNavigation(button, currentDate, index === 0 ? -1 : 1, selectDateKey);
	});
}

function installMiniCalendarNavigation(
	button: HTMLButtonElement,
	currentDate: Date,
	monthDelta: -1 | 1,
	selectDateKey: (dateKey: string) => void
): void {
	button.dataset.miniDateKey = dateKey(shiftedMonthDate(currentDate, monthDelta));
	if (button.dataset.monthNavigationInstalled === 'true') return;
	button.dataset.monthNavigationInstalled = 'true';
	button.addEventListener(
		'click',
		(event) => {
			event.preventDefault();
			event.stopImmediatePropagation();
			const selectedDateKey = button.dataset.miniDateKey;
			if (!selectedDateKey) return;
			selectDateKey(selectedDateKey);
		},
		{ capture: true }
	);
	button.addEventListener('keydown', (event) => {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		const selectedDateKey = button.dataset.miniDateKey;
		if (!selectedDateKey) return;
		selectDateKey(selectedDateKey);
	});
}

function shiftedMonthDate(currentDate: Date, monthDelta: -1 | 1): Date {
	const targetYear = currentDate.getFullYear();
	const targetMonth = currentDate.getMonth() + monthDelta;
	const daysInTargetMonth = new Date(targetYear, targetMonth + 1, 0).getDate();
	const targetDay = Math.min(currentDate.getDate(), daysInTargetMonth);
	return new Date(targetYear, targetMonth, targetDay, 12, 0, 0, 0);
}

function miniCalendarGridStartDate(month: Date): Date {
	const firstDate = new Date(month.getFullYear(), month.getMonth(), 1);
	const offset = firstDate.getDay();
	firstDate.setDate(firstDate.getDate() - offset);
	return firstDate;
}

function eventDateKeysFromDayFlowEvent(event: DayFlowEvent): string[] {
	const startDate = eventStartDate(event);
	const endDate = adjustedInclusiveEndDate(startDate, eventEndDate(event));
	const keys: string[] = [];
	const cursor = new Date(startDate.getFullYear(), startDate.getMonth(), startDate.getDate());
	const end = new Date(endDate.getFullYear(), endDate.getMonth(), endDate.getDate());
	while (cursor <= end) {
		keys.push(dateKey(cursor));
		cursor.setDate(cursor.getDate() + 1);
	}
	return keys;
}

function adjustedInclusiveEndDate(startDate: Date, endDate: Date): Date {
	const adjustedEndDate = new Date(endDate);
	if (
		adjustedEndDate.getHours() === 0 &&
		adjustedEndDate.getMinutes() === 0 &&
		adjustedEndDate.getSeconds() === 0 &&
		adjustedEndDate.getMilliseconds() === 0 &&
		dateKey(adjustedEndDate) !== dateKey(startDate)
	) {
		adjustedEndDate.setDate(adjustedEndDate.getDate() - 1);
	}
	return adjustedEndDate;
}

function isWeekendDate(date: Date): boolean {
	const day = date.getDay();
	return day === 0 || day === 6;
}

function dateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}
