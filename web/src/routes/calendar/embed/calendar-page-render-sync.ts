import type { Event as DayFlowEvent } from '@dayflow/core';
import { ViewType } from '@dayflow/svelte';
import {
	scheduleCalendarAllDayLayoutSync,
	scheduleCalendarMultiDayProxyLayoutSync
} from './calendar-embed-dom-sync';
import type { CalendarEventLoader } from './calendar-event-loader';
import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import { syncRemoteCalendarAndRefreshConflicts } from './calendar-remote-sync';

type CalendarPageRenderSyncContext = {
	broadcastCalendarEventsChanged: () => void;
	errorFallback: () => string;
	getCalendarEvents: () => DayFlowEvent[];
	getStageElement: () => HTMLElement | null;
	getToolbarDate: () => Date;
	getToolbarView: () => ViewType;
	isBrowser: () => boolean;
	loadCalendarConflicts: () => Promise<void>;
	openEventDetails: (eventID: string, anchor?: DraftPopoverAnchor | null) => void;
	refreshCurrentRange: CalendarEventLoader['refreshCurrentRange'];
};

export type CalendarPageRenderSyncActions = {
	refreshCalendar: () => Promise<void>;
	scheduleCalendarEventDOMSync: () => void;
	syncRemoteCalendarAndRefresh: () => Promise<void>;
};

export function createCalendarPageRenderSync(
	context: CalendarPageRenderSyncContext
): CalendarPageRenderSyncActions {
	async function refreshCalendar(): Promise<void> {
		await context.refreshCurrentRange();
	}

	function scheduleCalendarEventDOMSync(): void {
		if (!context.isBrowser()) return;
		scheduleCalendarMultiDayProxyLayoutSync(
			context.getStageElement(),
			context.getToolbarView(),
			context.getToolbarDate(),
			context.getCalendarEvents,
			context.openEventDetails
		);
		scheduleCalendarAllDayLayoutSync(context.getStageElement(), context.getToolbarView());
	}

	async function syncRemoteCalendarAndRefresh(): Promise<void> {
		await syncRemoteCalendarAndRefreshConflicts(
			context.errorFallback(),
			refreshCalendar,
			context.loadCalendarConflicts
		);
		context.broadcastCalendarEventsChanged();
	}

	return {
		refreshCalendar,
		scheduleCalendarEventDOMSync,
		syncRemoteCalendarAndRefresh
	};
}
