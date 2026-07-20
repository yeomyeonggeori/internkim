export type CalendarEventGesturePosition = {
	clientX: number;
	clientY: number;
};

export type CalendarEventGesturePoint = CalendarEventGesturePosition & {
	timestamp: number;
};

export type CalendarEventTap = {
	eventID: string;
	start: CalendarEventGesturePoint;
	end: CalendarEventGesturePoint;
};

const calendarEventMovementThresholdPx = 6;
const calendarEventTapDurationLimitMs = 450;

export function hasCalendarEventGestureMoved(
	start: CalendarEventGesturePosition,
	current: CalendarEventGesturePosition
): boolean {
	return gestureDistance(start, current) > calendarEventMovementThresholdPx;
}

export function isCalendarEventTap(tap: CalendarEventTap): boolean {
	if (tap.end.timestamp - tap.start.timestamp > calendarEventTapDurationLimitMs) return false;
	return !hasCalendarEventGestureMoved(tap.start, tap.end);
}

function gestureDistance(start: CalendarEventGesturePosition, current: CalendarEventGesturePosition): number {
	return Math.hypot(current.clientX - start.clientX, current.clientY - start.clientY);
}
