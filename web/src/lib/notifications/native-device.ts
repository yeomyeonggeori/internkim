import { isInsideNativeShell, shellPlatform } from '$lib/native-shell/shell';
import { ownPath } from '$lib/return-path';
import { pushDeviceKindOfPlatform, type PushDeviceKind, type Reachability } from './reachability';
import { claimPushDevice, releasePushDevice } from './push-device';

const heldTokenKey = 'internkim.push.deviceToken';

type PermissionState = 'granted' | 'denied' | 'prompt' | 'prompt-with-rationale';
type PermissionAnswer = { receive: PermissionState };
type DeviceToken = { value: string };
type TappedNotification = { notification: { data?: unknown } };
type ListenerHandle = { remove: () => Promise<void> };

type PushPlugin = {
	checkPermissions(): Promise<PermissionAnswer>;
	requestPermissions(): Promise<PermissionAnswer>;
	register(): Promise<void>;
	unregister(): Promise<void>;
	addListener(event: 'registration', on: (token: DeviceToken) => void): Promise<ListenerHandle>;
	addListener(event: 'registrationError', on: (failure: { error: string }) => void): Promise<ListenerHandle>;
	addListener(
		event: 'pushNotificationActionPerformed',
		on: (tapped: TappedNotification) => void
	): Promise<ListenerHandle>;
};

export function nativeDeviceKind(): PushDeviceKind {
	return pushDeviceKindOfPlatform(shellPlatform());
}

export function openPathOfTappedNotification(tapped: unknown): string {
	if (typeof tapped !== 'object' || tapped === null) return '';
	const carried = (tapped as { notification?: { data?: unknown } }).notification?.data;
	if (typeof carried !== 'object' || carried === null) return '';
	return ownPath((carried as Record<string, unknown>).openPath);
}

export function reachabilityOfPermission(permission: PermissionState): Reachability {
	if (permission === 'granted') return 'on';
	if (permission === 'denied') return 'blocked';
	return 'off';
}

type PushPluginBox = { push: PushPlugin };

async function pushPlugin(): Promise<PushPluginBox> {
	const { PushNotifications } = await import('@capacitor/push-notifications');
	return { push: PushNotifications as unknown as PushPlugin };
}

function rememberToken(token: string): void {
	try {
		window.localStorage.setItem(heldTokenKey, token);
	} catch {
		return;
	}
}

function heldToken(): string {
	try {
		return window.localStorage.getItem(heldTokenKey) ?? '';
	} catch {
		return '';
	}
}

function forgetToken(): void {
	try {
		window.localStorage.removeItem(heldTokenKey);
	} catch {
		return;
	}
}

async function registeredToken(push: PushPlugin): Promise<string> {
	const handles: ListenerHandle[] = [];
	try {
		return await new Promise<string>((resolve, reject) => {
			const settle = (finish: () => void) => finish();
			void push.addListener('registration', (token) => settle(() => resolve(token.value))).then((handle) =>
				handles.push(handle)
			);
			void push
				.addListener('registrationError', (failure) => settle(() => reject(new Error(failure.error))))
				.then((handle) => handles.push(handle));
			void push.register().catch(reject);
		});
	} finally {
		await Promise.all(handles.map((handle) => handle.remove().catch(() => undefined)));
	}
}

async function claim(token: string): Promise<void> {
	await claimPushDevice({ endpoint: token, kind: nativeDeviceKind() });
	rememberToken(token);
}

export async function nativeReachability(): Promise<Reachability> {
	const { push } = await pushPlugin();
	return reachabilityOfPermission((await push.checkPermissions()).receive);
}

export async function startBeingNativelyReached(): Promise<Reachability> {
	const { push } = await pushPlugin();
	const asked = await push.requestPermissions();
	if (asked.receive !== 'granted') return reachabilityOfPermission(asked.receive);
	await claim(await registeredToken(push));
	return 'on';
}

export async function stopBeingNativelyReached(): Promise<Reachability> {
	const { push } = await pushPlugin();
	const token = heldToken();
	if (token) {
		await releasePushDevice({ endpoint: token, kind: nativeDeviceKind() });
		forgetToken();
	}
	await push.unregister();
	return 'off';
}

export async function keepNativeDeviceClaimed(): Promise<void> {
	if (!isInsideNativeShell()) return;
	const { push } = await pushPlugin();
	if ((await push.checkPermissions()).receive !== 'granted') return;
	await claim(await registeredToken(push));
}

export async function goWhereNativeNotificationsPoint(go: (path: string) => void): Promise<() => void> {
	if (!isInsideNativeShell()) return () => {};
	const { push } = await pushPlugin();
	const handle = await push.addListener('pushNotificationActionPerformed', (tapped) => {
		const path = openPathOfTappedNotification(tapped);
		if (path) go(path);
	});
	return () => void handle.remove().catch(() => undefined);
}
