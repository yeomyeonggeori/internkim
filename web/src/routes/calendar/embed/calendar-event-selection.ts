import {
	calendarEventElementFromTarget,
	clearFocusedCalendarEventElements,
	normalizedCalendarEventID
} from './calendar-event-elements';

export type CalendarEventSelectionOptions = {
	stageElement: HTMLElement;
	clearSelectedEvent: () => void;
	selectEvent: (eventID: string) => void;
};

export function installCalendarEventSelection(options: CalendarEventSelectionOptions): () => void {
	const handlePointerDown = (event: PointerEvent): void => {
		if (
			event.target instanceof Element &&
			event.target.closest('.calendar-draft-popover, .calendar-mobile-event-editor')
		) {
			return;
		}
		const eventElement = calendarEventElementFromTarget(options.stageElement, event.target);
		const eventID = normalizedCalendarEventID(eventElement?.dataset.eventId);
		if (eventID) {
			options.selectEvent(eventID);
			return;
		}
		options.clearSelectedEvent();
		clearFocusedCalendarEventElements(options.stageElement);
	};

	document.addEventListener('pointerdown', handlePointerDown, true);

	return () => {
		document.removeEventListener('pointerdown', handlePointerDown, true);
	};
}
