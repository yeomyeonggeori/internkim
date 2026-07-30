import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import {
	dayFlowEventFromCalendarEvent,
	dayFlowEventFromCalendarHoliday,
	eventEndDate,
	eventStartDate
} from './calendar-event-mapping';
import { fetchCalendarEvents } from './calendar-event-persistence';
import {
	fetchCalendarHolidays,
	type CalendarHolidayLocale
} from './calendar-holiday-persistence';

type CalendarEventLoaderContext = {
	isBrowser: () => boolean;
	getLocale: () => CalendarHolidayLocale;
	errorFallback: () => string;
	getCalendarEvents: () => DayFlowEvent[];
	getVisibleEvents: () => DayFlowEvent[];
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
	invalidatePendingLoad: () => void;
	loadEvents: (startDate: Date, endDate: Date) => Promise<void>;
	refreshCurrentRange: () => Promise<void>;
	renderVisibleEvents: (events: DayFlowEvent[]) => void;
};

type CalendarEventLoaderDependencies = {
	fetchEvents: typeof fetchCalendarEvents;
	fetchHolidays: typeof fetchCalendarHolidays;
};

export function createCalendarEventLoader(
	context: CalendarEventLoaderContext,
	dependencies: Partial<CalendarEventLoaderDependencies> = {}
): CalendarEventLoader {
	let visibleRange: { startDate: Date; endDate: Date } | null = null;
	let loadEventsRequestID = 0;
	let isLoadPending = false;
	const fetchEvents = dependencies.fetchEvents ?? fetchCalendarEvents;
	const fetchHolidays = dependencies.fetchHolidays ?? fetchCalendarHolidays;

	function hasVisibleRange(): boolean {
		return visibleRange !== null;
	}

	async function loadEvents(startDate: Date, endDate: Date): Promise<void> {
		if (!context.isBrowser()) return;
		const requestID = (loadEventsRequestID += 1);
		isLoadPending = true;
		visibleRange = { startDate, endDate };
		context.setIsLoading(true);
		context.setErrorMessage('');
		try {
			const [calendarEvents, holidayResult] = await Promise.all([
				fetchEvents(startDate, endDate, context.errorFallback()),
				fetchHolidays(startDate, endDate, context.getLocale(), context.errorFallback())
					.then((holidays) => ({ holidays, error: null }))
					.catch((error: unknown) => ({ holidays: [], error }))
			]);
			if (requestID !== loadEventsRequestID) return;
			const events = mergePreservedLocalEvents([
				...calendarEvents.map(dayFlowEventFromCalendarEvent),
				...holidayResult.holidays.map(dayFlowEventFromCalendarHoliday)
			]);
			const mergedEvents = eventsOutsideRange(context.getVisibleEvents(), startDate, endDate).concat(events);
			context.setVisibleEvents(mergedEvents);
			context.setEventCount(mergedEvents.length);
			replaceCalendarEvents(mergedEvents);
			context.afterRenderEvents?.(events);
			if (holidayResult.error) {
				context.setErrorMessage(
					holidayResult.error instanceof Error ? holidayResult.error.message : context.errorFallback()
				);
			}
		} catch (error) {
			if (requestID !== loadEventsRequestID) return;
			context.setVisibleEvents([]);
			context.setErrorMessage(error instanceof Error ? error.message : context.errorFallback());
		} finally {
			if (requestID !== loadEventsRequestID) return;
			isLoadPending = false;
			context.setIsLoading(false);
		}
	}

	function invalidatePendingLoad(): void {
		if (!isLoadPending) return;
		loadEventsRequestID += 1;
		isLoadPending = false;
		context.setIsLoading(false);
	}

	async function refreshCurrentRange(): Promise<void> {
		if (!visibleRange) return;
		await loadEvents(visibleRange.startDate, visibleRange.endDate);
	}

	function renderVisibleEvents(events: DayFlowEvent[]): void {
		if (!visibleRange) return;
		replaceCalendarEvents(events);
	}

	function replaceCalendarEvents(events: DayFlowEvent[]): void {
		const eventIDs = context.getCalendarEvents().map((event) => event.id);
		context.applyCalendarEventsChanges({
			delete: eventIDs,
			add: events
		});
		context.triggerCalendarRender();
		context.refreshSelectedMonthDateCell();
	}

	function eventsOutsideRange(events: DayFlowEvent[], startDate: Date, endDate: Date): DayFlowEvent[] {
		return events.filter((event) => eventEndDate(event) <= startDate || eventStartDate(event) >= endDate);
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
		invalidatePendingLoad,
		loadEvents,
		refreshCurrentRange,
		renderVisibleEvents
	};
}
