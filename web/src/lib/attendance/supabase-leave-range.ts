import type {
	EmployeeLeavePartialPeriod,
	EmployeeLeavePreviewRequest
} from '../../routes/attendance/leave/employee-leave-types';

type LeavePreviewPeriod = {
	startTime: string;
	endTime: string;
	deductionMilliDays: number;
};

type LeaveTimestampRange = {
	startsAt: string;
	endsAt: string;
};

type LeaveDisplayRange = {
	startDate: string;
	endDate?: string;
	partialPeriod?: EmployeeLeavePartialPeriod;
	startTime?: string;
	endTime?: string;
};

type LocalDateTime = {
	year: number;
	month: number;
	day: number;
	hour: number;
	minute: number;
	second: number;
};

const lunchStartMinutes = 12 * 60;
const lunchDurationMinutes = 60;
const lunchEndMinutes = lunchStartMinutes + lunchDurationMinutes;

export function leavePreviewPeriod(request: EmployeeLeavePreviewRequest): LeavePreviewPeriod {
	if (request.unit === 'fullDay') {
		return { startTime: '09:00', endTime: '18:00', deductionMilliDays: 1000 };
	}
	if (request.partialPeriod === 'afternoon') {
		return {
			startTime: request.unit === 'halfDay' ? '14:00' : '16:00',
			endTime: '18:00',
			deductionMilliDays: request.unit === 'halfDay' ? 500 : 250
		};
	}
	if (request.partialPeriod === 'custom') {
		const startTime = request.startTime ?? '';
		if (!isClockTime(startTime)) throw new Error('Custom leave requires a valid start time');
		return {
			startTime,
			endTime: addWorkMinutes(startTime, request.unit === 'halfDay' ? 240 : 120),
			deductionMilliDays: request.unit === 'halfDay' ? 500 : 250
		};
	}
	return {
		startTime: '09:00',
		endTime: request.unit === 'halfDay' ? '14:00' : '11:00',
		deductionMilliDays: request.unit === 'halfDay' ? 500 : 250
	};
}

export function leaveTimestampRange(
	request: EmployeeLeavePreviewRequest,
	timeZone: string
): LeaveTimestampRange {
	if (request.unit === 'fullDay') {
		return {
			startsAt: companyDateTimeISO(request.startDate, '00:00', timeZone),
			endsAt: companyDateTimeISO(shiftedDay(request.endDate ?? request.startDate, 1), '00:00', timeZone)
		};
	}
	const period = leavePreviewPeriod(request);
	return {
		startsAt: companyDateTimeISO(request.startDate, period.startTime, timeZone),
		endsAt: companyDateTimeISO(request.startDate, period.endTime, timeZone)
	};
}

export function leaveDisplayRange(
	startsAt: string,
	endsAt: string,
	days: number,
	timeZone: string
): LeaveDisplayRange {
	if (days > 0.5) {
		return {
			startDate: leaveAllDayDate(startsAt, timeZone),
			endDate: shiftedDay(leaveAllDayDate(endsAt, timeZone), -1)
		};
	}
	const start = localDateTime(startsAt, timeZone);
	const end = localDateTime(endsAt, timeZone);
	const startDate = dateString(start);
	const startTime = timeString(start);
	const endTime = timeString(end);
	return {
		startDate,
		partialPeriod: partialPeriodOf(days, startTime, endTime),
		startTime,
		endTime
	};
}

export function companyDateOfTimestamp(value: string, timeZone: string): string {
	return dateString(localDateTime(value, timeZone));
}

export function leaveAllDayDate(value: string, timeZone: string): string {
	const timestamp = new Date(value);
	if (
		timestamp.getUTCHours() === 0 &&
		timestamp.getUTCMinutes() === 0 &&
		timestamp.getUTCSeconds() === 0 &&
		timestamp.getUTCMilliseconds() === 0
	) {
		return timestamp.toISOString().slice(0, 10);
	}
	return companyDateOfTimestamp(value, timeZone);
}

