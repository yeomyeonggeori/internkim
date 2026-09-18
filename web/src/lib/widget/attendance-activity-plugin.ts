import { isInsideNativeShell, shellPlatform } from '$lib/native-shell/shell';

export type ActivityToken = {
	kind: 'apns-activity-start' | 'apns-activity';
	token: string;
	replaces?: string;
};

export type AttendanceActivityPlugin = {
	watchTokens(): Promise<void>;
	heldTokens(): Promise<{ tokens: ActivityToken[] }>;
	lockScreenState(): Promise<{ allowed: boolean }>;
	closeCards(): Promise<void>;
	addListener(event: 'token', listener: (token: ActivityToken) => void): Promise<{ remove(): Promise<void> }>;
};

export type AttendanceActivityShell = { activity: AttendanceActivityPlugin };

export async function attendanceActivityShell(): Promise<AttendanceActivityShell | null> {
	if (!isInsideNativeShell() || shellPlatform() !== 'ios') return null;
	const { registerPlugin } = await import('@capacitor/core');
	return { activity: registerPlugin<AttendanceActivityPlugin>('AttendanceActivity') };
}
