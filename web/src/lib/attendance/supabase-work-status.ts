import { supabase } from '$lib/supabase';
import { membersInReadingOrder } from '$lib/member-order';
import type {
	AttendanceEmployeeWorkStatus,
	AttendanceWorkDayStatus,
	AttendanceWorkStatus,
	AttendanceWorkStatusRequest
} from '../../routes/attendance/attendance-api';

type MemberRow = {
	id: string;
	name: string | null;
	email: string | null;
	is_admin: boolean;
	user_id: string | null;
	joined_at: string | null;
	minimum_daily_minutes: number | null;
};
type CompanyRow = { timezone: string; minimum_daily_minutes: number | null };
type AttendanceRow = { member_id: string; kind: 'clock_in' | 'clock_out'; occurred_at: string };
type LeaveRow = { member_id: string; days: number; starts_at: string; ends_at: string; status: string };

type WorkedSegment = { startTime: string; endTime: string; provisional: boolean };
type WorkedSpan = { segment: WorkedSegment; minutes: number };

export async function supabaseWorkStatus(request: AttendanceWorkStatusRequest): Promise<AttendanceWorkStatus> {
	const client = supabase();
	const { data: auth } = await client.auth.getSession();
	const accountID = auth.session?.user.id ?? '';

	const company = await client
		.from('company')
		.select('timezone, minimum_daily_minutes')
		.limit(1)
		.single<CompanyRow>();
	if (company.error) throw new Error(company.error.message);

	const members = await client
		.from('member')
		.select('id, name, email, is_admin, user_id, joined_at, minimum_daily_minutes')
		.neq('status', 'withdrawn')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);

	const timeZone = company.data.timezone;
	const days = daysOf(request, timeZone);
	const [from, until] = [days[0], shiftedDay(days[days.length - 1], 1)];

	const attendance = await client
		.from('attendance')
		.select('member_id, kind, occurred_at')
		.gte('occurred_at', instantOf(from))
		.lt('occurred_at', instantOf(until))
		.order('occurred_at')
		.returns<AttendanceRow[]>();
	if (attendance.error) throw new Error(attendance.error.message);

	const leave = await client
		.from('leave')
		.select('member_id, days, starts_at, ends_at, status')
		.eq('status', 'approved')
		.lt('starts_at', instantOf(until))
		.gte('ends_at', instantOf(from))
		.returns<LeaveRow[]>();
	if (leave.error) throw new Error(leave.error.message);

	const me = members.data.find((member) => member.user_id === accountID);
	const employees = membersInReadingOrder(members.data, me?.id).map((member) =>
		employeeStatusOf(member, days, timeZone, attendance.data, leave.data, company.data.minimum_daily_minutes)
	);

	return {
		period: request.period,
		anchor: request.anchor,
		periodStart: days[0],
		periodEnd: days[days.length - 1],
		timeZone,
		isAdmin: me?.is_admin ?? false,
		personal: employees.find((employee) => employee.email === me?.email),
		employees
	};
}

function employeeStatusOf(
	member: MemberRow,
	days: string[],
	timeZone: string,
	attendance: AttendanceRow[],
	leave: LeaveRow[],
	companyMinimumMinutes: number | null
): AttendanceEmployeeWorkStatus {
	const targetMinutes = member.minimum_daily_minutes ?? companyMinimumMinutes ?? 0;
	const mine = attendance.filter((row) => row.member_id === member.id);
	const myLeave = leave.filter((row) => row.member_id === member.id);
	const dayStatuses = days.map((day) => dayStatusOf(day, timeZone, mine, myLeave, targetMinutes));
	const total = (pick: (day: AttendanceWorkDayStatus) => number) =>
		dayStatuses.reduce((sum, day) => sum + pick(day), 0);

	const actualMinutes = total((day) => day.actualMinutes);
	const leaveMinutes = total((day) => day.leaveMinutes);
	const periodTarget = total((day) => day.targetMinutes);
	const fulfilledMinutes = actualMinutes + leaveMinutes;
	const email = member.email ?? '';

	return {
		email,
		displayName: member.name || email.split('@')[0],
		periodStart: days[0],
		periodEnd: days[days.length - 1],
		workMode: 'autonomous',
		hasBaseline: targetMinutes > 0,
		targetMinutes: periodTarget,
		actualMinutes,
		provisionalMinutes: total((day) => day.provisionalMinutes),
		leaveMinutes,
		fulfilledMinutes,
		differenceMinutes: fulfilledMinutes - periodTarget,
		remainingMinutes: Math.max(0, periodTarget - fulfilledMinutes),
		overtimeMinutes: Math.max(0, fulfilledMinutes - periodTarget),
		nightMinutes: 0,
		isWorking: dayStatuses.some((day) => day.isWorking),
		needsReview: dayStatuses.some((day) => day.needsReview),
		coreTimeMissed: false,
		late: false,
		earlyLeave: false,
		hasLeaveWorkOverlap: dayStatuses.some((day) => day.hasLeaveWorkOverlap),
		hasIncompleteRecords: dayStatuses.some((day) => day.hasIncompleteWorkRecord),
		status: dayStatuses.some((day) => day.isWorking) ? 'working' : 'off',
		days: dayStatuses
	};
}

