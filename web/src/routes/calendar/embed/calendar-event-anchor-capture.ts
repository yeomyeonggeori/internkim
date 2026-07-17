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

export function calendarEventAnchorFromElement(
	element: HTMLElement,
	originElement: HTMLElement = calendarEventFocusOrigin(element)
): DraftPopoverAnchor {
	const rectangle = element.getBoundingClientRect();
	const anchorRectangle = anchorRectangleFromElement(element) ?? rectangle;
	const titleEndClientX = shouldUseTitleEndClientX(element) ? anchorRectangle.right : rectangle.right;
	return {
		clientX: rectangle.right,
		clientY: anchorRectangle.top + anchorRectangle.height / 2,
		originElement,
		leftClientX: rectangle.left,
		rightClientX: rectangle.right,
		topClientY: rectangle.top,
		bottomClientY: rectangle.bottom,
		titleEndClientX,
		titleTopClientY: anchorRectangle.top,
		titleBottomClientY: anchorRectangle.bottom,
		preferredSide: preferredPopoverSideFromElement(element)
	};
}

function calendarEventFocusOrigin(element: HTMLElement): HTMLElement {
	return element.querySelector<HTMLElement>('.calendar-dayflow-event-activator') ?? element;
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

function anchorRectangleFromElement(element: HTMLElement): DOMRect | null {
	const timeElement = shouldUseTimeEndClientX(element) ? element.querySelector<HTMLElement>('.calendar-event-time') : null;
	return timeElement?.getBoundingClientRect() ?? titleRectangleFromElement(element);
}

function titleRectangleFromElement(element: HTMLElement): DOMRect | null {
	if (element.classList.contains('draft-empty-title-event')) return null;
	const titleElement = element.querySelector<HTMLElement>(
		'.calendar-event-title, .calendar-month-event-title, .calendar-multi-day-all-day-proxy-start'
	);
	return titleElement?.getBoundingClientRect() ?? null;
}

function shouldUseTimeEndClientX(element: HTMLElement): boolean {
	return element.classList.contains('df-day-event') && !element.classList.contains('df-right-panel-event-card');
}

function shouldUseTitleEndClientX(element: HTMLElement): boolean {
	return !element.classList.contains('df-week-event');
}

function preferredPopoverSideFromElement(element: HTMLElement): DraftPopoverAnchor['preferredSide'] {
	if (element.classList.contains('df-right-panel-event-card') || element.closest('.df-right-panel-events')) return 'left';
	return undefined;
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
