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
const calendarEventDoubleTapDistanceThresholdPx = 24;
const calendarEventTapDurationLimitMs = 450;

export function hasCalendarEventGestureMoved(
	start: CalendarEventGesturePosition,
	current: CalendarEventGesturePosition
): boolean {
	return gestureDistance(start, current) > calendarEventMovementThresholdPx;
}

export function isCalendarEventDoubleTap(previousTap: CalendarEventTap | null, currentTap: CalendarEventTap): boolean {
	if (!previousTap || previousTap.eventID !== currentTap.eventID) return false;
	if (!isCalendarEventTap(previousTap) || !isCalendarEventTap(currentTap)) return false;
	if (currentTap.end.timestamp - previousTap.end.timestamp > calendarEventTapDurationLimitMs) return false;
	return gestureDistance(previousTap.end, currentTap.end) <= calendarEventDoubleTapDistanceThresholdPx;
}

export function isCalendarEventTap(tap: CalendarEventTap): boolean {
	if (tap.end.timestamp - tap.start.timestamp > calendarEventTapDurationLimitMs) return false;
	return !hasCalendarEventGestureMoved(tap.start, tap.end);
}

function gestureDistance(start: CalendarEventGesturePosition, current: CalendarEventGesturePosition): number {
	return Math.hypot(current.clientX - start.clientX, current.clientY - start.clientY);
}
