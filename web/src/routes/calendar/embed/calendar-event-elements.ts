export function openCalendarEventDetailPanel(stageElement: HTMLElement | null, eventID: string): void {
	const eventElement = calendarEventElementByID(stageElement, eventID);
	if (!eventElement) return;
	eventElement.dispatchEvent(detailOpenEventForElement(eventElement));
}

export function calendarEventElementsByID(stageElement: HTMLElement | null, eventID: string): HTMLElement[] {
	if (!stageElement) return [];
	const escapedEventID = window.CSS?.escape(eventID) ?? eventID.replaceAll('"', '\\"');
	return Array.from(
		stageElement.querySelectorAll<HTMLElement>(
			`[data-event-id="${escapedEventID}"], [data-event-id^="${escapedEventID}::"]`
		)
	);
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
