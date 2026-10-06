import { invokeTool } from '$lib/public-api-call';
import type { RecordAttendanceList, RecordLeaveList } from './attendance-record';
import { membersInReadingOrder } from '$lib/member-order';
import {
	approvedLeaveBetween,
	attendanceBetween,
	companyDirectory,
	companySettings,
	orderablePersonOf,
	type RecordPerson
} from '$lib/attendance/attendance-record';
import { shiftedDay } from '$lib/attendance/supabase-work-status-range';
import { companyDateOf, companyTimeOf } from '$lib/company-time';
import {
	supabaseWorkedDay,
	supabaseWorkRecordsFromEvents,
	type SupabaseWorkRecords
} from '$lib/attendance/supabase-work-time';
import {
	supabaseWorkPolicies,
	type SupabaseWorkPolicy
} from '$lib/attendance/supabase-work-policy';
import {
	revisionForDate,
	workingDateForDate
} from '$lib/attendance/work-calendar-derivation';
import {
	intervalsOverlap,
	leaveDayIntervals,
	type DayMinuteInterval
} from '$lib/attendance/leave-day-intervals';
import { workCompliance } from '$lib/attendance/work-compliance';
import { companyTimeInstant } from '$lib/attendance/supabase-work-status-range';
import { attendanceHolidayDates } from '$lib/attendance/attendance-holidays';
import { companyHolidayDatesBetween } from '$lib/attendance/company-holiday-dates';
import { supabaseCompanyHolidays } from '$lib/attendance/supabase-company-holidays';
import type { AttendanceSummaryRecords } from './attendance-summary-records';
import type {
	AttendanceEmployeeWorkStatus,
	AttendanceWorkDayStatus,
	AttendanceWorkStatus,
	AttendanceWorkStatusRequest
} from '../../routes/attendance/attendance-api';

export type SupabaseWorkStatusMember = {
	id: string;
	name: string | null;
	email: string | null;
	is_admin: boolean;
	joined_at: string | null;
};
export type SupabaseWorkStatusAttendance = {
	id?: string;
	member_id: string;
	kind: 'clock_in' | 'clock_out';
	occurred_at: string;
	location?: string | null;
};
export type SupabaseWorkStatusLeave = {
	member_id: string;
	days: number;
	starts_at: string;
	ends_at: string;
	status: string;
};
export type SupabaseEmployeeWorkStatusInput = {
	member: SupabaseWorkStatusMember;
	days: string[];
	timeZone: string;
	attendance: SupabaseWorkStatusAttendance[];
	leave: SupabaseWorkStatusLeave[];
	policy: SupabaseWorkPolicy;
	holidays: ReadonlySet<string>;
	now?: Date;
};

export type SupabaseWorkStatusInputs = {
	timeZone: string;
	members: SupabaseWorkStatusMember[];
	me: SupabaseWorkStatusMember | undefined;
	attendance: SupabaseWorkStatusAttendance[];
	leave: SupabaseWorkStatusLeave[];
	policiesByMember: Map<string, SupabaseWorkPolicy>;
	holidays: ReadonlySet<string>;
	coveredDays: string[];
	now: Date;
};

export function workStatusMemberOf(person: RecordPerson): SupabaseWorkStatusMember {
	return { ...orderablePersonOf(person), is_admin: person.isAdmin === true };
}

