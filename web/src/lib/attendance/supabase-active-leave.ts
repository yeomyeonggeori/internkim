import { invokeTool } from '$lib/public-api-call';
import { companyDateOf, companyTimeOf } from '$lib/company-time';
import { supabaseLeaveTypeDirectory } from './supabase-leave-types';
import type { RecordLeave, RecordLeaveList } from './attendance-record';
import type { AttendanceActiveLeave } from '../../routes/attendance/attendance-context.svelte';

export async function supabaseActiveLeave(
	timeZone: string,
	now: Date,
	knownLeave?: RecordLeave[]
): Promise<AttendanceActiveLeave | undefined> {
	const today = companyDateOf(now, timeZone);
	const mine = knownLeave ?? (await invokeTool<RecordLeaveList>('leave_list', {
		status: 'approved',
		from: today,
		to: today
	})).leave;
	const covering = mine
		.filter((taken) => coversTheMoment(taken, now))
		.sort((left, right) => left.startsAt.localeCompare(right.startsAt))[0];
	if (!covering) return undefined;

	const directory = await supabaseLeaveTypeDirectory();
	return {
		requestID: covering.leaveID,
		occurrenceID: covering.leaveID,
		leaveTypeID: covering.kindID,
		leaveTypeName: directory.nameOf(covering.kindID),
		startTime: companyTimeOf(new Date(covering.startsAt), timeZone),
		endTime: companyTimeOf(new Date(covering.endsAt), timeZone),
		deductionMilliDays: Math.round(covering.days * 1000),
		startAt: covering.startsAt,
		endAt: covering.endsAt
	};
}

function coversTheMoment(taken: RecordLeave, now: Date): boolean {
	const moment = now.getTime();
	return Date.parse(taken.startsAt) <= moment && Date.parse(taken.endsAt) > moment;
}

export async function returnEarlyFromSupabaseLeave(locationID?: string): Promise<void> {
	await invokeTool('leave_return_early', { location: locationID || undefined });
}
