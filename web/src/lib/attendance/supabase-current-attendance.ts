import { invokeTool } from '$lib/public-api-call';
import { companyDateOf, companyTimeOf } from '$lib/company-time';
import { attendanceEventFromClock, locationsOf } from './supabase-attendance';
import { defaultLeaveTypes } from './leave-policy-defaults';
import type { CurrentAttendance } from './current-attendance';
import type { AttendanceSummary } from '../../routes/attendance/attendance-context.svelte';

export async function supabaseCurrentAttendance(): Promise<AttendanceSummary> {
	return currentAttendanceSummaryOf(await invokeTool<CurrentAttendance>('attendance_current_get', {}));
}

export function currentAttendanceSummaryOf(state: CurrentAttendance): AttendanceSummary {
	const today = companyDateOf(new Date(state.serverTime), state.timeZone);
	const latest = state.latestEvent;
	const rows = latest && !state.todayEvents.some((event) => event.id === latest.id)
		? [latest, ...state.todayEvents] : state.todayEvents;
	const events = rows.map((event) => attendanceEventFromClock(event, state.email, state.timeZone));
	const leave = state.activeLeave;
	return {
		readScope: 'mine',
		month: today.slice(0, 7), serverTime: state.serverTime, timeZoneAuthoritative: true,
		backdatedAfterMinutes: state.backdatedAfterMinutes,
		currentUserEmail: state.email, currentMemberID: state.memberID,
		isAdmin: state.authorization.isAdmin, timeZone: state.timeZone,
		events,
		absences: leave ? [{ id: `${leave.leaveID}:${today}`, rangeID: leave.leaveID, email: state.email,
			kind: 'leave', labelKey: 'leave', date: today, startDate: today, endDate: today,
			reason: leave.kindID, createdAt: leave.startsAt, isRangeStart: true, isRangeEnd: true }] : [],
		members: [{ memberID: state.memberID, email: state.email, displayName: state.email }],
		todayStatus: state.todayEvents.at(-1)?.kind === 'clock_in' ? 'working' : state.todayEvents.length ? 'done' : 'none',
		activeLeave: leave ? {
			requestID: leave.leaveID, occurrenceID: leave.leaveID, leaveTypeID: leave.kindID,
			leaveTypeName: leave.kindName ?? defaultLeaveTypes().find((kind) => kind.id === leave.kindID)?.name ?? leave.kindID,
			startTime: companyTimeOf(new Date(leave.startsAt), state.timeZone),
			endTime: companyTimeOf(new Date(leave.endsAt), state.timeZone),
			deductionMilliDays: Math.round(leave.days * 1000), startAt: leave.startsAt, endAt: leave.endsAt
		} : undefined,
		locations: locationsOf(state.workLocations),
		teamViewVisibleToAll: state.authorization.teamViewVisibleToAll, teamViewBlocked: false
	};
}