function partialPeriodOf(
	days: number,
	startTime: string,
	endTime: string
): EmployeeLeavePartialPeriod {
	if (days === 0.5 && startTime === '09:00' && endTime === '14:00') return 'morning';
	if (days === 0.5 && startTime === '14:00' && endTime === '18:00') return 'afternoon';
	return 'custom';
}

function addWorkMinutes(startTime: string, workMinutes: number): string {
	const [hour, minute] = startTime.split(':').map(Number);
	const startMinutes = hour * 60 + minute;
	const workStartMinutes =
		startMinutes >= lunchStartMinutes && startMinutes < lunchEndMinutes
			? lunchEndMinutes
			: startMinutes;
	let totalMinutes = workStartMinutes + workMinutes;
	if (startMinutes < lunchStartMinutes && totalMinutes > lunchStartMinutes) {
		totalMinutes += lunchDurationMinutes;
	}
	if (totalMinutes > 23 * 60 + 59) throw new Error('Leave end time must be within the selected date');
	return `${twoDigits(Math.floor(totalMinutes / 60))}:${twoDigits(totalMinutes % 60)}`;
}

export function companyDateTimeISO(date: string, time: string, timeZone: string): string {
	if (!isISODate(date) || !isClockTime(time)) throw new Error('Leave range contains an invalid date or time');
	const [year, month, day] = date.split('-').map(Number);
	const [hour, minute] = time.split(':').map(Number);
	const expectedEpoch = Date.UTC(year, month - 1, day, hour, minute);
	let resolvedEpoch = expectedEpoch;
	for (let attempt = 0; attempt < 3; attempt += 1) {
		const resolved = localDateTime(new Date(resolvedEpoch).toISOString(), timeZone);
		const representedEpoch = Date.UTC(
			resolved.year,
			resolved.month - 1,
			resolved.day,
			resolved.hour,
			resolved.minute,
			resolved.second
		);
		const correction = expectedEpoch - representedEpoch;
		if (correction === 0) break;
		resolvedEpoch += correction;
	}
	const result = new Date(resolvedEpoch);
	const resolved = localDateTime(result.toISOString(), timeZone);
	if (dateString(resolved) !== date || timeString(resolved) !== time) {
		throw new Error(`Leave time does not exist in company timezone: ${date} ${time} ${timeZone}`);
	}
	return result.toISOString();
}

function localDateTime(value: string, timeZone: string): LocalDateTime {
	const parts = new Intl.DateTimeFormat('en-CA', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit',
		hourCycle: 'h23'
	}).formatToParts(new Date(value));
	const valueOf = (type: Intl.DateTimeFormatPartTypes): number =>
		Number(parts.find((part) => part.type === type)?.value ?? Number.NaN);
	const result = {
		year: valueOf('year'),
		month: valueOf('month'),
		day: valueOf('day'),
		hour: valueOf('hour'),
		minute: valueOf('minute'),
		second: valueOf('second')
	};
	if (Object.values(result).some(Number.isNaN)) throw new Error(`Invalid leave timestamp: ${value}`);
	return result;
}

function dateString(value: LocalDateTime): string {
	return `${value.year}-${twoDigits(value.month)}-${twoDigits(value.day)}`;
}

function timeString(value: LocalDateTime): string {
	return `${twoDigits(value.hour)}:${twoDigits(value.minute)}`;
}

function shiftedDay(date: string, days: number): string {
	const moved = new Date(`${date}T00:00:00Z`);
	moved.setUTCDate(moved.getUTCDate() + days);
	return moved.toISOString().slice(0, 10);
}

function isClockTime(value: string): boolean {
	const match = /^(\d{2}):(\d{2})$/.exec(value);
	if (!match) return false;
	return Number(match[1]) < 24 && Number(match[2]) < 60;
}

function isISODate(value: string): boolean {
	if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
	const parsed = new Date(`${value}T00:00:00Z`);
	return !Number.isNaN(parsed.getTime()) && parsed.toISOString().slice(0, 10) === value;
}

function twoDigits(value: number): string {
	return String(value).padStart(2, '0');
}
