import { pushState, replaceState } from '$app/navigation';
import { page } from '$app/state';

export function threadRootIDInHistory(): string | undefined {
	return page.state.openThreadRootID;
}

export function rememberOpenedThread(rootID: string): void {
	const state = { ...page.state, openThreadRootID: rootID };
	if (threadRootIDInHistory() === undefined) pushState('', state);
	else replaceState('', state);
}

export function leaveOpenedThreadThroughHistory(): boolean {
	if (threadRootIDInHistory() === undefined) return false;
	history.back();
	return true;
}
