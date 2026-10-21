import { isInsideCapacitorShell, isInsideTauriShell, shellPlatform } from '$lib/native-shell/shell';
import { ownPath } from '$lib/return-path';
import { pushDeviceKindOfPlatform, type PushDeviceKind, type Reachability } from './reachability';
import { claimPushDevice, releasePushDevice } from './push-device';
import { capacitorPushDevice } from './native-device-capacitor';
import { tauriPushDevice } from './native-device-tauri';

const heldTokenKey = 'internkim.push.deviceToken';

export type PermissionState = 'granted' | 'denied' | 'prompt' | 'prompt-with-rationale';

export type NativePushDevice = {
	permission(): Promise<PermissionState>;
	askPermission(): Promise<PermissionState>;
	registeredToken(): Promise<string>;
	unregister(): Promise<void>;
	whenTapped(onTapped: (tapped: unknown) => void): Promise<() => void>;
};

const tauriPushPlatforms = ['android', 'ios'];

export function hasNativePush(): boolean {
	if (isInsideTauriShell()) return tauriPushPlatforms.includes(shellPlatform());
	return isInsideCapacitorShell();
}

function nativePushDevice(): NativePushDevice {
	return isInsideTauriShell() ? tauriPushDevice : capacitorPushDevice;
}

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

async function claim(token: string): Promise<void> {
	await claimPushDevice({ endpoint: token, kind: nativeDeviceKind() });
	rememberToken(token);
}

export async function nativeReachability(): Promise<Reachability> {
	return reachabilityOfPermission(await nativePushDevice().permission());
}

export async function startBeingNativelyReached(): Promise<Reachability> {
	const device = nativePushDevice();
	const asked = await device.askPermission();
	if (asked !== 'granted') return reachabilityOfPermission(asked);
	await claim(await device.registeredToken());
	return 'on';
}

export async function stopBeingNativelyReached(): Promise<Reachability> {
	const device = nativePushDevice();
	const token = heldToken();
	if (token) {
		await releasePushDevice({ endpoint: token, kind: nativeDeviceKind() });
		forgetToken();
	}
	await device.unregister();
	return 'off';
}

export async function keepNativeDeviceClaimed(): Promise<void> {
	if (!hasNativePush()) return;
	const device = nativePushDevice();
	if ((await device.permission()) !== 'granted') return;
	await claim(await device.registeredToken());
}

export async function goWhereNativeNotificationsPoint(go: (path: string) => void): Promise<() => void> {
	if (!hasNativePush()) return () => {};
	return nativePushDevice().whenTapped((tapped) => {
		const path = openPathOfTappedNotification(tapped);
		if (path) go(path);
	});
}