function dayStatusOf(
	day: string,
	timeZone: string,
	attendance: AttendanceRow[],
	leave: LeaveRow[],
	targetMinutes: number
): AttendanceWorkDayStatus {
	const onThisDay = attendance.filter((row) => dateIn(new Date(row.occurred_at), timeZone) === day);
	const { spans, isWorking } = spansOf(onThisDay, timeZone);
	const segments = spans.map((span) => span.segment);
	const workedMinutes = spans.reduce((sum, span) => sum + span.minutes, 0);
	const provisionalMinutes = spans
		.filter((span) => span.segment.provisional)
		.reduce((sum, span) => sum + span.minutes, 0);
	const isOnLeave = leave.some((row) => day >= dateIn(new Date(row.starts_at), timeZone) && day < dateIn(new Date(row.ends_at), timeZone));
	const leaveMinutes = isOnLeave ? targetMinutes : 0;
	const fulfilledMinutes = workedMinutes + leaveMinutes;

	return {
		date: day,
		workMode: 'autonomous',
		hasBaseline: targetMinutes > 0,
		targetMinutes,
		actualMinutes: workedMinutes,
		provisionalMinutes,
		leaveMinutes,
		fulfilledMinutes,
		differenceMinutes: fulfilledMinutes - targetMinutes,
		remainingMinutes: Math.max(0, targetMinutes - fulfilledMinutes),
		overtimeMinutes: Math.max(0, fulfilledMinutes - targetMinutes),
		nightMinutes: 0,
		isWorking,
		needsReview: isWorking && provisionalMinutes > 0,
		coreTimeMissed: false,
		late: false,
		earlyLeave: false,
		hasLeaveWorkOverlap: isOnLeave && workedMinutes > 0,
		hasIncompleteWorkRecord: isWorking,
		status: isOnLeave ? 'leave' : workedMinutes > 0 ? 'worked' : 'off',
		workSegments: segments,
		leaveSegments: []
	};
}

function spansOf(rows: AttendanceRow[], timeZone: string): { spans: WorkedSpan[]; isWorking: boolean } {
	const spans: WorkedSpan[] = [];
	let openedAt: Date | null = null;
	for (const row of rows) {
		if (row.kind === 'clock_in') {
			openedAt = new Date(row.occurred_at);
			continue;
		}
		if (!openedAt) continue;
		spans.push(spanOf(openedAt, new Date(row.occurred_at), timeZone, false));
		openedAt = null;
	}
	if (openedAt) spans.push(spanOf(openedAt, new Date(), timeZone, true));
	return { spans, isWorking: openedAt !== null };
}

function spanOf(startAt: Date, endAt: Date, timeZone: string, provisional: boolean): WorkedSpan {
	return {
		segment: { startTime: timeIn(startAt, timeZone), endTime: timeIn(endAt, timeZone), provisional },
		minutes: Math.max(0, Math.round((endAt.getTime() - startAt.getTime()) / 60000))
	};
}

function daysOf(request: AttendanceWorkStatusRequest, timeZone: string): string[] {
	const anchor = request.anchor || dateIn(new Date(), timeZone);
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

function shiftedDay(day: string, days: number): string {
	const moved = new Date(`${day}T00:00:00Z`);
	moved.setUTCDate(moved.getUTCDate() + days);
	return moved.toISOString().slice(0, 10);
}

function instantOf(day: string): string {
	return new Date(`${day}T00:00:00Z`).toISOString();
}

function dateIn(instant: Date, timeZone: string): string {
	return new Intl.DateTimeFormat('en-CA', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(instant);
}

function timeIn(instant: Date, timeZone: string): string {
	return new Intl.DateTimeFormat('en-GB', { timeZone, hour: '2-digit', minute: '2-digit', hour12: false }).format(instant);
}
