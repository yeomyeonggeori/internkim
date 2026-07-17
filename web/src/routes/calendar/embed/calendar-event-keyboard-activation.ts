import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import { calendarEventAnchorFromElement } from './calendar-event-anchor-capture';
import {
	calendarEventElementFromTarget,
	isCalendarMonthEventLayerElement,
	normalizedCalendarEventID
} from './calendar-event-elements';

export type CalendarEventKeyboardActivationOptions = {
	stageElement: HTMLElement;
	openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
};

export function installCalendarEventKeyboardActivation(
	options: CalendarEventKeyboardActivationOptions
): () => void {
	const handleKeydown = (event: KeyboardEvent): void => {
		if (!isCalendarEventActivationKey(event) || event.repeat || event.altKey || event.ctrlKey || event.metaKey) {
			return;
		}
		const eventElement = keyboardEventElementFromTarget(options.stageElement, event.target);
		const eventID = normalizedCalendarEventID(eventElement?.dataset.eventId);
		if (!eventElement || !eventID) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
		options.openEvent(eventID, calendarEventAnchorFromElement(eventElement));
	};

	options.stageElement.addEventListener('keydown', handleKeydown, true);
	return () => {
		options.stageElement.removeEventListener('keydown', handleKeydown, true);
	};
}

function isCalendarEventActivationKey(event: KeyboardEvent): boolean {
	return event.key === 'Enter' || event.key === ' ';
}

function keyboardEventElementFromTarget(stageElement: HTMLElement, target: EventTarget | null): HTMLElement | null {
	const eventElement = calendarEventElementFromTarget(stageElement, target);
	if (eventElement && !isCalendarMonthEventLayerElement(eventElement)) return eventElement;
	return null;
}
