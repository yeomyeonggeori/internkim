import { dayIn, dayOfInstant, dayShifted, instantOfDay } from './days';
import { personOfHint } from './people';
import { statusOfPostgresCode, RecordRefusedTheWrite } from './tasks';
import {
	attendanceByID,
	attendanceOfCompany,
	timeOfInstant,
	NoSuchAttendanceRecord,
	type AttendanceRow
} from './attendance';
import type { RecordContext } from './company';

const defaultWindowDays = 30;
const hintWindowDays = 90;
const mostRecentRows = 1000;

export type AnsweredAttendance = {
	eventID: string;
	person: string;
	kind: string;
	date: string;
	time: string;
	location: string | null;
	wasCorrected: boolean;
	reason: string | null;
};

function answeredAttendance(context: RecordContext, row: AttendanceRow): AnsweredAttendance {
	const nameOf = new Map(context.people.map((person) => [person.personID, person.name]));
	return {
		eventID: row.id,
		person: nameOf.get(row.member_id) ?? row.member_id,
		kind: row.kind,
		date: dayOfInstant(context.labels.timezone, row.occurred_at),
		time: timeOfInstant(context.labels.timezone, row.occurred_at),
		location: row.location,
		wasCorrected: row.original_occurred_at !== null,
		reason: row.edit_reason
	};
}

function describedAttendance(context: RecordContext, row: AttendanceRow): string {
	const answered = answeredAttendance(context, row);
	return `${answered.person} · ${answered.kind} · ${answered.date} ${answered.time}`;
}

function windowOf(context: RecordContext, from: string | undefined, to: string | undefined) {
	const today = dayIn(context.labels.timezone, context.now);
	const firstDay = from?.trim() || dayShifted(today, -defaultWindowDays);
	const lastDay = to?.trim() || today;
	return {
		firstDay,
		lastDay,
		from: instantOfDay(context.labels.timezone, firstDay),
		to: instantOfDay(context.labels.timezone, lastDay, true)
	};
}

function targetMember(context: RecordContext, personHint: string | undefined): string {
	return personHint ? personOfHint(context.people, personHint).personID : context.requesterID;
}

async function rowsInWindow(
	context: RecordContext,
	personHint: string | undefined,
	scope: string | undefined,
	from: string | undefined,
	to: string | undefined
): Promise<{ rows: AttendanceRow[]; memberID: string | null; firstDay: string; lastDay: string }> {
	const window = windowOf(context, from, to);
	const everyone = scope === 'all' && !personHint;
	const memberID = everyone ? null : targetMember(context, personHint);
	const rows = await attendanceOfCompany(context.caller, window.from, window.to, mostRecentRows);
	return {
		rows: memberID ? rows.filter((row) => row.member_id === memberID) : rows,
		memberID,
		firstDay: window.firstDay,
		lastDay: window.lastDay
	};
}

// An identifier is asked for by name, because scanning a window for it would
// miss any record the window or the row ceiling left out.
async function attendanceOfHint(context: RecordContext, hint: string): Promise<AttendanceRow> {
	const asked = hint.trim();
	if (!asked) throw new NoSuchAttendanceRecord(hint, []);

	const named = await attendanceByID(context.caller, asked).catch(() => null);
	if (named) return named;

	const found = await rowsInWindow(context, undefined, 'all', reachableFrom(context), undefined);
	const described = found.rows.filter((row) => describedAttendance(context, row).includes(asked));
	if (described.length === 1) return described[0];
	throw new NoSuchAttendanceRecord(
		hint,
		described.map((row) => describedAttendance(context, row))
	);
}

export type AttendanceListInput = {
	personHint?: string;
	scope?: string;
	from?: string;
	to?: string;
	limit?: number;
};

export async function attendanceList(context: RecordContext, input: AttendanceListInput) {
	const found = await rowsInWindow(context, input.personHint, input.scope, input.from, input.to);
	const kept = input.limit && input.limit > 0 ? found.rows.slice(-input.limit) : found.rows;
	return {
		scope: found.memberID ? 'person' : 'everyone',
		personID: found.memberID,
		personName: found.memberID
			? context.people.find((one) => one.personID === found.memberID)?.name ?? ''
			: '',
		from: found.firstDay,
		to: found.lastDay,
		count: kept.length,
		attendance: kept.map((row) => answeredAttendance(context, row))
	};
}

export type AttendanceAddInput = {
	personHint?: string;
	kind?: string;
	date?: string;
	time?: string;
	location?: string;
	reason?: string;
};

export async function attendanceAdd(context: RecordContext, input: AttendanceAddInput) {
	if (input.kind !== 'clock_in' && input.kind !== 'clock_out') {
		throw new Error('an attendance record is a clock_in or a clock_out');
	}
	if (!input.date?.trim() || !input.time?.trim()) {
		throw new Error('an attendance record names the date and the time it happened');
	}
	if (!input.reason?.trim()) throw new Error('an attendance record says why it is being written by hand');

	const memberID = targetMember(context, input.personHint);
	const { data, error } = await context.caller.rpc('attendance_add', {
		target_member: memberID,
		kind: input.kind,
		local_date: input.date.trim(),
		local_time: input.time.trim(),
		location: input.location?.trim() || null,
		reason: input.reason.trim()
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return answeredWrite(data);
}

export type AttendanceUpdateInput = {
	eventHint?: string;
	date?: string;
	time?: string;
	location?: string;
	reason?: string;
};

export async function attendanceUpdate(context: RecordContext, input: AttendanceUpdateInput) {
	if (!input.eventHint) throw new Error('a correction names the attendance record it corrects');
	if (!input.reason?.trim()) throw new Error('a correction says why the record was wrong');

	const row = await attendanceOfHint(context, input.eventHint);
	const held = answeredAttendance(context, row);

	const { data, error } = await context.caller.rpc('attendance_correct', {
		corrections: [
			{
				event_id: row.id,
				local_date: input.date?.trim() || held.date,
				local_time: input.time?.trim() || held.time,
				location: input.location?.trim() || held.location
			}
		],
		reason: input.reason.trim()
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return answeredWrite(data);
}

export type AttendanceDeleteInput = { eventHint?: string; reason?: string };

export async function attendanceDelete(context: RecordContext, input: AttendanceDeleteInput) {
	if (!input.eventHint) throw new Error('a removal names the attendance record it removes');
	if (!input.reason?.trim()) throw new Error('a removal says why the record should not be there');

	const row = await attendanceOfHint(context, input.eventHint);

	const { data, error } = await context.caller.rpc('attendance_remove', {
		event_id: row.id,
		reason: input.reason.trim()
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return answeredWrite(data);
}

function reachableFrom(context: RecordContext): string {
	return dayShifted(dayIn(context.labels.timezone, context.now), -hintWindowDays);
}

function answeredWrite(written: unknown) {
	const answer = (written ?? {}) as { status?: string; eventID?: string; approvalID?: string };
	return {
		status: answer.status ?? 'unknown',
		eventID: answer.eventID ?? null,
		approvalID: answer.approvalID ?? null
	};
}
