import type { NativePushDevice, PermissionState } from './native-device';

type PermissionAnswer = { receive: PermissionState };
type DeviceToken = { value: string };
type ListenerHandle = { remove: () => Promise<void> };

type PushPlugin = {
	checkPermissions(): Promise<PermissionAnswer>;
	requestPermissions(): Promise<PermissionAnswer>;
	register(): Promise<void>;
	unregister(): Promise<void>;
	addListener(event: 'registration', on: (token: DeviceToken) => void): Promise<ListenerHandle>;
	addListener(event: 'registrationError', on: (failure: { error: string }) => void): Promise<ListenerHandle>;
	addListener(event: 'pushNotificationActionPerformed', on: (tapped: unknown) => void): Promise<ListenerHandle>;
};

type PushPluginBox = { push: PushPlugin };

async function pushPlugin(): Promise<PushPluginBox> {
	const { PushNotifications } = await import('@capacitor/push-notifications');
	return { push: PushNotifications as unknown as PushPlugin };
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

export const capacitorPushDevice: NativePushDevice = {
	permission: async () => {
		const { push } = await pushPlugin();
		return (await push.checkPermissions()).receive;
	},
	askPermission: async () => {
		const { push } = await pushPlugin();
		return (await push.requestPermissions()).receive;
	},
	registeredToken: async () => {
		const { push } = await pushPlugin();
		return registeredToken(push);
	},
	unregister: async () => {
		const { push } = await pushPlugin();
		await push.unregister();
	},
	whenTapped: async (onTapped) => {
		const { push } = await pushPlugin();
		const handle = await push.addListener('pushNotificationActionPerformed', onTapped);
		return () => void handle.remove().catch(() => undefined);
	}
};
