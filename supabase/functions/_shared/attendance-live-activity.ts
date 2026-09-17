import type { SupabaseClient } from './service-client.ts';
import {
	sendLiveActivity,
	type AttendanceActivityChange,
	type AttendanceActivityState
} from './apns-live-activity.ts';
import type { PushKeys } from './push-keys.ts';

export const activityStartKind = 'apns-activity-start';
export const activityKind = 'apns-activity';

export type ClockedForActivity = { kind: string; location: string | null; occurred_at: string };

type ActivityDevice = { kind: string; address: string };

export function activityChangesFor(
	clocked: ClockedForActivity,
	devices: ActivityDevice[],
	alert: { title: string; body: string }
): { device: ActivityDevice; change: AttendanceActivityChange }[] {
	const running = devices.filter((device) => device.kind === activityKind);
	const starters = devices.filter((device) => device.kind === activityStartKind);
	const state: AttendanceActivityState = {
		startedAt: Math.floor(new Date(clocked.occurred_at).getTime() / 1000),
		location: (clocked.location ?? '').trim()
	};

	if (clocked.kind === 'clock_out') {
		return running.map((device) => ({ device, change: { event: 'end', state } }));
	}
	if (clocked.kind !== 'clock_in') return [];
	if (running.length > 0) {
		return running.map((device) => ({ device, change: { event: 'update', state } }));
	}
	return starters.map((device) => ({ device, change: { event: 'start', state, alert } }));
}

export async function showClockOnOwnPhones(
	record: SupabaseClient,
	memberID: string,
	clocked: ClockedForActivity,
	alert: { title: string; body: string },
	pushKeys: PushKeys,
	nowInSeconds: number
): Promise<number> {
	if (!pushKeys.apns) return 0;
	const { data, error } = await record
		.from('push_device')
		.select('kind, address')
		.eq('member_id', memberID)
		.in('kind', [activityStartKind, activityKind])
		.returns<ActivityDevice[]>();
	if (error) throw new Error(error.message);

	let reached = 0;
	for (const { device, change } of activityChangesFor(clocked, data ?? [], alert)) {
		const outcome = await sendLiveActivity(device.address, change, pushKeys.apns, nowInSeconds);
		if (outcome === 'delivered') reached += 1;
		if (outcome === 'gone' || change.event === 'end') await forgetActivityDevice(record, device);
	}
	return reached;
}

async function forgetActivityDevice(record: SupabaseClient, device: ActivityDevice): Promise<void> {
	const { error } = await record.from('push_device').delete().eq('kind', device.kind).eq('address', device.address);
	if (error) throw new Error(error.message);
}
