import { supabase } from '$lib/supabase';
import { colourOf, type NamedColour } from '$lib/flow/task-vocabulary';
import type {
	AttendanceAbsence,
	AttendanceEvent,
	AttendanceKind,
	AttendanceLocation,
	AttendanceMember,
	AttendanceSummary
} from '../../routes/attendance/attendance-context.svelte';

type MemberRow = { id: string; name: string | null; email: string | null; is_admin: boolean; user_id: string | null };
type CompanyRow = { timezone: string; work_locations: NamedColour[] | null };
type AttendanceRow = { id: string; member_id: string; kind: AttendanceKind; location: string | null; occurred_at: string };
type LeaveRow = {
	id: string;
	member_id: string;
	kind: string;
	is_paid: boolean;
	starts_at: string;
	ends_at: string;
	note: string | null;
};

export async function supabaseAttendanceSummary(month: string): Promise<AttendanceSummary> {
	const client = supabase();
	const { data: auth } = await client.auth.getSession();
	const accountID = auth.session?.user.id ?? '';

	const company = await client.from('company').select('timezone, work_locations').limit(1).single<CompanyRow>();
	if (company.error) throw new Error(company.error.message);

	const members = await client
		.from('member')
		.select('id, name, email, is_admin, user_id')
		.neq('status', 'withdrawn')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);

	const timeZone = company.data.timezone;
	const selectedMonth = month || monthIn(new Date(), timeZone);
	const [from, until] = monthBounds(selectedMonth);

	const attendance = await client
		.from('attendance')
		.select('id, member_id, kind, location, occurred_at')
		.gte('occurred_at', from.toISOString())
		.lt('occurred_at', until.toISOString())
		.order('occurred_at')
		.returns<AttendanceRow[]>();
	if (attendance.error) throw new Error(attendance.error.message);

	const leave = await client
		.from('leave')
		.select('id, member_id, kind, is_paid, starts_at, ends_at, note')
		.lt('starts_at', until.toISOString())
		.gte('ends_at', from.toISOString())
		.returns<LeaveRow[]>();
	if (leave.error) throw new Error(leave.error.message);

	const byID = new Map(members.data.map((member) => [member.id, member]));
	const me = members.data.find((member) => member.user_id === accountID);
	const events = attendance.data.map((row) => eventOf(row, byID.get(row.member_id), timeZone));

	return {
		month: selectedMonth,
		serverTime: new Date().toISOString(),
		timeZoneAuthoritative: true,
		currentUserEmail: me?.email ?? '',
		isAdmin: me?.is_admin ?? false,
		timeZone,
		events,
		absences: leave.data.flatMap((row) => absencesOf(row, byID.get(row.member_id), timeZone)),
		members: [...members.data]
			.sort((left, right) => Number(right.id === me?.id) - Number(left.id === me?.id))
			.map(memberOf),
		todayStatus: todayStatusOf(events, me?.email ?? '', timeZone),
		locations: locationsOf(company.data.work_locations),
		teamViewVisibleToAll: true,
		teamViewBlocked: false
	};
}

export async function recordSupabaseAttendance(kind?: AttendanceKind, locationID?: string): Promise<void> {
	const client = supabase();
	const { data: auth } = await client.auth.getSession();
	const accountID = auth.session?.user.id;
	if (!accountID) throw new Error('sign in first');

	const member = await client.from('member').select('id').eq('user_id', accountID).single<{ id: string }>();
	if (member.error) throw new Error(member.error.message);

	const recorded = kind ?? (await nextKindFor(member.data.id));
	const { error } = await client.from('attendance').insert({
		member_id: member.data.id,
		kind: recorded,
		location: recorded === 'clock_in' ? (locationID || null) : null
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
		localDate: dateIn(new Date(row.occurred_at), timeZone),
		localTime: timeIn(new Date(row.occurred_at), timeZone),
		timeZoneAtEvent: timeZone,
		source: 'web',
		resultPostID: '',
		locationID: row.location ?? undefined,
		locationName: row.location ?? undefined
	};
}

// The screen draws one absence per day, so a leave that spans days becomes one
// entry a day with the ends marked.
function absencesOf(row: LeaveRow, member: MemberRow | undefined, timeZone: string): AttendanceAbsence[] {
	const startDate = dateIn(new Date(row.starts_at), timeZone);
	const endDate = dateIn(new Date(row.ends_at), timeZone);
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

function nextDate(date: string): string {
	const moved = new Date(`${date}T00:00:00Z`);
	moved.setUTCDate(moved.getUTCDate() + 1);
	return moved.toISOString().slice(0, 10);
}
