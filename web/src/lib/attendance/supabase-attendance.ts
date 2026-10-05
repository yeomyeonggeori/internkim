import { isAttendanceChangeReason } from './change-reason';
import { announceToTheCompany } from './announce-attendance';
import { companyDateOf, companyTimeOf } from '$lib/company-time';
import { shiftedDay } from './supabase-work-status-range';
import {
	attendanceWriteResultFrom,
	type AttendanceWriteEvent,
	type AttendanceWriteResult
} from './attendance-write';
import { invokeTool } from '$lib/public-api-call';
import {
	approvedLeaveBetween,
	attendanceBetween,
	companyDirectory,
	companySettings,
	orderablePersonOf,
	type RecordAttendance,
	type RecordAttendanceList,
	type RecordLeave,
	type RecordLeaveList,
	type RecordWorkLocation
} from './attendance-record';
import { returnEarlyFromSupabaseLeave, supabaseActiveLeave } from './supabase-active-leave';
import { attendanceSummaryRecords } from './attendance-summary-records';
import { colourOf } from '$lib/task/task-vocabulary';
import { membersInReadingOrder, type OrderableMember } from '$lib/member-order';
import type {
	AttendanceAbsence,
	AttendanceEvent,
	AttendanceKind,
	AttendanceLocation,
	AttendanceMember,
	AttendanceSummary
} from '../../routes/attendance/attendance-context.svelte';

export type SupabaseAttendanceCorrection = {
	eventID: string;
	localDate: string;
	localTime: string;
	locationID: string;
};

export type SupabaseAttendanceAddition = {
	email: string;
	kind: AttendanceKind;
	localDate: string;
	localTime: string;
	locationID: string;
	reason: string;
};

export async function supabaseAttendanceSummary(month: string): Promise<AttendanceSummary> {
	const settingsRequest = companySettings();
	const directoryRequest = companyDirectory();
	const recordsRequest = (async () => {
		const selectedMonth = month || monthIn(new Date(), (await settingsRequest).timeZone);
		const firstDay = `${selectedMonth}-01`;
		const lastDay = lastDayOfMonth(selectedMonth);
		const [attendance, leave] = await Promise.all([
			attendanceBetween(shiftedDay(firstDay, -1), lastDay),
			approvedLeaveBetween(firstDay, lastDay)
		]);
		return { selectedMonth, attendance, leave };
	})();
	const [settings, directory, { selectedMonth, attendance, leave }] = await Promise.all([
		settingsRequest,
		directoryRequest,
		recordsRequest
	]);
	const timeZone = settings.timeZone;
	const firstDay = `${selectedMonth}-01`;
	const lastDay = lastDayOfMonth(selectedMonth);

	const me = directory.people.find((person) => person.personID === directory.requesterID);
	const emailOf = new Map(directory.people.map((person) => [person.personID, person.email]));
	const events = attendance.attendance.map((row) =>
		eventOf(row, emailOf.get(row.personID) ?? '', timeZone)
	);
	const myEmail = me?.email ?? '';
	const serverNow = new Date(attendance.serverTime);
	const today = companyDateOf(serverNow, timeZone);
	const todayLeave = today >= firstDay && today <= lastDay
		? leave.leave.filter((taken) => taken.personID === me?.personID)
		: undefined;

	return {
		[attendanceSummaryRecords]: { readScope: 'all', settings, directory, attendance, leave, from: firstDay, to: lastDay },
		readScope: 'all',
		month: selectedMonth,
		serverTime: attendance.serverTime,
		timeZoneAuthoritative: true,
		backdatedAfterMinutes: attendance.backdatedAfterMinutes,
		currentUserEmail: myEmail,
		currentMemberID: me?.personID,
		isAdmin: me?.isAdmin === true,
		timeZone,
		events,
		absences: leave.leave.flatMap((row) => absencesOf(row, emailOf.get(row.personID) ?? '')),
		members: membersInReadingOrder(
			directory.people.map(orderablePersonOf),
			me?.personID
		).map(memberOf),
		todayStatus: todayStatusOf(events, myEmail, timeZone),
		activeLeave: me ? await supabaseActiveLeave(timeZone, serverNow, todayLeave) : undefined,
		locations: locationsOf(settings.workLocations),
		teamViewVisibleToAll: settings.teamViewVisibleToAll,
		teamViewBlocked: false
	};
}

