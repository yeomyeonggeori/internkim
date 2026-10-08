import type { SupabaseClient } from './service-client.ts';
import type { ApnsKey } from './apns.ts';
import { sendApnsBackground } from './apns-background.ts';
import { sendFcmWidgetRefresh } from './fcm.ts';
import type { PushKeys } from './push-keys.ts';
import type { PushOutcome } from './push-vocabulary.ts';

export function widgetClockOf(newestClock: { kind: string } | null): string {
	return newestClock?.kind ?? 'clock_out';
}

export function widgetRefreshPayload(clock: string): Record<string, unknown> {
	return { aps: { 'content-available': 1 }, widget: 'attendance', clock };
}

export function sendWidgetRefresh(
	deviceToken: string,
	clock: string,
	key: ApnsKey,
	nowInSeconds: number
): Promise<PushOutcome> {
	return sendApnsBackground(deviceToken, widgetRefreshPayload(clock), key, nowInSeconds);
}

export async function refreshOwnWidgets(
	record: SupabaseClient,
	memberID: string,
	clock: string,
	pushKeys: PushKeys,
	nowInSeconds: number
): Promise<number> {
	const kinds = widgetDeviceKinds(pushKeys);
	if (kinds.length === 0) return 0;
	const { data, error } = await record
		.from('push_device')
		.select('kind, address')
		.eq('member_id', memberID)
		.in('kind', kinds)
		.returns<{ kind: string; address: string }[]>();
	if (error) throw new Error(error.message);

	let reached = 0;
	for (const device of data ?? []) {
		if ((await refreshWidgetOn(device, clock, pushKeys, nowInSeconds)) === 'delivered') reached += 1;
	}
	return reached;
}

export function widgetDeviceKinds(pushKeys: PushKeys): string[] {
	return [pushKeys.apns ? 'apns' : '', pushKeys.fcm ? 'fcm' : ''].filter((kind) => kind !== '');
}

async function refreshWidgetOn(
	device: { kind: string; address: string },
	clock: string,
	pushKeys: PushKeys,
	nowInSeconds: number
): Promise<PushOutcome> {
	if (device.kind === 'apns' && pushKeys.apns) return sendWidgetRefresh(device.address, clock, pushKeys.apns, nowInSeconds);
	if (device.kind === 'fcm' && pushKeys.fcm) return sendFcmWidgetRefresh(device.address, clock, pushKeys.fcm, nowInSeconds);
	return 'refused';
}
