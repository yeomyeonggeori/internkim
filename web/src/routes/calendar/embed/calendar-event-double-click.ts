import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import { createCalendarEventActivationGuard } from './calendar-event-activation-guard';
import { calendarEventAnchorFromElement } from './calendar-event-anchor-capture';

export type CalendarEventDoubleClickOptions = {
	stageElement: HTMLElement;
	openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
};

export function installCalendarEventDoubleClick(options: CalendarEventDoubleClickOptions): () => void {
	const activationGuard = createCalendarEventActivationGuard(options.stageElement);

	const handleDoubleClick = (event: MouseEvent): void => {
		if (!(event.target instanceof Element)) return;
		const eventElement = editableEventElementFromTarget(options.stageElement, event.target);
		if (!eventElement) return;
		const eventID = normalizedEventID(eventElement.dataset.eventId);
		if (!eventID) return;
		if (activationGuard.shouldSuppressActivation(event)) {
			event.preventDefault();
			event.stopPropagation();
			event.stopImmediatePropagation();
			return;
		}
		event.preventDefault();
		event.stopPropagation();
		options.openEvent(eventID, calendarEventAnchorFromElement(eventElement));
	};

	options.stageElement.addEventListener('dblclick', handleDoubleClick, true);

	return () => {
		options.stageElement.removeEventListener('dblclick', handleDoubleClick, true);
		activationGuard.destroy();
	};
}

function editableEventElementFromTarget(stageElement: HTMLElement, target: Element): HTMLElement | null {
	const eventElement = target.closest<HTMLElement>(
		'[data-event-id].df-event, .calendar-multi-day-all-day-proxy[data-event-id]'
	);
	if (!eventElement || !stageElement.contains(eventElement)) return null;
	if (eventElement.classList.contains('calendar-month-direct-event')) return null;
	return eventElement;
}

function normalizedEventID(eventID: string | undefined): string | null {
	if (!eventID) return null;
	return eventID.split('::')[0] ?? null;
}
