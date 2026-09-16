import { isInsideNativeShell } from '$lib/native-shell/shell';
import { nativeReachability, startBeingNativelyReached } from './native-device';

const askedKey = 'internkim.push.asked';

export type FirstAsk = 'allowed' | 'refused' | 'not-asked';

let asking: Promise<FirstAsk> | undefined;

function wasAlreadyAsked(): boolean {
	try {
		return window.localStorage.getItem(askedKey) === 'yes';
	} catch {
		return false;
	}
}

function rememberAsking(): void {
	try {
		window.localStorage.setItem(askedKey, 'yes');
	} catch {
		return;
	}
}

async function ask(): Promise<FirstAsk> {
	if (!isInsideNativeShell() || wasAlreadyAsked()) return 'not-asked';
	if ((await nativeReachability()) !== 'off') {
		rememberAsking();
		return 'not-asked';
	}
	const reach = await startBeingNativelyReached();
	rememberAsking();
	return reach === 'on' ? 'allowed' : 'refused';
}

export function askToBeReachedOnce(): Promise<FirstAsk> {
	asking ??= ask().finally(() => {
		asking = undefined;
	});
	return asking;
}
