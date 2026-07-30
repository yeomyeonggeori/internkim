import type { Event as CalendarStoreEvent } from '@dayflow/core';

export type CalendarEventStore = {
	readonly events: CalendarStoreEvent[];
	getAllEvents: () => CalendarStoreEvent[];
	addEvent: (event: CalendarStoreEvent) => void;
	updateEvent: (eventID: string, changes: Partial<CalendarStoreEvent>) => void;
	applyEventsChanges: (changes: { delete: string[]; add: CalendarStoreEvent[] }) => void;
};

export function createCalendarEventStore(): CalendarEventStore {
	let events = $state<CalendarStoreEvent[]>([]);

	function applyEventsChanges(changes: { delete: string[]; add: CalendarStoreEvent[] }): void {
		const deletedEventIDs = new Set(changes.delete);
		const addedEventIDs = new Set(changes.add.map((event) => event.id));
		events = [
			...events.filter((event) => !deletedEventIDs.has(event.id) && !addedEventIDs.has(event.id)),
			...changes.add
		];
	}

	return {
		get events() {
			return events;
		},
		getAllEvents: () => events,
		addEvent: (event) => applyEventsChanges({ delete: [], add: [event] }),
		updateEvent: (eventID, changes) => {
			events = events.map((event) => (event.id === eventID ? { ...event, ...changes } : event));
		},
		applyEventsChanges
	};
}
