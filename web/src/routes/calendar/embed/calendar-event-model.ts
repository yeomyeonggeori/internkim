export type CalendarModelEvent = {
	id: string;
	title: string;
	description?: string;
	start: Date;
	end: Date;
	allDay?: boolean;
	calendarId?: string;
	meta?: Record<string, unknown>;
};

export type CalendarModelEventParams = Omit<CalendarModelEvent, 'start' | 'end'> & {
	start: Date;
	end: Date;
};

export function createCalendarModelEvent(params: CalendarModelEventParams): CalendarModelEvent {
	return {
		id: params.id,
		title: params.title,
		description: params.description ?? '',
		start: new Date(params.start),
		end: new Date(params.end),
		allDay: params.allDay ?? false,
		calendarId: params.calendarId ?? 'internkim',
		meta: params.meta ?? {}
	};
}
