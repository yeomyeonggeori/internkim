//   bun run web/scripts/compare-device-attendance.ts --device https://<device>/ --url <project> --key <service role> --months 2026-06,2026-07,2026-08

import { createClient } from '@supabase/supabase-js';
import { windowOf } from './attendance-window';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const deviceURL = (argument('device') ?? '').replace(/\/$/, '');
const projectURL = argument('url') ?? '';
const serviceRoleKey = argument('key') ?? '';
const months = (argument('months') ?? '').split(',').filter(Boolean);
if (!deviceURL || !projectURL || !serviceRoleKey || months.length === 0) {
	throw new Error('pass --device, --url, --key and --months');
}

type DeviceEvent = { kind?: string; occurredAt?: string; mattermostUserID?: string; email?: string };
type RecordedEvent = { kind: string; occurred_at: string; member_id: string };

const admin = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

function minuteOf(moment: string): string {
	return new Date(moment).toISOString().slice(0, 16);
}

async function deviceEvents(month: string): Promise<DeviceEvent[]> {
	const response = await fetch(`${deviceURL}/attendance/api/summary?month=${month}`);
	if (!response.ok) throw new Error(`the device answered ${response.status} for ${month}`);
	const answered = (await response.json()) as { events?: DeviceEvent[] };
	const { from, to } = windowOf(month);
	return (answered.events ?? []).filter((event) => {
		const moment = event.occurredAt ? new Date(event.occurredAt).toISOString() : '';
		return moment >= from && moment < to;
	});
}

async function recordedEvents(month: string): Promise<RecordedEvent[]> {
	const from = `${month}-01T00:00:00Z`;
	const [year, index] = month.split('-').map(Number);
	const nextMonth = index === 12 ? `${year + 1}-01` : `${year}-${String(index + 1).padStart(2, '0')}`;
	const { data, error } = await admin
		.from('attendance')
		.select('kind, occurred_at, member_id')
		.gte('occurred_at', from)
		.lt('occurred_at', `${nextMonth}-01T00:00:00Z`)
		.returns<RecordedEvent[]>();
	if (error) throw new Error(error.message);
	return data ?? [];
}

async function membersByExternalID(): Promise<Map<string, string>> {
	const { data, error } = await admin
		.from('member')
		.select('id, messenger')
		.returns<{ id: string; messenger: Record<string, string> | null }[]>();
	if (error) throw new Error(error.message);
	return new Map(
		(data ?? [])
			.map((member) => [member.messenger?.mattermost ?? '', member.id] as const)
			.filter(([externalID]) => externalID !== '')
	);
}

const memberOf = await membersByExternalID();
let deviceTotal = 0;
let recordTotal = 0;
let onlyOnTheDevice = 0;
let onlyInTheRecord = 0;

for (const month of months) {
	const [fromDevice, fromRecord] = await Promise.all([deviceEvents(month), recordedEvents(month)]);
	const deviceKeys = new Set(
		fromDevice
			.filter((event) => event.occurredAt && event.kind)
			.map((event) => `${memberOf.get(event.mattermostUserID ?? '') ?? event.email ?? '?'}|${event.kind}|${minuteOf(event.occurredAt ?? '')}`)
	);
	const recordKeys = new Set(
		fromRecord.map((event) => `${event.member_id}|${event.kind}|${minuteOf(event.occurred_at)}`)
	);

	const missing = [...deviceKeys].filter((key) => !recordKeys.has(key)).length;
	const strays = [...recordKeys].filter((key) => !deviceKeys.has(key));
	const extra = strays.length;
	for (const stray of strays) console.log(`  only in the record: ${stray}`);
	deviceTotal += fromDevice.length;
	recordTotal += fromRecord.length;
	onlyOnTheDevice += missing;
	onlyInTheRecord += extra;

	console.log(`${month}  device ${String(fromDevice.length).padStart(4)}  record ${String(fromRecord.length).padStart(4)}  missing ${missing}  stale ${extra}`);
}

console.log('');
console.log(`device ${deviceTotal} · record ${recordTotal}`);
console.log(`${onlyOnTheDevice} on the device and not in the record`);
console.log(`${onlyInTheRecord} in the record and not on the device`);
