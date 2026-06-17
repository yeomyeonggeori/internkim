import { clearFocusedCalendarEventElements } from './calendar-event-elements';

export type CalendarEventSelectionOptions = {
	stageElement: HTMLElement;
	clearSelectedEvent: () => void;
};

export function installCalendarEventSelection(options: CalendarEventSelectionOptions): () => void {
	const handlePointerDown = (event: PointerEvent): void => {
		if (!(event.target instanceof Element)) return;
		if (event.target.closest('.calendar-draft-popover')) return;
		if (event.target.closest('.df-event, .df-month-segment-event, .calendar-multi-day-all-day-proxy')) return;
		options.clearSelectedEvent();
		clearFocusedCalendarEventElements(options.stageElement);
	};

	document.addEventListener('pointerdown', handlePointerDown, true);

	return () => {
		document.removeEventListener('pointerdown', handlePointerDown, true);
	};
}
