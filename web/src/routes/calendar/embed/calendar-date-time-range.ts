export type CalendarDateTimeRangeFields = {
	startDateKey: string;
	endDateKey: string;
	startTime: string;
	endTime: string;
	allDay: boolean;
};

export type CalendarDateTimeRangeStartInput = {
	startDateKey?: string;
	startTime?: string;
};

export function calendarDateTimeRangeChangesForStart(
	fields: CalendarDateTimeRangeFields,
	input: CalendarDateTimeRangeStartInput
): CalendarDateTimeRangeFields {
	const durationMilliseconds = calendarDateTimeRangeDurationMilliseconds(fields);
	const startDateKey = input.startDateKey ?? fields.startDateKey;
	const startTime = input.startTime ?? fields.startTime;
	const start = calendarDateTimeRangeDateTime(startDateKey, fields.allDay ? '00:00' : startTime);
	const end = new Date(start.getTime() + durationMilliseconds);
	return {
		startDateKey,
		startTime,
		endDateKey: calendarDateTimeRangeDateKey(end),
		endTime: calendarDateTimeRangeTimeValue(end),
		allDay: fields.allDay
	};
}

function calendarDateTimeRangeDurationMilliseconds(fields: CalendarDateTimeRangeFields): number {
	const start = calendarDateTimeRangeDateTime(fields.startDateKey, fields.allDay ? '00:00' : fields.startTime);
	const end = calendarDateTimeRangeDateTime(fields.endDateKey, fields.allDay ? '00:00' : fields.endTime);
	return Math.max(0, end.getTime() - start.getTime());
}

function calendarDateTimeRangeDateTime(selectedDateKey: string, time: string): Date {
	const [year = '1970', month = '1', day = '1'] = selectedDateKey.split('-');
	const [hour = '0', minute = '0'] = time.split(':');
	return new Date(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute), 0, 0);
}

function calendarDateTimeRangeDateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

function calendarDateTimeRangeTimeValue(date: Date): string {
	return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}
