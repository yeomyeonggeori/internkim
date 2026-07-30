import type { Event as DayFlowEvent } from '@dayflow/core';
import { ViewType } from '@dayflow/svelte';
import type { CalendarEventLoader } from './calendar-event-loader';
import { syncRemoteCalendarAndRefreshConflicts } from './calendar-remote-sync';

type CalendarPageRenderSyncContext = {
	broadcastCalendarEventsChanged: () => void;
	errorFallback: () => string;
	getCalendarEvents: () => DayFlowEvent[];
	getStageElement: () => HTMLElement | null;
	getSelectedEventID: () => string | null;
	getToolbarDate: () => Date;
	getToolbarView: () => ViewType;
	isBrowser: () => boolean;
	loadCalendarConflicts: () => Promise<void>;
	refreshCurrentRange: CalendarEventLoader['refreshCurrentRange'];
};

export type CalendarPageRenderSyncActions = {
	refreshCalendar: () => Promise<void>;
	syncRemoteCalendarAndRefresh: () => Promise<void>;
};

export function createCalendarPageRenderSync(
	context: CalendarPageRenderSyncContext
): CalendarPageRenderSyncActions {
	async function refreshCalendar(): Promise<void> {
		await context.refreshCurrentRange();
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
		syncRemoteCalendarAndRefresh
	};
}
