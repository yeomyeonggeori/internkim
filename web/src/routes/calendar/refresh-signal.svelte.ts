export const calendarRefresh = $state({ ticks: 0 });
export const calendarVisibility = $state({ work: true });

export function bumpCalendarRefresh() {
	calendarRefresh.ticks += 1;
}
