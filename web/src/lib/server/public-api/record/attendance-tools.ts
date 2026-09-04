import { dayIn, dayOfInstant, dayShifted, instantOfDay } from './days';
import { personOfHint } from './people';
import { statusOfPostgresCode, RecordRefusedTheWrite } from './tasks';
import {
	attendanceBackdatedAfterMinutes,
	attendanceByID,
	attendanceClock,
	attendanceOfCompany,
	attendanceWrittenByHand,
	timeOfInstant,
	NoSuchAttendanceRecord,
	type AttendanceRow
} from './attendance';
import type { RecordContext } from './company';

const defaultWindowDays = 30;
const hintWindowDays = 90;
const mostRecentRows = 20000;

export type AnsweredAttendance = {
	eventID: string;
	personID: string;
	person: string;
	kind: string;
	date: string;
	time: string;
	occurredAt: string;
	location: string | null;
	wasCorrected: boolean;
	originalDate: string | null;
	originalTime: string | null;
	originalOccurredAt: string | null;
	reason: string | null;
};

function answeredAttendance(context: RecordContext, row: AttendanceRow): AnsweredAttendance {
	const nameOf = new Map(context.people.map((person) => [person.personID, person.name]));
	return {
		eventID: row.id,
		personID: row.member_id,
		person: nameOf.get(row.member_id) ?? row.member_id,
		kind: row.kind,
		date: dayOfInstant(context.labels.timezone, row.occurred_at),
		time: timeOfInstant(context.labels.timezone, row.occurred_at),
		occurredAt: new Date(row.occurred_at).toISOString(),
		location: row.location,
		wasCorrected: row.original_occurred_at !== null,
		originalDate: row.original_occurred_at
			? dayOfInstant(context.labels.timezone, row.original_occurred_at)
			: null,
		originalTime: row.original_occurred_at
			? timeOfInstant(context.labels.timezone, row.original_occurred_at)
			: null,
		originalOccurredAt: row.original_occurred_at
			? new Date(row.original_occurred_at).toISOString()
			: null,
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
	handWrittenOnly?: boolean;
	limit?: number;
};

export async function attendanceList(context: RecordContext, input: AttendanceListInput) {
	const [found, serverTime, backdatedAfterMinutes] = await Promise.all([
		rowsInWindow(context, input.personHint, input.scope, input.from, input.to),
		attendanceClock(context.caller),
		attendanceBackdatedAfterMinutes(context.caller)
	]);
	const written = input.handWrittenOnly ? attendanceWrittenByHand(found.rows) : found.rows;
	const kept = input.limit && input.limit > 0 ? written.slice(-input.limit) : written;
	return {
		scope: found.memberID ? 'person' : 'everyone',
		personID: found.memberID,
		personName: found.memberID
			? context.people.find((one) => one.personID === found.memberID)?.name ?? ''
			: '',
		from: found.firstDay,
		to: found.lastDay,
		serverTime,
		backdatedAfterMinutes,
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
	const memberID = targetMember(context, input.personHint);
	const { data, error } = await context.caller.rpc('attendance_add', {
		target_member: memberID,
		kind: input.kind,
		local_date: input.date?.trim() || null,
		local_time: input.time?.trim() || null,
		location: input.location?.trim() || null,
		reason: input.reason?.trim() || null
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return answeredWrite(data);
}

export type AttendanceCorrection = {
	eventHint?: string;
	date?: string;
	time?: string;
	location?: string;
};

export type AttendanceUpdateInput = { corrections?: AttendanceCorrection[]; reason?: string };

export async function attendanceUpdate(context: RecordContext, input: AttendanceUpdateInput) {
	const asked = input.corrections ?? [];
	if (asked.length === 0) throw new Error('a correction names the attendance records it corrects');

	const corrections = [];
	for (const correction of asked) {
		if (!correction.eventHint) throw new Error('a correction names the attendance record it corrects');
		const row = await attendanceOfHint(context, correction.eventHint);
		const held = answeredAttendance(context, row);
		corrections.push({
			event_id: row.id,
			local_date: correction.date?.trim() || held.date,
			local_time: correction.time?.trim() || held.time,
			location: correction.location?.trim() || held.location
		});
	}

	const { data, error } = await context.caller.rpc('attendance_correct', {
		corrections,
		reason: input.reason?.trim() || null
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return { ...answeredWrite(data), eventID: corrections[0].event_id };
}

export type AttendanceDeleteInput = { eventHint?: string; reason?: string };

export async function attendanceDelete(context: RecordContext, input: AttendanceDeleteInput) {
	if (!input.eventHint) throw new Error('a removal names the attendance record it removes');

	const row = await attendanceOfHint(context, input.eventHint);

	const { data, error } = await context.caller.rpc('attendance_remove', {
		event_id: row.id,
		reason: input.reason?.trim() || null
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return answeredWrite(data);
}

function reachableFrom(context: RecordContext): string {
	return dayShifted(dayIn(context.labels.timezone, context.now), -hintWindowDays);
}

function answeredWrite(written: unknown) {
	const answer = (written ?? {}) as { status?: string; eventID?: string; backdated?: boolean };
	return {
		status: answer.status ?? 'unknown',
		eventID: answer.eventID ?? null,
		backdated: answer.backdated === true
	};
}
