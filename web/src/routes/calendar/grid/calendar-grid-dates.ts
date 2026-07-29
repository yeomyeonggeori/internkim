export type CalendarGridWeek = {
	startDateKey: string;
	days: Date[];
};

export function calendarGridDateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

export function calendarGridDateFromKey(dateKey: string): Date {
	const [year = '1970', month = '1', day = '1'] = dateKey.split('-');
	return new Date(Number(year), Number(month) - 1, Number(day), 0, 0, 0, 0);
}

export function startOfCalendarGridDay(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate(), 0, 0, 0, 0);
}

export function startOfCalendarGridWeek(date: Date, firstWeekday = 0): Date {
	const startOfDay = startOfCalendarGridDay(date);
	const weekdayOffset = (startOfDay.getDay() - firstWeekday + 7) % 7;
	return addCalendarGridDays(startOfDay, -weekdayOffset);
}

export function addCalendarGridDays(date: Date, dayCount: number): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate() + dayCount, 0, 0, 0, 0);
}

export function isSameCalendarGridDay(first: Date, second: Date): boolean {
	return calendarGridDateKey(first) === calendarGridDateKey(second);
}

export function calendarGridWeek(weekStart: Date): CalendarGridWeek {
	return {
		startDateKey: calendarGridDateKey(weekStart),
		days: Array.from({ length: 7 }, (_, dayOffset) => addCalendarGridDays(weekStart, dayOffset))
	};
}

export function calendarGridWeeks(anchorDate: Date, weeksBefore: number, weeksAfter: number, firstWeekday = 0): CalendarGridWeek[] {
	const anchorWeekStart = startOfCalendarGridWeek(anchorDate, firstWeekday);
	const weekCount = weeksBefore + weeksAfter + 1;
	return Array.from({ length: weekCount }, (_, weekIndex) =>
		calendarGridWeek(addCalendarGridDays(anchorWeekStart, (weekIndex - weeksBefore) * 7))
	);
}

export function calendarGridDominantMonth(week: CalendarGridWeek): Date {
	const monthCounts = new Map<string, { count: number; date: Date }>();
	for (const day of week.days) {
		const monthKey = `${day.getFullYear()}-${day.getMonth()}`;
		const entry = monthCounts.get(monthKey);
		if (entry) {
			entry.count += 1;
			continue;
		}
		monthCounts.set(monthKey, { count: 1, date: new Date(day.getFullYear(), day.getMonth(), 1) });
	}
	return [...monthCounts.values()].sort((first, second) => second.count - first.count)[0].date;
}

export function calendarGridMinutesFromMidnight(date: Date): number {
	return date.getHours() * 60 + date.getMinutes();
}

export function calendarGridDateAtMinutes(day: Date, minutes: number): Date {
	return new Date(day.getFullYear(), day.getMonth(), day.getDate(), 0, minutes, 0, 0);
}
