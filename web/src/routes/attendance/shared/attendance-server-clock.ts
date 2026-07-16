const rfc3339ServerTimePattern = new RegExp(
	[
		'^(\\d{4})-(\\d{2})-(\\d{2})',
		'T(\\d{2}):(\\d{2}):(\\d{2})',
		'(?:\\.\\d{1,9})?',
		'(?:Z|[+-](\\d{2}):(\\d{2}))$'
	].join('')
);

const monthDayCounts = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];

export type AttendanceServerClock = Readonly<{
	serverTimestampMilliseconds: number;
	monotonicTimestampMilliseconds: number;
}>;

function isInRange(value: number, minimum: number, maximum: number): boolean {
	return value >= minimum && value <= maximum;
}

function isLeapYear(year: number): boolean {
	return year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
}

function daysInMonth(year: number, month: number): number {
	const dayCount = monthDayCounts[month - 1];
	if (dayCount === undefined) return 0;
	if (month === 2 && isLeapYear(year)) return 29;
	return dayCount;
}

function isValidRFC3339ServerTime(serverTime: unknown): serverTime is string {
	if (typeof serverTime !== 'string') return false;
	const fields = rfc3339ServerTimePattern.exec(serverTime);
	if (!fields) return false;

	const year = Number(fields[1]);
	const month = Number(fields[2]);
	const day = Number(fields[3]);
	const hour = Number(fields[4]);
	const minute = Number(fields[5]);
	const second = Number(fields[6]);
	const offsetHour = Number(fields[7] ?? 0);
	const offsetMinute = Number(fields[8] ?? 0);

	return (
		isInRange(month, 1, 12) &&
		isInRange(day, 1, daysInMonth(year, month)) &&
		isInRange(hour, 0, 23) &&
		isInRange(minute, 0, 59) &&
		isInRange(second, 0, 59) &&
		isInRange(offsetHour, 0, 23) &&
		isInRange(offsetMinute, 0, 59)
	);
}

export function createAttendanceServerClock(
	serverTime: unknown,
	monotonicTimestampMilliseconds: number
): AttendanceServerClock | null {
	if (!isValidRFC3339ServerTime(serverTime)) return null;
	const serverTimestamp = Date.parse(serverTime);
	if (!Number.isFinite(serverTimestamp)) return null;
	return { serverTimestampMilliseconds: serverTimestamp, monotonicTimestampMilliseconds };
}

export function attendanceServerTime(
	serverClock: AttendanceServerClock,
	monotonicTimestampMilliseconds: number = performance.now()
): Date {
	const elapsedMilliseconds = monotonicTimestampMilliseconds - serverClock.monotonicTimestampMilliseconds;
	return new Date(serverClock.serverTimestampMilliseconds + elapsedMilliseconds);
}
