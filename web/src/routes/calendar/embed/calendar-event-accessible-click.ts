type CalendarEventClick = Pick<MouseEvent, 'detail' | 'isTrusted'>;

export function isCalendarEventAccessibleClick(mouseEvent: CalendarEventClick): boolean {
	return mouseEvent.detail === 0 || !mouseEvent.isTrusted;
}
