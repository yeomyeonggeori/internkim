export type Reachability = 'unsupported' | 'unconfigured' | 'blocked' | 'off' | 'on';

export const pushDeviceKinds = ['web-push', 'apns', 'fcm'] as const;

export type PushDeviceKind = (typeof pushDeviceKinds)[number];

export type PushReachability = {
	serverKey: string;
	isServerKeyVaulted: boolean;
	hasClaimedDevice: boolean;
};

export function pushDeviceKindOfPlatform(platform: string): PushDeviceKind {
	if (platform === 'ios') return 'apns';
	if (platform === 'android') return 'fcm';
	return 'web-push';
}
