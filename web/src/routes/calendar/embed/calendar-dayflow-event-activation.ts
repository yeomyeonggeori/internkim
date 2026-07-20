import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import { createCalendarEventActivationGuard } from './calendar-event-activation-guard';
import { calendarEventAnchorFromElement } from './calendar-event-anchor-capture';
import {
	calendarEventElementFromTarget,
	isCalendarMonthEventLayerElement,
	normalizedCalendarEventID
} from './calendar-event-elements';
import { installCalendarEventTouchActivation } from './calendar-event-touch-activation';

type CalendarDayFlowEventActivationOptions = {
	stageElement: HTMLElement;
	openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
};

export function installCalendarDayFlowEventActivation(
	options: CalendarDayFlowEventActivationOptions
): () => void {
	const activationGuard = createCalendarEventActivationGuard(options.stageElement);
	const handleClick = (event: MouseEvent): void => {
		const eventElement = calendarEventElementFromTarget(options.stageElement, event.target);
		const eventID = normalizedCalendarEventID(eventElement?.dataset.eventId);
		if (!eventElement || isCalendarMonthEventLayerElement(eventElement) || !eventID) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
		if (activationGuard.shouldSuppressActivation(event)) return;
		options.openEvent(eventID, calendarEventAnchorFromElement(eventElement));
	};

	options.stageElement.addEventListener('click', handleClick, true);
	const stopTouchActivation = installCalendarEventTouchActivation(options);
	return () => {
		options.stageElement.removeEventListener('click', handleClick, true);
		stopTouchActivation();
		activationGuard.destroy();
	};
}
