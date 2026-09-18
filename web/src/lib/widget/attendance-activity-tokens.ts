import { claimPushDevice, releasePushDevice } from '$lib/notifications/push-device';
import { attendanceActivityShell, type ActivityToken } from './attendance-activity-plugin';

let listening = false;
let inTurn: Promise<void> = Promise.resolve();

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
	await inTurn;
	const { tokens } = await shell.activity.heldTokens();
	await Promise.all(tokens.map(({ kind, token }) => releasePushDevice({ endpoint: token, kind })));
}

function claim({ kind, token, replaces }: ActivityToken): void {
	if (replaces) {
		afterTheOthers(
			() => releasePushDevice({ endpoint: replaces, kind }),
			'the lock screen token this phone replaced stayed on the record',
			kind
		);
	}
	afterTheOthers(
		() => claimPushDevice({ endpoint: token, kind }),
		'this phone cannot be shown its attendance on the lock screen',
		kind
	);
}

function afterTheOthers(work: () => Promise<unknown>, complaint: string, kind: string): void {
	inTurn = inTurn.then(work).then(
		() => undefined,
		(failure: unknown) => console.warn(complaint, kind, failure)
	);
}
