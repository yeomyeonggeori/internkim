import { announceToTheCompany } from './announce-attendance';
import { supabase } from '$lib/supabase';
import { returnEarlyFromSupabaseLeave, supabaseActiveLeave } from './supabase-active-leave';
import { colourOf, type NamedColour } from '$lib/task/task-vocabulary';
import { membersInReadingOrder } from '$lib/member-order';
import type {
	AttendanceAbsence,
	AttendanceEvent,
	AttendanceKind,
	AttendanceLocation,
	AttendanceMember,
	AttendanceSummary
} from '../../routes/attendance/attendance-context.svelte';

type MemberRow = { id: string; name: string | null; email: string | null; is_admin: boolean; user_id: string | null; joined_at: string | null };
type CompanyRow = { id: string; timezone: string; work_locations: NamedColour[] | null; rules: { teamViewVisibleToAll?: boolean } };
type AttendanceRow = {
	id: string;
	member_id: string;
	kind: AttendanceKind;
	location: string | null;
	occurred_at: string;
	original_occurred_at: string | null;
};
type LeaveRow = {
	id: string;
	member_id: string;
	kind: string;
	is_paid: boolean;
	starts_at: string;
	ends_at: string;
	note: string | null;
};

export type SupabaseAttendanceCorrection = {
	eventID: string;
	localDate: string;
	localTime: string;
	locationID: string;
};

export async function supabaseAttendanceSummary(month: string): Promise<AttendanceSummary> {
	const client = supabase();
	const { data: auth } = await client.auth.getSession();
	const accountID = auth.session?.user.id ?? '';

	const company = await client.from('company').select('id, timezone, work_locations, rules').limit(1).single<CompanyRow>();
	if (company.error) throw new Error(company.error.message);

	const members = await client
		.from('member')
		.select('id, name, email, is_admin, user_id, joined_at')
		.neq('status', 'withdrawn')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);

	const timeZone = company.data.timezone;
	const selectedMonth = month || monthIn(new Date(), timeZone);
	const [from, until] = monthBounds(selectedMonth);

	const attendance = await client
		.from('attendance')
		.select('id, member_id, kind, location, occurred_at, original_occurred_at')
		.gte('occurred_at', from.toISOString())
		.lt('occurred_at', until.toISOString())
		.order('occurred_at')
		.returns<AttendanceRow[]>();
	if (attendance.error) throw new Error(attendance.error.message);

	const leave = await client
		.from('leave')
		.select('id, member_id, kind, is_paid, starts_at, ends_at')
		.eq('status', 'approved')
		.lt('starts_at', until.toISOString())
		.gte('ends_at', from.toISOString())
		.returns<LeaveRow[]>();
	if (leave.error) throw new Error(leave.error.message);
	const [serverTime, correctionWindow] = await Promise.all([
		client.rpc('attendance_server_time'),
		client.rpc('attendance_correction_window_minutes')
	]);
	if (serverTime.error) throw new Error(serverTime.error.message);
	if (correctionWindow.error) throw new Error(correctionWindow.error.message);
	const correctionWindowMinutes = Number(correctionWindow.data);
	if (!Number.isInteger(correctionWindowMinutes) || correctionWindowMinutes <= 0) {
		throw new Error('attendance correction window must be a positive integer');
	}

	const byID = new Map(members.data.map((member) => [member.id, member]));
	const me = members.data.find((member) => member.user_id === accountID);
	const events = attendance.data.map((row) => eventOf(row, byID.get(row.member_id), timeZone));

	return {
		month: selectedMonth,
		serverTime: serverTime.data,
		timeZoneAuthoritative: true,
		correctionWindowMinutes,
		currentUserEmail: me?.email ?? '',
		isAdmin: me?.is_admin ?? false,
		timeZone,
		events,
		absences: leave.data.flatMap((row) => absencesOf(row, byID.get(row.member_id), timeZone)),
		members: membersInReadingOrder(members.data, me?.id).map(memberOf),
		todayStatus: todayStatusOf(events, me?.email ?? '', timeZone),
		activeLeave: me ? await supabaseActiveLeave(me.id, timeZone, new Date(serverTime.data)) : undefined,
		locations: locationsOf(company.data.work_locations),
		teamViewVisibleToAll: company.data.rules.teamViewVisibleToAll !== false,
		teamViewBlocked: false
	};
}

export async function setSupabaseTeamViewVisibility(visible: boolean): Promise<void> {
	const client = supabase();
	const company = await client.from('company').select('id, rules').limit(1).single<{ id: string; rules: Record<string, unknown> }>();
	if (company.error) throw new Error(company.error.message);
	const { error } = await client
		.from('company')
		.update({ rules: { ...company.data.rules, teamViewVisibleToAll: visible } })
		.eq('id', company.data.id);
	if (error) throw new Error(error.message);
}

