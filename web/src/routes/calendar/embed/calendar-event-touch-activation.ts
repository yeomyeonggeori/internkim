import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import { calendarEventAnchorFromElement } from './calendar-event-anchor-capture';
import { calendarEventElementFromTarget, normalizedCalendarEventID } from './calendar-event-elements';
import {
	hasCalendarEventGestureMoved,
	isCalendarEventDoubleTap,
	isCalendarEventTap,
	type CalendarEventGesturePoint,
	type CalendarEventTap
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
	let previousTap: CalendarEventTap | null = null;

	const handleTouchStart = (event: TouchEvent): void => {
		const eventElement = calendarEventElementFromTarget(options.stageElement, event.target);
		const eventID = normalizedCalendarEventID(eventElement?.dataset.eventId);
		const touch = event.touches.item(0);
		if (!eventElement || !eventID || !touch || event.touches.length !== 1) {
			activeTouch = null;
			previousTap = null;
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
			previousTap = null;
		}
	};
	const handleTouchEnd = (event: TouchEvent): void => {
		const completedTouch = activeTouch;
		activeTouch = null;
		if (!completedTouch) return;
		const touch = event.changedTouches.item(0);
		if (!touch || event.changedTouches.length !== 1) {
			previousTap = null;
			return;
		}
		const completedTap: CalendarEventTap = {
			eventID: completedTouch.eventID,
			start: completedTouch.start,
			end: {
				clientX: touch.clientX,
				clientY: touch.clientY,
				timestamp: event.timeStamp
			}
		};
		if (!isCalendarEventTap(completedTap)) {
			previousTap = null;
			return;
		}
		if (!isCalendarEventDoubleTap(previousTap, completedTap)) {
			previousTap = completedTap;
			return;
		}
		previousTap = null;
		options.openEvent(completedTap.eventID, calendarEventAnchorFromElement(completedTouch.eventElement));
	};
	const clearTouchState = (): void => {
		activeTouch = null;
		previousTap = null;
	};

	options.stageElement.addEventListener('touchstart', handleTouchStart, { capture: true, passive: true });
	options.stageElement.addEventListener('touchmove', handleTouchMove, { capture: true, passive: true });
	options.stageElement.addEventListener('touchend', handleTouchEnd, { capture: true, passive: true });
	options.stageElement.addEventListener('touchcancel', clearTouchState, { capture: true, passive: true });
	return () => {
		options.stageElement.removeEventListener('touchstart', handleTouchStart, true);
		options.stageElement.removeEventListener('touchmove', handleTouchMove, true);
		options.stageElement.removeEventListener('touchend', handleTouchEnd, true);
		options.stageElement.removeEventListener('touchcancel', clearTouchState, true);
	};
}
