// Imports device attendance and absences into a company. Reads credentials from .env.
//   bun run web/scripts/import-attendance.ts --sqlite <path> --company <uuid> [--apply]
//
// Open a copy, not the device file: it is WAL and read-only opens fail without its sidecars.
// History is imported as it happened. The record's clock-in/clock-out rule guards
// new input, so rows the past violated are reported rather than reshaped, and the
// company's registered work locations are lifted during the run so that events the
// device stored without a location do not silently acquire one.

import { Database } from 'bun:sqlite';
import { controlPlane } from '../src/lib/server/control-plane';

type DeviceEvent = {
	email: string;
	kind: 'clock_in' | 'clock_out';
	occurred_at: string;
	location_name: string | null;
	canceled_at: string | null;
};

type DeviceAbsence = {
	email: string;
	kind: string;
	start_date: string;
	end_date: string;
	reason: string | null;
	canceled_at: string | null;
};

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const sqlitePath = argument('sqlite');
const companyID = argument('company');
const shouldApply = process.argv.includes('--apply');
if (!sqlitePath || !companyID) throw new Error('pass --sqlite <path> --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { data: members, error: memberError } = await client
	.from('member')
	.select('id, email')
	.eq('company_id', companyID);
if (memberError) throw new Error(memberError.message);
const memberByEmail = new Map((members ?? []).map((member) => [member.email ?? '', member.id]));

const device = new Database(sqlitePath);
const events = device
	.query('select email, kind, occurred_at, location_name, canceled_at from attendance_events order by occurred_at')
	.all() as DeviceEvent[];
const absences = device
	.query('select email, kind, start_date, end_date, reason, canceled_at from attendance_absence_ranges')
	.all() as DeviceAbsence[];

const live = events.filter((event) => !(event.canceled_at ?? '').trim());
const unknownPeople = new Set(live.filter((event) => !memberByEmail.has(event.email)).map((event) => event.email));
const importable = live.filter((event) => memberByEmail.has(event.email));

// The same rule the record enforces, applied here so the run reports what it will
// refuse instead of discovering it halfway through.
const previousOf = new Map<string, { kind: string; location: string | null }>();
const rejected: DeviceEvent[] = [];
const accepted: DeviceEvent[] = [];
for (const event of importable) {
	const location = (event.location_name ?? '').trim() || null;
	const previous = previousOf.get(event.email);
	const repeats = event.kind === 'clock_in' && previous?.kind === 'clock_in' && previous.location === location;
	(repeats ? rejected : accepted).push(event);
	if (!repeats) previousOf.set(event.email, { kind: event.kind, location });
}

console.log(`events ${events.length}, cancelled ${events.length - live.length}, importable ${importable.length}`);
if (unknownPeople.size) console.log(`skipped, nobody by that address: ${[...unknownPeople].join(', ')}`);
console.log(`accepted ${accepted.length}, refused by the clock-in rule ${rejected.length}`);
for (const event of rejected) console.log(`  refused ${event.email} ${event.occurred_at} ${event.location_name || '(no location)'}`);

const liveAbsences = absences.filter((absence) => !(absence.canceled_at ?? '').trim() && memberByEmail.has(absence.email));
console.log(`absences ${absences.length}, importable ${liveAbsences.length}`);

if (!shouldApply) {
	console.log('\ndry run — pass --apply to write');
	process.exit(0);
}

const { data: company } = await client
	.from('company')
	.select('work_locations')
	.eq('id', companyID)
	.single();
const registeredLocations = company?.work_locations ?? null;

try {
	await client.from('company').update({ work_locations: null }).eq('id', companyID);

	for (let index = 0; index < accepted.length; index += 200) {
		const batch = accepted.slice(index, index + 200).map((event) => ({
			member_id: memberByEmail.get(event.email),
			kind: event.kind,
			location: event.kind === 'clock_in' ? (event.location_name ?? '').trim() || null : null,
			occurred_at: event.occurred_at,
		}));
		const { error } = await client.from('attendance').insert(batch);
		if (error) throw new Error(`attendance batch at ${index}: ${error.message}`);
		console.log(`  wrote ${Math.min(index + 200, accepted.length)}/${accepted.length}`);
	}

	const leaveRows = liveAbsences.map((absence) => ({
		member_id: memberByEmail.get(absence.email),
		kind: absence.kind,
		is_paid: true,
		// Leave here is not drawn from an allowance, so nothing is deducted.
		is_deducted: false,
		days: inclusiveDays(absence.start_date, absence.end_date),
		status: 'approved',
		starts_at: `${absence.start_date}T00:00:00+09:00`,
		ends_at: `${absence.end_date}T23:59:00+09:00`,
		note: (absence.reason ?? '').trim() || null,
	}));
	if (leaveRows.length) {
		const { error } = await client.from('leave').insert(leaveRows);
		if (error) throw new Error(`leave: ${error.message}`);
		console.log(`  wrote ${leaveRows.length} leave rows`);
	}
} finally {
	await client.from('company').update({ work_locations: registeredLocations }).eq('id', companyID);
	console.log('restored work locations');
}

function inclusiveDays(startDate: string, endDate: string): number {
	const start = new Date(`${startDate}T00:00:00Z`).getTime();
	const end = new Date(`${endDate}T00:00:00Z`).getTime();
	return Math.round((end - start) / 86400000) + 1;
}
