import { dayFlowEventSelectorForID } from './calendar-dayflow-dom-adapter';

const calendarEventTargetSelector =
	'[data-event-id].df-event, [data-event-id].df-month-segment-event, .calendar-multi-day-all-day-proxy[data-event-id], .calendar-month-more-popover-event[data-event-id]';

export function calendarEventElementFromTarget(
	stageElement: HTMLElement,
	target: EventTarget | null
): HTMLElement | null {
	if (!(target instanceof Element)) return null;
	const eventElement = target.closest<HTMLElement>(calendarEventTargetSelector);
	if (!eventElement || !stageElement.contains(eventElement)) return null;
	return eventElement;
}

export function normalizedCalendarEventID(eventID: string | undefined): string | null {
	if (!eventID) return null;
	return eventID.split('::')[0] ?? null;
}

export function isCalendarMonthEventLayerElement(eventElement: HTMLElement): boolean {
	return (
		eventElement.classList.contains('calendar-month-direct-event') ||
		eventElement.classList.contains('calendar-month-more-popover-event')
	);
}

export function openCalendarEventDetailPanel(stageElement: HTMLElement | null, eventID: string): void {
	const eventElement = calendarEventElementByID(stageElement, eventID);
	if (!eventElement) return;
	eventElement.dispatchEvent(detailOpenEventForElement(eventElement));
}

export function focusCalendarEventElement(stageElement: HTMLElement | null, eventID: string): void {
	const eventElements = calendarEventElementsByID(stageElement, eventID).filter(isVisibleCalendarEventElement);
	const eventElement = eventElements[0] ?? calendarEventElementByID(stageElement, eventID);
	if (!eventElement) return;
	const focusedElements = eventElements.length > 0 ? eventElements : [eventElement];
	const focusedElementSet = new Set(focusedElements);
	for (const element of stageElement?.querySelectorAll<HTMLElement>('.internkim-calendar-event-focused') ?? []) {
		if (focusedElementSet.has(element)) continue;
		element.classList.remove('internkim-calendar-event-focused');
		syncCalendarEventPressedState(element, false);
	}
	for (const element of focusedElements) {
		if (!element.classList.contains('internkim-calendar-event-focused')) {
			element.classList.add('internkim-calendar-event-focused');
		}
		syncCalendarEventPressedState(element, true);
	}
}

export function clearFocusedCalendarEventElements(stageElement: HTMLElement | null): void {
	if (!stageElement) return;
	for (const element of stageElement.querySelectorAll<HTMLElement>('.internkim-calendar-event-focused')) {
		element.classList.remove('internkim-calendar-event-focused');
		syncCalendarEventPressedState(element, false);
	}
}

export function calendarEventElementsByID(stageElement: HTMLElement | null, eventID: string): HTMLElement[] {
	if (!stageElement) return [];
	return Array.from(stageElement.querySelectorAll<HTMLElement>(dayFlowEventSelectorForID(eventID)));
}

export function isVisibleCalendarEventElement(element: HTMLElement): boolean {
	const rectangle = element.getBoundingClientRect();
	return (
		rectangle.width > 0 &&
		rectangle.height > 0 &&
		rectangle.right > 0 &&
		rectangle.bottom > 0 &&
		rectangle.left < window.innerWidth &&
		rectangle.top < window.innerHeight
	);
}

function calendarEventElementByID(stageElement: HTMLElement | null, eventID: string): HTMLElement | null {
	const eventElements = calendarEventElementsByID(stageElement, eventID);
	return eventElements.find(isVisibleCalendarEventElement) ?? eventElements[0] ?? null;
}

function syncCalendarEventPressedState(element: HTMLElement, isPressed: boolean): void {
	if (!element.hasAttribute('aria-pressed')) return;
	const value = String(isPressed);
	if (element.getAttribute('aria-pressed') !== value) element.setAttribute('aria-pressed', value);
}

function detailOpenEventForElement(element: HTMLElement): MouseEvent {
	const rectangle = element.getBoundingClientRect();
	return new MouseEvent('dblclick', {
		bubbles: true,
		cancelable: true,
		view: window,
		clientX: rectangle.left + rectangle.width / 2,
		clientY: rectangle.top + rectangle.height / 2
	});
}
