import { describe, expect, mock, test } from 'bun:test';

type PermissionState = 'granted' | 'denied' | 'prompt';

const answered: PermissionState[] = [];

let permission: PermissionState = 'prompt';

const capacitorPluginProxy = new Proxy(
	{},
	{
		get(_target, property) {
			if (property === 'checkPermissions') {
				return async () => {
					answered.push(permission);
					return { receive: permission };
				};
			}
			return () => new Promise(() => {});
		}
	}
);

mock.module('@capacitor/push-notifications', () => ({
	PushNotifications: capacitorPluginProxy
}));

async function within(milliseconds: number, work: Promise<unknown>): Promise<unknown> {
	let deadline: ReturnType<typeof setTimeout> | undefined;
	const timeout = new Promise((_resolve, reject) => {
		deadline = setTimeout(() => reject(new Error(`did not settle within ${milliseconds}ms`)), milliseconds);
	});
	try {
		return await Promise.race([work, timeout]);
	} finally {
		clearTimeout(deadline);
	}
}

describe('loading the native push plugin', () => {
	test('reachability settles rather than waiting on the plugin proxy', async () => {
		const { nativeReachability } = await import('../../src/lib/notifications/native-device');
		permission = 'prompt';
		expect(await within(1000, nativeReachability())).toBe('off');
		expect(answered).toEqual(['prompt']);
	});

	test('a granted permission reads as on', async () => {
		const { nativeReachability } = await import('../../src/lib/notifications/native-device');
		permission = 'granted';
		expect(await within(1000, nativeReachability())).toBe('on');
	});

	test('a denied permission reads as blocked', async () => {
		const { nativeReachability } = await import('../../src/lib/notifications/native-device');
		permission = 'denied';
		expect(await within(1000, nativeReachability())).toBe('blocked');
	});
});