export async function supabaseWorkStatusInputs(
	requests: AttendanceWorkStatusRequest[],
	providedSummaryRecords?: AttendanceSummaryRecords,
    scopedPerson?: RecordPerson
): Promise<SupabaseWorkStatusInputs> {
	const requestNow = new Date();
	const summaryRecords = providedSummaryRecords?.readScope === 'mine' ? undefined : providedSummaryRecords;
	const settingsRequest = summaryRecords ? Promise.resolve(summaryRecords.settings) : companySettings();
	const directoryRequest = scopedPerson ? Promise.resolve({requesterID: scopedPerson.personID, count:1, people:[scopedPerson]}) : summaryRecords ? Promise.resolve(summaryRecords.directory) : companyDirectory();
	const policiesRequest = supabaseWorkPolicies();
	const [settings, directory, policiesByMember, records] = await Promise.all([
		settingsRequest,
		directoryRequest,
		policiesRequest,
		(async () => {
			const settings = await settingsRequest;
			const coveredDays = coveredDaysOf(requests, settings.timeZone, requestNow);
			const firstDay = coveredDays[0];
			const lastDay = coveredDays[coveredDays.length - 1];
			const isCovered = summaryRecords && firstDay >= summaryRecords.from && lastDay <= summaryRecords.to;
			const [attendance, leave, holidays] = await Promise.all([
				isCovered ? summaryRecords.attendance : scopedPerson ? invokeTool<RecordAttendanceList>('attendance_list', {personHints:[scopedPerson.personID], from:shiftedDay(firstDay, -1), to:lastDay}) : attendanceBetween(shiftedDay(firstDay, -1), lastDay),
				isCovered ? summaryRecords.leave : scopedPerson ? invokeTool<RecordLeaveList>('leave_list', {personHints:[scopedPerson.personID], status:'approved', from:firstDay, to:lastDay}) : approvedLeaveBetween(firstDay, lastDay),
				workStatusHolidays(coveredDays)
			]);
			return { coveredDays, attendance, leave, holidays };
		})()
	]);
	const timeZone = settings.timeZone;
	const { coveredDays, attendance, leave, holidays } = records;
	const members = directory.people.map(workStatusMemberOf);

	return {
		timeZone,
		members,
		me: members.find((member) => member.id === directory.requesterID),
		attendance: attendance.attendance.map((event) => ({
			id: event.eventID,
			member_id: event.personID,
			kind: event.kind,
			occurred_at: event.occurredAt,
			location: event.location
		})),
		leave: leave.leave.map((taken) => ({
			member_id: taken.personID,
			days: taken.days,
			starts_at: taken.startsAt,
			ends_at: taken.endsAt,
			status: taken.status
		})),
		policiesByMember,
		holidays,
		coveredDays,
		now: requestNow
	};
}

export function attendanceWorkStatusFrom(
	inputs: SupabaseWorkStatusInputs,
	request: AttendanceWorkStatusRequest
): AttendanceWorkStatus {
	const days = daysOf(request, inputs.timeZone, inputs.now);
	const employees = membersInReadingOrder(inputs.members, inputs.me?.id).map((member) =>
		calculateSupabaseEmployeeWorkStatus({
			member,
			days,
			timeZone: inputs.timeZone,
			attendance: inputs.attendance,
			leave: inputs.leave,
			policy: requiredPolicy(inputs.policiesByMember.get(member.id), member.id),
			holidays: inputs.holidays,
			now: inputs.now
		})
	);

	return {
		period: request.period,
		anchor: request.anchor,
		periodStart: days[0],
		periodEnd: days[days.length - 1],
		timeZone: inputs.timeZone,
		isAdmin: inputs.me?.is_admin ?? false,
		personal: employees.find((employee) => employee.email === inputs.me?.email),
		employees
	};
}

export function coveredDaysOf(
	requests: AttendanceWorkStatusRequest[],
	timeZone: string,
	now: Date
): string[] {
	if (requests.length === 0) throw new Error('a work status range needs at least one period');
	const days = new Set<string>();
	for (const request of requests) {
		for (const day of daysOf(request, timeZone, now)) days.add(day);
	}
	const sorted = [...days].sort();
	const covered: string[] = [];
	for (let day = sorted[0]; day <= sorted[sorted.length - 1]; day = shiftedDay(day, 1)) {
		covered.push(day);
	}
	return covered;
}

export function rangeCovers(
	coveredDays: string[],
	requests: AttendanceWorkStatusRequest[],
	timeZone: string,
	now: Date
): boolean {
	if (coveredDays.length === 0) return false;
	const wanted = coveredDaysOf(requests, timeZone, now);
	return wanted[0] >= coveredDays[0] && wanted[wanted.length - 1] <= coveredDays[coveredDays.length - 1];
}

export async function supabaseWorkStatus(
	request: AttendanceWorkStatusRequest
): Promise<AttendanceWorkStatus> {
	return attendanceWorkStatusFrom(await supabaseWorkStatusInputs([request]), request);
}

function requiredPolicy(
	policy: SupabaseWorkPolicy | undefined,
	memberID: string
): SupabaseWorkPolicy {
	if (!policy) throw new Error(`work policy is missing for member ${memberID}`);
	return policy;
}

async function workStatusHolidays(days: string[]): Promise<ReadonlySet<string>> {
	if (days.length === 0) return new Set<string>();
	const range = { from: days[0], to: shiftedDay(days[days.length - 1], 1) };
	return attendanceHolidayDates(
		range,
		companyHolidayDatesBetween(await supabaseCompanyHolidays(), range.from, range.to)
	);
}