// The visible page reuses the monthly day-cell model, with only today's
// records and the existing 24-hour shift carry window.
export async function supabaseAttendanceTodaySummary(members: AttendanceMember[], requester: AttendanceSummary): Promise<AttendanceSummary> {
 const today = companyDateOf(new Date(), requester.timeZone);
 const personHints = members.map(member => member.memberID);
 const [attendance, leave] = await Promise.all([
  invokeTool<RecordAttendanceList>('attendance_list', {personHints, from: shiftedDay(today, -1), to: today}),
  invokeTool<RecordLeaveList>('leave_list', {personHints, status:'approved', from:today, to:today})
 ]);
 if (attendance.count >= 20000) throw new Error('Today attendance exceeded the supported page');
 const emails = new Map(members.map(member => [member.memberID, member.email]));
 return {...requester, readScope:'person', month:today.slice(0,7), members,
  events:attendance.attendance.map(row => eventOf(row, emails.get(row.personID) ?? '', requester.timeZone)),
  absences:leave.leave.flatMap(row => absencesOf(row, emails.get(row.personID) ?? ''))};
}

// A selected person's month uses only that member's event and leave rows. The
// selected month is shown directly in the person side panel.
export async function supabaseAttendancePersonSummary(
	month: string,
	member: AttendanceMember,
	requester: AttendanceSummary
): Promise<AttendanceSummary> {
	if (!member.memberID || !/^\d{4}-(0[1-9]|1[0-2])$/.test(month)) {
		throw new Error('Select a member and month to read attendance');
	}
	const firstDay = `${month}-01`;
	const lastDay = lastDayOfMonth(month);
	const personHints = [member.memberID];
	const [attendance, leave] = await Promise.all([
		invokeTool<RecordAttendanceList>('attendance_list', {
			personHints, from: shiftedDay(firstDay, -1), to: lastDay
		}),
		invokeTool<RecordLeaveList>('leave_list', {
			personHints, status: 'approved', from: firstDay, to: lastDay
		})
	]);
	if (attendance.count >= 20000) throw new Error('Attendance history exceeds the supported page; narrow the period');
	const events = attendance.attendance.map((row) => eventOf(row, member.email, requester.timeZone));
	return {
		readScope: 'person',
		month,
		serverTime: attendance.serverTime,
		timeZoneAuthoritative: true,
		backdatedAfterMinutes: attendance.backdatedAfterMinutes,
		currentUserEmail: requester.currentUserEmail,
		currentMemberID: requester.currentMemberID,
		isAdmin: requester.isAdmin,
		timeZone: requester.timeZone,
		events,
		absences: leave.leave.flatMap((row) => absencesOf(row, member.email)),
		members: [member],
		todayStatus: todayStatusOf(events, member.email, requester.timeZone),
		locations: requester.locations,
		teamViewVisibleToAll: requester.teamViewVisibleToAll,
		teamViewBlocked: false
	};
}

export async function setSupabaseTeamViewVisibility(visible: boolean): Promise<void> {
	await invokeTool('company_settings_update', { teamViewVisibleToAll: visible });
}

export async function recordSupabaseAttendance(
	kind: AttendanceKind,
	locationID?: string,
	confirmedEarlyReturn = false
): Promise<AttendanceWriteResult | void> {
	if (confirmedEarlyReturn && kind === 'clock_in') {
		await returnEarlyFromSupabaseLeave(locationID);
		void announceToTheCompany('clock');
		return;
	}
	return attendanceWriteResultFrom(await invokeTool('attendance_add', {
		kind,
		location: kind === 'clock_in' ? locationID || undefined : undefined
	}));
}

export function attendanceEventFromClock(
	event: AttendanceWriteEvent,
	email: string,
	timeZone: string
): AttendanceEvent {
	const occurredAt = new Date(event.occurredAt);
	return {
		id: event.id,
		email,
		displayName: email,
		kind: event.kind,
		occurredAt: event.occurredAt,
		localDate: companyDateOf(occurredAt, timeZone),
		localTime: companyTimeOf(occurredAt, timeZone),
		timeZoneAtEvent: timeZone,
		source: 'web',
		resultPostID: '',
		locationID: event.location ?? undefined,
		locationName: event.location ?? undefined
	};
}

