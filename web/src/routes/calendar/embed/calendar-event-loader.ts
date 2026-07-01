import { recalculateEventDays, type Event as DayFlowEvent } from '@dayflow/core';
import { dayFlowEventFromCalendarEvent } from './calendar-event-mapping';
import { fetchCalendarEvents } from './calendar-event-persistence';

type CalendarEventLoaderContext = {
	isBrowser: () => boolean;
	errorFallback: () => string;
	getCalendarEvents: () => DayFlowEvent[];
	applyCalendarEventsChanges: (changes: { delete: string[]; add: DayFlowEvent[] }) => void;
	triggerCalendarRender: () => void;
	setVisibleEvents: (events: DayFlowEvent[]) => void;
	setEventCount: (eventCount: number) => void;
	setIsLoading: (isLoading: boolean) => void;
	setErrorMessage: (message: string) => void;
	refreshSelectedMonthDateCell: () => void;
	preservedLocalEvents?: () => DayFlowEvent[];
	shouldPreserveLocalEvent?: (event: DayFlowEvent) => boolean;
	afterRenderEvents?: (events: DayFlowEvent[]) => void;
};

export type CalendarEventLoader = {
	hasVisibleRange: () => boolean;
	loadEvents: (startDate: Date, endDate: Date) => Promise<void>;
	refreshCurrentRange: () => Promise<void>;
	renderVisibleEvents: (events: DayFlowEvent[]) => void;
};

export function createCalendarEventLoader(context: CalendarEventLoaderContext): CalendarEventLoader {
	let visibleRange: { startDate: Date; endDate: Date } | null = null;
	let loadEventsRequestID = 0;

	function hasVisibleRange(): boolean {
		return visibleRange !== null;
	}

	async function loadEvents(startDate: Date, endDate: Date): Promise<void> {
		if (!context.isBrowser()) return;
		const requestID = (loadEventsRequestID += 1);
		visibleRange = { startDate, endDate };
		context.setIsLoading(true);
		context.setErrorMessage('');
		try {
			const calendarEvents = await fetchCalendarEvents(startDate, endDate, context.errorFallback());
			if (requestID !== loadEventsRequestID) return;
			const events = mergePreservedLocalEvents(calendarEvents.map(dayFlowEventFromCalendarEvent));
			context.setVisibleEvents(events);
			context.setEventCount(events.length);
			replaceCalendarEvents(events, startDate);
			context.afterRenderEvents?.(events);
		} catch (error) {
			if (requestID !== loadEventsRequestID) return;
			context.setVisibleEvents([]);
			context.setErrorMessage(error instanceof Error ? error.message : context.errorFallback());
		} finally {
			if (requestID === loadEventsRequestID) context.setIsLoading(false);
		}
	}

	async function refreshCurrentRange(): Promise<void> {
		if (!visibleRange) return;
		await loadEvents(visibleRange.startDate, visibleRange.endDate);
	}

	function renderVisibleEvents(events: DayFlowEvent[]): void {
		if (!visibleRange) return;
		replaceCalendarEvents(events, visibleRange.startDate);
	}

	function replaceCalendarEvents(events: DayFlowEvent[], visibleRangeStartDate: Date): void {
		const eventIDs = context.getCalendarEvents().map((event) => event.id);
		context.applyCalendarEventsChanges({
			delete: eventIDs,
			add: recalculateEventDays(events, visibleRangeStartDate)
		});
		context.triggerCalendarRender();
		context.refreshSelectedMonthDateCell();
	}

	function mergePreservedLocalEvents(events: DayFlowEvent[]): DayFlowEvent[] {
		const preservedEvents = [
			...(context.preservedLocalEvents?.() ?? []),
			...context.getCalendarEvents().filter((event) => context.shouldPreserveLocalEvent?.(event) ?? false)
		];
		if (preservedEvents.length === 0) return events;
		const remoteEventIDs = new Set(events.map((event) => event.id));
		const uniquePreservedEvents = preservedEvents.filter((event) => !remoteEventIDs.has(event.id));
		return [...events, ...uniqueEventsByID(uniquePreservedEvents)];
	}

	function uniqueEventsByID(events: DayFlowEvent[]): DayFlowEvent[] {
		const eventsByID = new Map(events.map((event) => [event.id, event]));
		return Array.from(eventsByID.values());
	}

	return {
		hasVisibleRange,
		loadEvents,
		refreshCurrentRange,
		renderVisibleEvents
	};
}