export function calculateSupabaseEmployeeWorkStatus(
	input: SupabaseEmployeeWorkStatusInput
): AttendanceEmployeeWorkStatus {
	const { member, days, timeZone, attendance, leave, policy, holidays } = input;
	const now = input.now ?? new Date();
	const mine = attendance.filter((row) => row.member_id === member.id);
	const myLeave = leave.filter((row) => row.member_id === member.id);
	const records = supabaseWorkRecordsFromEvents(mine, now, timeZone);
	const dayStatuses = days.map((day) =>
		dayStatusOf(day, timeZone, records, myLeave, policy, holidays, now)
	);
	const total = (pick: (day: AttendanceWorkDayStatus) => number) =>
		dayStatuses.reduce((sum, day) => sum + pick(day), 0);

	const actualMinutes = total((day) => day.actualMinutes);
	const actualSeconds = total((day) => day.actualSeconds);
	const leaveMinutes = total((day) => (day.hasBaseline ? day.leaveMinutes : 0));
	const baselineActualMinutes = total((day) =>
		day.hasBaseline && !leaveCoversBaseline(day.hasBaseline, day.targetMinutes, day.leaveMinutes)
			? day.actualMinutes + day.provisionalMinutes
			: 0
	);
	const periodTarget = total((day) => (day.hasBaseline ? day.targetMinutes : 0));
	const fulfilledMinutes = Math.min(periodTarget, baselineActualMinutes);
	const email = member.email ?? '';
	const workMode = dayStatuses.at(-1)?.workMode ?? policy.workMode;
	const hasBaseline = dayStatuses.some((day) => day.hasBaseline);
	const referenceDailyMinutes =
		policy.minimumDailyMinutes ??
		(policy.currentPolicy.dailyTargetMinutes > 0 ? policy.currentPolicy.dailyTargetMinutes : 480);

	return {
		email,
		displayName: member.name || email.split('@')[0],
		periodStart: days[0],
		periodEnd: days[days.length - 1],
		workMode,
		hasBaseline,
		targetMinutes: periodTarget,
		actualMinutes,
		actualSeconds,
		provisionalMinutes: total((day) => day.provisionalMinutes),
		provisionalSeconds: total((day) => day.provisionalSeconds),
		workingCapacitySeconds: dayStatuses.filter((day) => day.workingDate).length * 24 * 60 * 60,
		calendarCapacitySeconds: dayStatuses.length * 24 * 60 * 60,
		referenceDailyMinutes,
		leaveMinutes,
		fulfilledMinutes,
		differenceMinutes: baselineActualMinutes - periodTarget,
		remainingMinutes: Math.max(0, periodTarget - fulfilledMinutes),
		overtimeMinutes: Math.max(0, baselineActualMinutes - periodTarget),
		nightMinutes: total((day) => day.nightMinutes),
		isWorking: dayStatuses.some((day) => day.isWorking),
		needsReview: dayStatuses.some((day) => day.needsReview),
		coreTimeMissed: dayStatuses.some((day) => day.coreTimeMissed),
		late: dayStatuses.some((day) => day.late),
		earlyLeave: dayStatuses.some((day) => day.earlyLeave),
		hasLeaveWorkOverlap: dayStatuses.some((day) => day.hasLeaveWorkOverlap),
		hasIncompleteRecords: dayStatuses.some((day) => day.hasIncompleteWorkRecord),
		status: dayStatuses.some((day) => day.isWorking) ? 'working' : 'off',
		days: dayStatuses
	};
}

