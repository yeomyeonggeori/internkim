import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import { calendarEventAnchorFromElement } from './calendar-event-anchor-capture';
import { isCalendarEventAccessibleClick } from './calendar-event-accessible-click';
import { calendarEventElementFromTarget, normalizedCalendarEventID } from './calendar-event-elements';

const calendarAccessibleEventActivatorSelector =
	'.calendar-dayflow-event-activator, .calendar-multi-day-all-day-proxy';

type CalendarDayFlowEventActivationOptions = {
	stageElement: HTMLElement;
	openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
};

export function installCalendarDayFlowEventActivation(
	options: CalendarDayFlowEventActivationOptions
): () => void {
	const handleClick = (event: MouseEvent): void => {
		if (!isCalendarEventAccessibleClick(event)) return;
		if (!(event.target instanceof Element)) return;
		const activator = event.target.closest<HTMLElement>(calendarAccessibleEventActivatorSelector);
		if (!activator || !options.stageElement.contains(activator)) return;
		const eventElement = calendarEventElementFromTarget(options.stageElement, event.target);
		const eventID = normalizedCalendarEventID(eventElement?.dataset.eventId);
		if (!eventElement || !eventID) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
		options.openEvent(eventID, calendarEventAnchorFromElement(eventElement));
	};

	options.stageElement.addEventListener('click', handleClick, true);
	return () => {
		options.stageElement.removeEventListener('click', handleClick, true);
	};
}