export async function recordSupabaseAttendance(
	kind?: AttendanceKind,
	locationID?: string,
	confirmedEarlyReturn = false
): Promise<void> {
	const client = supabase();
	const { data: auth } = await client.auth.getSession();
	const accountID = auth.session?.user.id;
	if (!accountID) throw new Error('sign in first');

	const member = await client.from('member').select('id').eq('user_id', accountID).single<{ id: string }>();
	if (member.error) throw new Error(member.error.message);

	const recorded = kind ?? (await nextKindFor(member.data.id));
	if (confirmedEarlyReturn && recorded === 'clock_in') {
		await returnEarlyFromSupabaseLeave(locationID);
		void announceToTheCompany('clock');
		return;
	}
	const { error } = await client.from('attendance').insert({
		member_id: member.data.id,
		kind: recorded,
		location: recorded === 'clock_in' ? (locationID || null) : null
	});
	if (error) throw new Error(error.message);
	void announceToTheCompany('clock');
}

export async function correctSupabaseAttendanceEvents(
	corrections: SupabaseAttendanceCorrection[],
	reason: string
): Promise<void> {
	if (corrections.length === 0) return;
	const { error } = await supabase().rpc('attendance_correct', {
		corrections: corrections.map((correction) => ({
			event_id: correction.eventID,
			local_date: correction.localDate,
			local_time: correction.localTime,
			location: correction.locationID
		})),
		reason
	});
	if (error) throw new Error(error.message);
}

async function nextKindFor(memberID: string): Promise<AttendanceKind> {
	const last = await supabase()
		.from('attendance')
		.select('kind')
		.eq('member_id', memberID)
		.order('occurred_at', { ascending: false })
		.limit(1)
		.maybeSingle<{ kind: AttendanceKind }>();
	if (last.error) throw new Error(last.error.message);
	return last.data?.kind === 'clock_in' ? 'clock_out' : 'clock_in';
}

function memberOf(member: MemberRow): AttendanceMember {
	const email = member.email ?? '';
	return { email, displayName: member.name || email.split('@')[0], mattermostUsername: '' };
}

function eventOf(row: AttendanceRow, member: MemberRow | undefined, timeZone: string): AttendanceEvent {
	const email = member?.email ?? '';
	return {
		id: row.id,
		mattermostUserID: '',
		mattermostUsername: '',
		email,
		displayName: member?.name || email.split('@')[0],
		kind: row.kind,
		occurredAt: row.occurred_at,
		originalOccurredAt: row.original_occurred_at ?? undefined,
		localDate: dateIn(new Date(row.occurred_at), timeZone),
		localTime: timeIn(new Date(row.occurred_at), timeZone),
		timeZoneAtEvent: timeZone,
		source: 'web',
		resultPostID: '',
		locationID: row.location ?? undefined,
		locationName: row.location ?? undefined
	};
}

function absencesOf(row: LeaveRow, member: MemberRow | undefined, timeZone: string): AttendanceAbsence[] {
	const startDate = dateIn(new Date(row.starts_at), timeZone);
	const endDate = lastLeaveDate(startDate, row.ends_at, timeZone);
	const email = member?.email ?? '';
	const days: AttendanceAbsence[] = [];
	for (let date = startDate; date <= endDate; date = nextDate(date)) {
		days.push({
			id: `${row.id}:${date}`,
			rangeID: row.id,
			email,
			kind: 'leave',
			labelKey: 'leave',
			date,
			startDate,
			endDate,
			reason: row.note ?? row.kind,
			createdAt: row.starts_at,
			isRangeStart: date === startDate,
			isRangeEnd: date === endDate
		});
	}
	return days;
}

function locationsOf(workLocations: NamedColour[] | null): AttendanceLocation[] {
	return (workLocations ?? []).map((location, index) => ({
		id: location.name,
		name: location.name,
		color: colourOf(location),
		isDefault: index === 0
	}));
}

function todayStatusOf(events: AttendanceEvent[], email: string, timeZone: string): string {
	const today = dateIn(new Date(), timeZone);
	const mine = events.filter((event) => event.email === email && event.localDate === today);
	const last = mine.at(-1);
	if (!last) return 'none';
	return last.kind === 'clock_in' ? 'working' : 'done';
}

function monthBounds(month: string): [Date, Date] {
	const [year, monthNumber] = month.split('-').map(Number);
	return [new Date(Date.UTC(year, monthNumber - 1, 1)), new Date(Date.UTC(year, monthNumber, 1))];
}

function monthIn(instant: Date, timeZone: string): string {
	return dateIn(instant, timeZone).slice(0, 7);
}

function dateIn(instant: Date, timeZone: string): string {
	return new Intl.DateTimeFormat('en-CA', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).format(instant);
}

function timeIn(instant: Date, timeZone: string): string {
	return new Intl.DateTimeFormat('en-GB', {
		timeZone,
		hour: '2-digit',
		minute: '2-digit',
		hour12: false
	}).format(instant);
}

function lastLeaveDate(startDate: string, endsAt: string, timeZone: string): string {
	const lastCoveredInstant = new Date(new Date(endsAt).getTime() - 1);
	const lastDate = dateIn(lastCoveredInstant, timeZone);
	return lastDate < startDate ? startDate : lastDate;
}

function nextDate(date: string): string {
	const moved = new Date(`${date}T00:00:00Z`);
	moved.setUTCDate(moved.getUTCDate() + 1);
	return moved.toISOString().slice(0, 10);
}
