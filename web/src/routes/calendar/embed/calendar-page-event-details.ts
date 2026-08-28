import type { CalendarModelEvent as DayTaskEvent } from './calendar-event-model';
import type { CalendarDraftPopoverActions } from './calendar-draft-popover-actions';
import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import { findCalendarEventByID } from './calendar-event-lookup';

type CalendarPageEventDetailsContext = {
	getAppEvents: () => DayTaskEvent[];
	getFallbackEvents: () => DayTaskEvent[];
	openEventDraftPopover: CalendarDraftPopoverActions['openEventDraftPopover'];
};

export type CalendarPageEventDetailsActions = {
	openEventDetails: (eventID: string, anchor?: DraftPopoverAnchor | null) => void;
};

export function createCalendarPageEventDetails(
	context: CalendarPageEventDetailsContext
): CalendarPageEventDetailsActions {
	function openEventDetails(eventID: string, anchor: DraftPopoverAnchor | null = null): void {
		const event = findCalendarEventByID(context.getAppEvents(), context.getFallbackEvents(), eventID);
		if (!event || event.calendarId === 'holidays' || event.meta?.readOnly === true) return;
		context.openEventDraftPopover(event, 'edit', anchor);
	}

	return { openEventDetails };
}
