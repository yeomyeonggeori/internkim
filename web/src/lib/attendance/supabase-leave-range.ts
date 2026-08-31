import { companyDateString, companyDateTimeOf, companyInstantOf, companyTimeString } from '../company-time';
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
	const start = companyDateTimeOf(new Date(startsAt), timeZone);
	const end = companyDateTimeOf(new Date(endsAt), timeZone);
	const startDate = companyDateString(start);
	const startTime = companyTimeString(start);
	const endTime = companyTimeString(end);
	return {
		startDate,
		partialPeriod: partialPeriodOf(days, startTime, endTime),
		startTime,
		endTime
	};
}

export function companyDateOfTimestamp(value: string, timeZone: string): string {
	return companyDateString(companyDateTimeOf(new Date(value), timeZone));
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
	const instant = companyInstantOf(date, time, timeZone);
	if (!instant) {
		throw new Error(`Leave time does not exist in company timezone: ${date} ${time} ${timeZone}`);
	}
	return instant;
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
