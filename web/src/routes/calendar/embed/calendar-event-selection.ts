import { clearFocusedCalendarEventElements } from './calendar-event-elements';

export type CalendarEventSelectionOptions = {
	stageElement: HTMLElement;
	clearSelectedEvent: () => void;
	selectEvent: (eventID: string) => void;
};

export function installCalendarEventSelection(options: CalendarEventSelectionOptions): () => void {
	const handlePointerDown = (event: PointerEvent): void => {
		if (!(event.target instanceof Element)) return;
		if (event.target.closest('.calendar-draft-popover')) return;
		const eventElement = event.target.closest<HTMLElement>(
			'[data-event-id].df-event, [data-event-id].df-month-segment-event, .calendar-multi-day-all-day-proxy[data-event-id]'
		);
		const eventID = normalizedEventID(eventElement?.dataset.eventId);
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

function normalizedEventID(eventID: string | undefined): string | null {
	if (!eventID) return null;
	return eventID.split('::')[0] ?? null;
}