export async function correctSupabaseAttendanceEvents(
	corrections: SupabaseAttendanceCorrection[],
	reason: string
): Promise<AttendanceWriteResult> {
	if (corrections.length === 0) throw new Error('at least one attendance correction is required');
	return attendanceWriteResultFrom(
		await invokeTool('attendance_update', {
			corrections: corrections.map((correction) => ({
				eventHint: correction.eventID,
				date: correction.localDate,
				time: correction.localTime,
				location: correction.locationID || undefined
			})),
			...(isAttendanceChangeReason(reason) ? {reasonCode: reason} : {reason})
		})
	);
}

export async function addSupabaseAttendanceEvent(
	addition: SupabaseAttendanceAddition
): Promise<AttendanceWriteResult> {
	if (!addition.email) throw new Error('an attendance record needs the person it belongs to');
	return attendanceWriteResultFrom(
		await invokeTool('attendance_add', {
			personHint: addition.email,
			kind: addition.kind,
			date: addition.localDate,
			time: addition.localTime,
			location: addition.kind === 'clock_in' ? addition.locationID || undefined : undefined,
			...(isAttendanceChangeReason(addition.reason) ? {reasonCode: addition.reason} : {reason: addition.reason})
		})
	);
}

export async function removeSupabaseAttendanceEvent(
	eventID: string,
	reason: string
): Promise<AttendanceWriteResult> {
	return attendanceWriteResultFrom(await invokeTool('attendance_delete', { eventHint: eventID, ...(isAttendanceChangeReason(reason) ? {reasonCode: reason} : {reason}) }));
}

function memberOf(member: OrderableMember): AttendanceMember {
	const email = member.email ?? '';
	return { memberID: member.id, email, displayName: member.name || email.split('@')[0] };
}

function eventOf(row: RecordAttendance, email: string, timeZone: string): AttendanceEvent {
	return {
		id: row.eventID,
		email,
		displayName: row.person,
		kind: row.kind,
		occurredAt: row.occurredAt,
		originalOccurredAt: row.originalOccurredAt ?? undefined,
		localDate: row.date,
		localTime: row.time,
		timeZoneAtEvent: timeZone,
		source: 'web',
		resultPostID: '',
		locationID: row.location ?? undefined,
		locationName: row.location ?? undefined
	};
}

function absencesOf(row: RecordLeave, email: string): AttendanceAbsence[] {
	const startDate = row.startDate;
	const endDate = row.endDate < startDate ? startDate : row.endDate;
	const days: AttendanceAbsence[] = [];
	for (let date = startDate; date <= endDate; date = nextDate(date)) {
		days.push({
			id: `${row.leaveID}:${date}`,
			rangeID: row.leaveID,
			email,
			kind: 'leave',
			labelKey: 'leave',
			date,
			startDate,
			endDate,
			reason: row.note ?? row.kindID,
			createdAt: row.startsAt,
			isRangeStart: date === startDate,
			isRangeEnd: date === endDate
		});
	}
	return days;
}

export function locationsOf(workLocations: RecordWorkLocation[]): AttendanceLocation[] {
	return workLocations.map((location, index) => ({
		id: location.name,
		name: location.name,
		color: colourOf({ name: location.name, ...(location.color ? { color: location.color } : {}) }),
		isDefault: index === 0
	}));
}

function todayStatusOf(events: AttendanceEvent[], email: string, timeZone: string): string {
	const today = companyDateOf(new Date(), timeZone);
	const mine = events.filter((event) => event.email === email && event.localDate === today);
	const last = mine.at(-1);
	if (!last) return 'none';
	return last.kind === 'clock_in' ? 'working' : 'done';
}

function monthIn(instant: Date, timeZone: string): string {
	return companyDateOf(instant, timeZone).slice(0, 7);
}

function lastDayOfMonth(month: string): string {
	const [year, monthNumber] = month.split('-').map(Number);
	const dayCount = new Date(Date.UTC(year, monthNumber, 0)).getUTCDate();
	return `${month}-${String(dayCount).padStart(2, '0')}`;
}

function nextDate(date: string): string {
	const moved = new Date(`${date}T00:00:00Z`);
	moved.setUTCDate(moved.getUTCDate() + 1);
	return moved.toISOString().slice(0, 10);
}
