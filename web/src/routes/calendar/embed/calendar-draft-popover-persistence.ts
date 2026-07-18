import { createEvent, type Event as DayFlowEvent } from '@dayflow/core';
import type { CalendarEventActions } from './calendar-event-actions';
import {
	draftPopoverChanges,
	hasDraftPopoverEventChanges,
	isDraftPopoverValid,
	type DraftPopoverState
} from './calendar-draft-popover-state';

type CalendarDraftPopoverPersistenceContext = {
	eventActions: CalendarEventActions;
	getCalendarEvents: () => DayFlowEvent[];
	getDraftPopover: () => DraftPopoverState | null;
	replaceLocalEvent: (event: DayFlowEvent) => void;
	setDraftPopover: (popover: DraftPopoverState | null) => void;
};

export type CalendarDraftPopoverPersistenceActions = {
	cancelDraftPopover: () => Promise<void>;
	deleteDraftPopover: () => Promise<void>;
	saveDraftPopover: () => Promise<void>;
};

export function createCalendarDraftPopoverPersistence(
	context: CalendarDraftPopoverPersistenceContext
): CalendarDraftPopoverPersistenceActions {
	async function saveDraftPopover(): Promise<void> {
		const popover = context.getDraftPopover();
		if (!popover || !isDraftPopoverValid(popover)) return;
		const event = context.getCalendarEvents().find((calendarEvent) => calendarEvent.id === popover.eventID);
		if (!event) return;
		if (popover.mode === 'edit' && !hasDraftPopoverEventChanges(popover, event)) {
			context.setDraftPopover(null);
			return;
		}
		const changes = draftPopoverChanges(popover);
		const updatedEvent = createEvent({
			id: event.id,
			title: changes.title,
			description: changes.description,
			start: changes.start,
			end: changes.end,
			allDay: changes.allDay,
			calendarId: changes.calendarId,
			meta: { ...(event.meta ?? {}), ...changes.meta }
		});
		context.replaceLocalEvent(updatedEvent);
		context.setDraftPopover(null);
		await context.eventActions.saveUpdatedEvent(updatedEvent, event);
	}

	async function cancelDraftPopover(): Promise<void> {
		const popover = context.getDraftPopover();
		if (!popover) return;
		context.setDraftPopover(null);
		if (popover.mode === 'create') await context.eventActions.deleteEvent(popover.eventID);
	}

	async function deleteDraftPopover(): Promise<void> {
		const popover = context.getDraftPopover();
		if (!popover || popover.mode !== 'edit') return;
		context.setDraftPopover(null);
		await context.eventActions.deleteEvent(popover.eventID);
	}

	return {
		cancelDraftPopover,
		deleteDraftPopover,
		saveDraftPopover
	};
}
