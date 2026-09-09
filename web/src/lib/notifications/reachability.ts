export type Reachability = 'unsupported' | 'unconfigured' | 'blocked' | 'off' | 'on';

export type PushDeviceKind = 'web-push' | 'apns' | 'fcm';

export function pushDeviceKindOfPlatform(platform: string): PushDeviceKind {
	if (platform === 'ios') return 'apns';
	if (platform === 'android') return 'fcm';
	return 'web-push';
}
