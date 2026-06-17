import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import { calendarEventElementsByID, isVisibleCalendarEventElement } from './calendar-event-elements';

type CalendarEventAnchorCapture = {
	eventID: string;
	anchor: DraftPopoverAnchor;
	capturedAt: number;
};

let lastEventAnchorCapture: CalendarEventAnchorCapture | null = null;

export function installCalendarEventAnchorCapture(stageElement: HTMLElement): () => void {
	const handlePointerDown = (event: PointerEvent): void => {
		if (!(event.target instanceof Element)) return;
		const eventElement = event.target.closest<HTMLElement>('[data-event-id].df-event, .calendar-multi-day-all-day-proxy[data-event-id]');
		if (!eventElement) return;
		const eventID = normalizedEventID(eventElement.dataset.eventId);
		if (!eventID) return;
		lastEventAnchorCapture = {
			eventID,
			anchor: anchorFromElement(eventElement),
			capturedAt: Date.now()
		};
	};

	stageElement.addEventListener('pointerdown', handlePointerDown, true);

	return () => {
		stageElement.removeEventListener('pointerdown', handlePointerDown, true);
	};
}

export function recentCalendarEventAnchorForID(eventID: string): DraftPopoverAnchor | null {
	if (!lastEventAnchorCapture) return null;
	if (lastEventAnchorCapture.eventID !== eventID) return null;
	if (Date.now() - lastEventAnchorCapture.capturedAt > 800) return null;
	return lastEventAnchorCapture.anchor;
}

export function calendarEventAnchorFromElement(element: HTMLElement): DraftPopoverAnchor {
	const rectangle = element.getBoundingClientRect();
	const titleRectangle = titleRectangleFromElement(element) ?? rectangle;
	return {
		clientX: rectangle.right,
		clientY: titleRectangle.top + titleRectangle.height / 2,
		leftClientX: rectangle.left,
		rightClientX: rectangle.right,
		topClientY: rectangle.top,
		bottomClientY: rectangle.bottom,
		titleEndClientX: titleRectangle.right,
		titleTopClientY: titleRectangle.top,
		titleBottomClientY: titleRectangle.bottom
	};
}

export function canonicalCalendarEventAnchorForID(stageElement: HTMLElement | null, eventID: string): DraftPopoverAnchor | null {
	const eventElement = canonicalCalendarEventElementForID(stageElement, eventID);
	return eventElement ? calendarEventAnchorFromElement(eventElement) : null;
}

function normalizedEventID(eventID: string | undefined): string | null {
	if (!eventID) return null;
	return eventID.split('::')[0] ?? null;
}

function anchorFromElement(element: HTMLElement): DraftPopoverAnchor {
	return calendarEventAnchorFromElement(element);
}

function titleRectangleFromElement(element: HTMLElement): DOMRect | null {
	if (element.classList.contains('draft-empty-title-event')) return null;
	const titleElement = element.querySelector<HTMLElement>(
		'.calendar-event-title, .calendar-month-event-title, .calendar-multi-day-all-day-proxy-start'
	);
	return titleElement?.getBoundingClientRect() ?? null;
}

function canonicalCalendarEventElementForID(stageElement: HTMLElement | null, eventID: string): HTMLElement | null {
	const visibleElements = calendarEventElementsByID(stageElement, eventID).filter(isVisibleCalendarEventElement);
	const titledElements = visibleElements.filter((element) => Boolean(titleRectangleFromElement(element)));
	const candidates = titledElements.length > 0 ? titledElements : visibleElements;
	return candidates.sort(compareElementsByPosition)[0] ?? null;
}

function compareElementsByPosition(firstElement: HTMLElement, secondElement: HTMLElement): number {
	const firstRectangle = firstElement.getBoundingClientRect();
	const secondRectangle = secondElement.getBoundingClientRect();
	return firstRectangle.top - secondRectangle.top || firstRectangle.left - secondRectangle.left;
}
