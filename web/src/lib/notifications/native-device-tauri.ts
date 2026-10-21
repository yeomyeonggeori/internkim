import type { NativePushDevice, PermissionState } from './native-device';

function permissionStateOf(answer: NotificationPermission): PermissionState {
	if (answer === 'granted') return 'granted';
	if (answer === 'denied') return 'denied';
	return 'prompt';
}

function permissionStateOfGranted(granted: boolean | null): PermissionState {
	if (granted === true) return 'granted';
	if (granted === false) return 'denied';
	return 'prompt';
}

export const tauriPushDevice: NativePushDevice = {
	permission: async () => {
		const { isPermissionGranted } = await import('@choochmeque/tauri-plugin-notifications-api');
		return permissionStateOfGranted(await isPermissionGranted());
	},
	askPermission: async () => {
		const { isPermissionGranted, requestPermission } = await import('@choochmeque/tauri-plugin-notifications-api');
		if (await isPermissionGranted()) return 'granted';
		return permissionStateOf(await requestPermission());
	},
	registeredToken: async () => {
		const { registerForPushNotifications } = await import('@choochmeque/tauri-plugin-notifications-api');
		return registerForPushNotifications();
	},
	unregister: async () => {
		const { unregisterForPushNotifications } = await import('@choochmeque/tauri-plugin-notifications-api');
		await unregisterForPushNotifications();
	},
	whenTapped: async (onTapped) => {
		const { onNotificationClicked } = await import('@choochmeque/tauri-plugin-notifications-api');
		const listener = await onNotificationClicked((clicked) => onTapped({ notification: { data: clicked.data } }));
		return () => void listener.unregister().catch(() => undefined);
	}
};
