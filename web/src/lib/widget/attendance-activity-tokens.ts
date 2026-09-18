import { invokeTool } from '$lib/public-api-call';
import { attendanceActivityShell, type ActivityToken } from './attendance-activity-plugin';

let listening = false;

export async function keepActivityTokensClaimed(): Promise<void> {
	const shell = await attendanceActivityShell();
	if (!shell) return;
	if (!listening) {
		listening = true;
		await shell.activity.addListener('token', claim);
		await shell.activity.watchTokens();
	}
	const { tokens } = await shell.activity.heldTokens();
	tokens.forEach(claim);
}

export async function releaseActivityTokens(): Promise<void> {
	const shell = await attendanceActivityShell();
	if (!shell) return;
	const { tokens } = await shell.activity.heldTokens();
	await Promise.all(
		tokens.map(({ kind, token }) => invokeTool('push_device_release', { endpoint: token, kind }))
	);
}

function claim({ kind, token, replaces }: ActivityToken): void {
	if (replaces) {
		invokeTool('push_device_release', { endpoint: replaces, kind }).catch((failure: unknown) =>
			console.warn('the lock screen token this phone replaced stayed on the record', kind, failure)
		);
	}
	invokeTool('push_device_claim', { endpoint: token, kind }).catch((failure: unknown) =>
		console.warn('this phone cannot be shown its attendance on the lock screen', kind, failure)
	);
}
