import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import { calendarEventAnchorFromElement } from './calendar-event-anchor-capture';
import {
	calendarEventElementFromTarget,
	isCalendarMonthEventLayerElement,
	normalizedCalendarEventID
} from './calendar-event-elements';
import {
	hasCalendarEventGestureMoved,
	isCalendarEventTap,
	type CalendarEventGesturePoint
} from './calendar-event-gesture';

type CalendarEventTouchActivationOptions = {
	stageElement: HTMLElement;
	openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
};

type ActiveTouch = {
	eventElement: HTMLElement;
	eventID: string;
	start: CalendarEventGesturePoint;
};

export function installCalendarEventTouchActivation(options: CalendarEventTouchActivationOptions): () => void {
	let activeTouch: ActiveTouch | null = null;

	const handleTouchStart = (event: TouchEvent): void => {
		const eventElement = calendarEventElementFromTarget(options.stageElement, event.target);
		const eventID = normalizedCalendarEventID(eventElement?.dataset.eventId);
		const touch = event.touches.item(0);
		if (
			!eventElement ||
			isCalendarMonthEventLayerElement(eventElement) ||
			!eventID ||
			!touch ||
			event.touches.length !== 1
		) {
			activeTouch = null;
			return;
		}
		activeTouch = {
			eventElement,
			eventID,
			start: {
				clientX: touch.clientX,
				clientY: touch.clientY,
				timestamp: event.timeStamp
			}
		};
	};

	const handleTouchMove = (event: TouchEvent): void => {
		if (!activeTouch) return;
		const touch = event.touches.item(0);
		if (!touch || event.touches.length !== 1 || hasCalendarEventGestureMoved(activeTouch.start, touch)) {
			activeTouch = null;
		}
	};

	const handleTouchEnd = (event: TouchEvent): void => {
		const completedTouch = activeTouch;
		activeTouch = null;
		if (!completedTouch) return;
		const touch = event.changedTouches.item(0);
		if (!touch || event.changedTouches.length !== 1) return;
		const isTap = isCalendarEventTap({
			eventID: completedTouch.eventID,
			start: completedTouch.start,
			end: {
				clientX: touch.clientX,
				clientY: touch.clientY,
				timestamp: event.timeStamp
			}
		});
		if (!isTap) return;
		event.preventDefault();
		options.openEvent(completedTouch.eventID, calendarEventAnchorFromElement(completedTouch.eventElement));
	};

	const clearTouch = (): void => {
		activeTouch = null;
	};

	options.stageElement.addEventListener('touchstart', handleTouchStart, { capture: true, passive: true });
	options.stageElement.addEventListener('touchmove', handleTouchMove, { capture: true, passive: true });
	options.stageElement.addEventListener('touchend', handleTouchEnd, { capture: true, passive: false });
	options.stageElement.addEventListener('touchcancel', clearTouch, { capture: true, passive: true });
	return () => {
		options.stageElement.removeEventListener('touchstart', handleTouchStart, true);
		options.stageElement.removeEventListener('touchmove', handleTouchMove, true);
		options.stageElement.removeEventListener('touchend', handleTouchEnd, true);
		options.stageElement.removeEventListener('touchcancel', clearTouch, true);
	};
}
