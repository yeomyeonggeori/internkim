import { dayFlowEventSelectorForID } from './calendar-dayflow-dom-adapter';

export function openCalendarEventDetailPanel(stageElement: HTMLElement | null, eventID: string): void {
	const eventElement = calendarEventElementByID(stageElement, eventID);
	if (!eventElement) return;
	eventElement.dispatchEvent(detailOpenEventForElement(eventElement));
}

export function focusCalendarEventElement(stageElement: HTMLElement | null, eventID: string): void {
	const eventElements = calendarEventElementsByID(stageElement, eventID).filter(isVisibleCalendarEventElement);
	const eventElement = eventElements[0] ?? calendarEventElementByID(stageElement, eventID);
	if (!eventElement) return;
	clearFocusedCalendarEventElements(stageElement);
	for (const element of eventElements.length > 0 ? eventElements : [eventElement]) {
		element.classList.add('internkim-calendar-event-focused');
	}
}

export function clearFocusedCalendarEventElements(stageElement: HTMLElement | null): void {
	if (!stageElement) return;
	for (const element of stageElement.querySelectorAll('.internkim-calendar-event-focused')) {
		element.classList.remove('internkim-calendar-event-focused');
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
