import type { ApnsKey } from './apns.ts';
import type { SupabaseClient } from './service-client.ts';
import {
	sendLiveActivity,
	type AttendanceActivityChange,
	type AttendanceActivityState
} from './apns-live-activity.ts';
import { closedWorkedMinutesOn, companyDayOf } from './attendance-worked-time.ts';
import type { PushKeys } from './push-keys.ts';

export const activityStartKind = 'apns-activity-start';
export const activityKind = 'apns-activity';

export type ClockedForActivity = { kind: string; location: string | null; occurred_at: string };

type ActivityDevice = { kind: string; address: string };

export function activityChangesFor(
	clocked: ClockedForActivity,
	devices: ActivityDevice[],
	alert: { title: string; body: string },
	earlierMinutes: number
): { device: ActivityDevice; change: AttendanceActivityChange }[] {
	const running = devices.filter((device) => device.kind === activityKind);
	const starters = devices.filter((device) => device.kind === activityStartKind);
	const state: AttendanceActivityState = {
		startedAt: Math.floor(new Date(clocked.occurred_at).getTime() / 1000),
		earlierMinutes: Math.max(0, Math.round(earlierMinutes)),
		location: (clocked.location ?? '').trim()
	};

	const ends = running.map((device) => ({ device, change: { event: 'end', state } as AttendanceActivityChange }));
	if (clocked.kind === 'clock_out') return ends;
	if (clocked.kind !== 'clock_in') return [];
	return [
		...ends,
		...starters.map((device) => ({ device, change: { event: 'start', state, alert } as AttendanceActivityChange }))
	];
}

export function activityEndsWithNothingOnRecord(
	devices: ActivityDevice[],
	nowInSeconds: number
): { device: ActivityDevice; change: AttendanceActivityChange }[] {
	const state: AttendanceActivityState = { startedAt: nowInSeconds, earlierMinutes: 0, location: '' };
	return devices
		.filter((device) => device.kind === activityKind)
		.map((device) => ({ device, change: { event: 'end', state } as AttendanceActivityChange }));
}

export async function showClockOnOwnPhones(
	record: SupabaseClient,
	memberID: string,
	clocked: ClockedForActivity,
	alert: { title: string; body: string },
	pushKeys: PushKeys,
	nowInSeconds: number,
	companyTimeZone: string
): Promise<number> {
	if (!pushKeys.apns) return 0;
	const listening = await activityDevicesOf(record, memberID);
	if (listening.length === 0) return 0;

	const earlierMinutes =
		clocked.kind === 'clock_in' ? await minutesWorkedBefore(record, memberID, clocked, companyTimeZone) : 0;
	return sendActivityChanges(record, activityChangesFor(clocked, listening, alert, earlierMinutes), pushKeys.apns, nowInSeconds);
}

export async function endActivityOnOwnPhones(
	record: SupabaseClient,
	memberID: string,
	pushKeys: PushKeys,
	nowInSeconds: number
): Promise<number> {
	if (!pushKeys.apns) return 0;
	const listening = await activityDevicesOf(record, memberID);
	return sendActivityChanges(record, activityEndsWithNothingOnRecord(listening, nowInSeconds), pushKeys.apns, nowInSeconds);
}

async function activityDevicesOf(record: SupabaseClient, memberID: string): Promise<ActivityDevice[]> {
	const { data, error } = await record
		.from('push_device')
		.select('kind, address')
		.eq('member_id', memberID)
		.in('kind', [activityStartKind, activityKind])
		.returns<ActivityDevice[]>();
	if (error) throw new Error(error.message);
	return data ?? [];
}

async function sendActivityChanges(
	record: SupabaseClient,
	changes: { device: ActivityDevice; change: AttendanceActivityChange }[],
	key: ApnsKey,
	nowInSeconds: number
): Promise<number> {
	let reached = 0;
	for (const { device, change } of changes) {
		const outcome = await sendLiveActivity(device.address, change, key, nowInSeconds);
		if (outcome === 'delivered') reached += 1;
		if (outcome === 'gone' || change.event === 'end') await forgetActivityDevice(record, device);
	}
	return reached;
}

async function forgetActivityDevice(record: SupabaseClient, device: ActivityDevice): Promise<void> {
	const { error } = await record.from('push_device').delete().eq('kind', device.kind).eq('address', device.address);
	if (error) throw new Error(error.message);
}

async function minutesWorkedBefore(
	record: SupabaseClient,
	memberID: string,
	clocked: ClockedForActivity,
	companyTimeZone: string
): Promise<number> {
	const clockedAt = new Date(clocked.occurred_at);
	const from = new Date(clockedAt.getTime() - 2 * 24 * 60 * 60 * 1000);
	const { data, error } = await record
		.from('attendance')
		.select('kind, occurred_at')
		.eq('member_id', memberID)
		.is('deleted_at', null)
		.gte('occurred_at', from.toISOString())
		.lte('occurred_at', clockedAt.toISOString())
		.order('occurred_at')
		.returns<{ kind: string; occurred_at: string }[]>();
	if (error) throw new Error(error.message);
	return closedWorkedMinutesOn(companyDayOf(clockedAt, companyTimeZone), data ?? [], companyTimeZone);
}