function dayStatusOf(
	day: string,
	timeZone: string,
	records: SupabaseWorkRecords,
	leave: SupabaseWorkStatusLeave[],
	policy: SupabaseWorkPolicy,
	holidays: ReadonlySet<string>,
	now: Date
): AttendanceWorkDayStatus {
	const revision = revisionForDate(policy.revisions, day);
	const worked = supabaseWorkedDay(records, day, timeZone, revision);
	const segments = worked.spans.map((span) => ({
		startTime: companyTimeOf(span.startAt, timeZone),
		endTime: companyTimeOf(span.endAt, timeZone),
		provisional: span.provisional
	}));
	const workedMinutes = worked.actualMinutes;
	const workedSeconds = worked.actualSeconds;
	const provisionalMinutes = worked.provisionalMinutes;
	const provisionalSeconds = worked.provisionalSeconds;
	const workingDate = workingDateForDate(revision, day, holidays);
	const workMode = revision.workMode;
	const hasBaseline = workMode !== 'autonomous';
	const grossTargetMinutes = workingDate && hasBaseline ? revision.dailyTargetMinutes : 0;
	const dayLeave = leave.filter((row) => leaveRowAppliesToDay(row, day, timeZone));
	const isOnLeave = dayLeave.length > 0;
	const leaveMinutes = leaveMinutesForDay(dayLeave, grossTargetMinutes);
	const dayTargetMinutes = grossTargetMinutes - leaveMinutes;
	const creditedMinutes = workedMinutes + provisionalMinutes;
	const fulfilledMinutes = hasBaseline
		? Math.min(dayTargetMinutes, creditedMinutes)
		: creditedMinutes;
	const baselineWaivedByLeave = leaveCoversBaseline(hasBaseline, dayTargetMinutes, leaveMinutes);
	const differenceMinutes =
		hasBaseline && !baselineWaivedByLeave ? creditedMinutes - dayTargetMinutes : 0;
	const remainingMinutes = hasBaseline ? Math.max(0, dayTargetMinutes - fulfilledMinutes) : 0;
	const overtimeMinutes =
		hasBaseline && !baselineWaivedByLeave ? Math.max(0, creditedMinutes - dayTargetMinutes) : 0;
	const dayStart = new Date(companyTimeInstant(day, '00:00', timeZone)).getTime();
	const workIntervals = worked.spans.map((span) => dayInterval(span.startAt, span.endAt, dayStart));
	const leaveIntervals = leaveDayIntervals(dayLeave, day, timeZone, revision);
	const hasLeaveWorkOverlap = intervalsOverlap(workIntervals, leaveIntervals);
	const needsReview = worked.hasIncompleteWorkRecord || hasLeaveWorkOverlap;
	const compliance = workCompliance(
		revision,
		workingDate,
		workIntervals,
		leaveIntervals,
		Math.round((now.getTime() - dayStart) / 60_000)
	);

	return {
		date: day,
		workMode,
		hasBaseline,
		workingDate,
		targetMinutes: dayTargetMinutes,
		actualMinutes: workedMinutes,
		actualSeconds: workedSeconds,
		provisionalMinutes,
		provisionalSeconds,
		leaveMinutes,
		fulfilledMinutes,
		differenceMinutes,
		remainingMinutes,
		overtimeMinutes,
		nightMinutes: worked.nightMinutes,
		isWorking: worked.isWorking,
		needsReview,
		coreTimeMissed: compliance.coreTimeMissed,
		late: compliance.late,
		earlyLeave: compliance.earlyLeave,
		hasLeaveWorkOverlap,
		hasIncompleteWorkRecord: worked.hasIncompleteWorkRecord,
		status: isOnLeave ? 'leave' : workedMinutes > 0 ? 'worked' : 'off',
		workSegments: segments,
		leaveSegments: []
	};
}

function leaveCoversBaseline(
	hasBaseline: boolean,
	targetMinutes: number,
	leaveMinutes: number
): boolean {
	return hasBaseline && leaveMinutes > 0 && targetMinutes === 0;
}

function dayInterval(startAt: Date, endAt: Date, dayStart: number): DayMinuteInterval {
	const minutesPerDay = 24 * 60;
	const boundedStart = Math.min(Math.max(startAt.getTime(), dayStart), dayStart + minutesPerDay * 60_000);
	const boundedEnd = Math.min(Math.max(endAt.getTime(), dayStart), dayStart + minutesPerDay * 60_000);
	return {
		startMinute: Math.round((boundedStart - dayStart) / 60_000),
		endMinute: Math.round((boundedEnd - dayStart) / 60_000)
	};
}

function leaveRowAppliesToDay(row: SupabaseWorkStatusLeave, day: string, timeZone: string): boolean {
	if (row.days > 0.5) {
		return (
			day >= companyDateOf(new Date(row.starts_at), timeZone) &&
			day < companyDateOf(new Date(row.ends_at), timeZone)
		);
	}
	return day === companyDateOf(new Date(row.starts_at), timeZone);
}

function leaveMinutesForDay(dayLeave: SupabaseWorkStatusLeave[], dayTargetMinutes: number): number {
	const totalMinutes = dayLeave.reduce((total, row) => {
		if (row.days > 0.5) return total + dayTargetMinutes;
		const milliDays = Math.round(row.days * 1000);
		const partialMinutes = Math.ceil((dayTargetMinutes * milliDays) / 1000);
		return total + Math.min(dayTargetMinutes, partialMinutes);
	}, 0);
	return Math.min(dayTargetMinutes, totalMinutes);
}

function daysOf(request: AttendanceWorkStatusRequest, timeZone: string, now: Date): string[] {
	const anchor = request.anchor || companyDateOf(now, timeZone);
	if (request.period === 'day') return [anchor];
	if (request.period === 'week') {
		const weekday = new Date(`${anchor}T00:00:00Z`).getUTCDay();
		const monday = shiftedDay(anchor, -((weekday + 6) % 7));
		return Array.from({ length: 7 }, (_, index) => shiftedDay(monday, index));
	}
	const [year, month] = anchor.split('-').map(Number);
	const dayCount = new Date(Date.UTC(year, month, 0)).getUTCDate();
	return Array.from({ length: dayCount }, (_, index) => `${anchor.slice(0, 7)}-${String(index + 1).padStart(2, '0')}`);
}
