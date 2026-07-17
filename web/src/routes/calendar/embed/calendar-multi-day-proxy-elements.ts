export type CalendarMultiDayProxyElementLayout = {
	eventID: string;
	height: number;
	isSelected: boolean;
	label: string;
	left: number;
	top: number;
	width: number;
};

export type ReusableCalendarMultiDayProxyElements = Map<string, HTMLButtonElement>;

export const calendarMultiDayProxyClass = 'calendar-multi-day-all-day-proxy';

const proxyStartClass = 'calendar-multi-day-all-day-proxy-start';
const focusedEventClass = 'internkim-calendar-event-focused';

export function reusableCalendarMultiDayProxyElements(
	stageElement: HTMLElement
): ReusableCalendarMultiDayProxyElements {
	const reusableElements: ReusableCalendarMultiDayProxyElements = new Map();
	for (const proxyElement of stageElement.querySelectorAll<HTMLButtonElement>(`.${calendarMultiDayProxyClass}`)) {
		const eventID = proxyElement.dataset.eventId;
		if (!eventID || reusableElements.has(eventID)) {
			proxyElement.remove();
			continue;
		}
		reusableElements.set(eventID, proxyElement);
	}
	return reusableElements;
}

export function syncCalendarMultiDayProxyElements(
	layerElement: HTMLElement,
	layouts: CalendarMultiDayProxyElementLayout[],
	reusableElements: ReusableCalendarMultiDayProxyElements
): void {
	let previousElement = lastNonProxyElement(layerElement);
	for (const layout of layouts) {
		const proxyElement = reusableElements.get(layout.eventID) ?? document.createElement('button');
		reusableElements.delete(layout.eventID);
		syncProxyElement(proxyElement, layout);
		placeProxyElement(layerElement, proxyElement, previousElement);
		previousElement = proxyElement;
	}
}

export function removeUnusedCalendarMultiDayProxyElements(
	reusableElements: ReusableCalendarMultiDayProxyElements
): void {
	for (const proxyElement of reusableElements.values()) proxyElement.remove();
}

function syncProxyElement(
	proxyElement: HTMLButtonElement,
	layout: CalendarMultiDayProxyElementLayout
): void {
	const className = `${calendarMultiDayProxyClass} df-event${layout.isSelected ? ` ${focusedEventClass}` : ''}`;
	if (proxyElement.type !== 'button') proxyElement.type = 'button';
	if (proxyElement.className !== className) proxyElement.className = className;
	if (proxyElement.dataset.eventId !== layout.eventID) proxyElement.dataset.eventId = layout.eventID;
	setAttributeIfChanged(proxyElement, 'aria-pressed', String(layout.isSelected));
	setStyleIfChanged(proxyElement, 'left', `${layout.left}px`);
	setStyleIfChanged(proxyElement, 'top', `${layout.top}px`);
	setStyleIfChanged(proxyElement, 'width', `${layout.width}px`);
	setStyleIfChanged(proxyElement, 'height', `${layout.height}px`, 'important');
	syncProxyStartElement(proxyElement, layout.label);
}

function syncProxyStartElement(proxyElement: HTMLButtonElement, label: string): void {
	const existingStartElement = proxyElement.querySelector<HTMLElement>(`.${proxyStartClass}`);
	const startElement = existingStartElement ?? document.createElement('span');
	if (!existingStartElement) {
		startElement.className = proxyStartClass;
		proxyElement.replaceChildren(startElement);
	}
	if (startElement.textContent !== label) startElement.textContent = label;
}

function setAttributeIfChanged(element: HTMLElement, name: string, value: string): void {
	if (element.getAttribute(name) !== value) element.setAttribute(name, value);
}

function setStyleIfChanged(element: HTMLElement, property: string, value: string, priority = ''): void {
	if (element.style.getPropertyValue(property) === value && element.style.getPropertyPriority(property) === priority) return;
	element.style.setProperty(property, value, priority);
}

function lastNonProxyElement(layerElement: HTMLElement): HTMLElement | null {
	const childElements = [...layerElement.querySelectorAll<HTMLElement>(':scope > *')];
	return childElements.findLast((element) => !element.classList.contains(calendarMultiDayProxyClass)) ?? null;
}

function placeProxyElement(
	layerElement: HTMLElement,
	proxyElement: HTMLButtonElement,
	previousElement: HTMLElement | null
): void {
	const expectedNode = previousElement?.nextSibling ?? layerElement.firstChild;
	if (proxyElement.parentElement === layerElement && expectedNode === proxyElement) return;
	layerElement.insertBefore(proxyElement, expectedNode);
}
