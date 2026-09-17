import { isInsideNativeShell, shellPlatform } from '$lib/native-shell/shell';
import { invokeTool } from '$lib/public-api-call';

type ActivityToken = { kind: 'apns-activity-start' | 'apns-activity'; token: string };

type AttendanceActivityPlugin = {
	watchTokens(): Promise<void>;
	heldTokens(): Promise<{ tokens: ActivityToken[] }>;
	addListener(event: 'token', listener: (token: ActivityToken) => void): Promise<{ remove(): Promise<void> }>;
};

type AttendanceActivityShell = { activity: AttendanceActivityPlugin };

let listening = false;

async function attendanceActivityShell(): Promise<AttendanceActivityShell | null> {
	if (!isInsideNativeShell() || shellPlatform() !== 'ios') return null;
	const { registerPlugin } = await import('@capacitor/core');
	return { activity: registerPlugin<AttendanceActivityPlugin>('AttendanceActivity') };
}

export async function keepActivityTokensClaimed(): Promise<void> {
	const shell = await attendanceActivityShell();
	if (!shell || listening) return;
	listening = true;
	await shell.activity.addListener('token', ({ kind, token }) => {
		invokeTool('push_device_claim', { endpoint: token, kind }).catch((failure: unknown) =>
			console.warn('this phone cannot be shown its attendance on the lock screen', kind, failure)
		);
	});
	await shell.activity.watchTokens();
}

export async function releaseActivityTokens(): Promise<void> {
	const shell = await attendanceActivityShell();
	if (!shell) return;
	const { tokens } = await shell.activity.heldTokens();
	await Promise.all(
		tokens.map(({ kind, token }) => invokeTool('push_device_release', { endpoint: token, kind }))
	);
}
