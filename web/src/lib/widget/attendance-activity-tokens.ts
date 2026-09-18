import { claimPushDevice, releasePushDevice } from '$lib/notifications/push-device';
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
	await Promise.all(tokens.map(({ kind, token }) => releasePushDevice({ endpoint: token, kind })));
}

function claim({ kind, token, replaces }: ActivityToken): void {
	if (replaces) {
		releasePushDevice({ endpoint: replaces, kind }).catch((failure: unknown) =>
			console.warn('the lock screen token this phone replaced stayed on the record', kind, failure)
		);
	}
	claimPushDevice({ endpoint: token, kind }).catch((failure: unknown) =>
		console.warn('this phone cannot be shown its attendance on the lock screen', kind, failure)
	);
}
