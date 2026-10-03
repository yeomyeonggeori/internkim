export const chatdRestartWaitMilliseconds = 30_000;

const firstPauseMilliseconds = 250;
const longestPauseMilliseconds = 4_000;
const refusedConnectionCode = 'ConnectionRefused';

export async function fetchWhenChatdListens(
	url: string,
	init: RequestInit,
	waitMilliseconds = chatdRestartWaitMilliseconds
): Promise<Response> {
	const giveUpAt = Date.now() + waitMilliseconds;
	let pauseMilliseconds = firstPauseMilliseconds;
	for (;;) {
		try {
			return await fetch(url, init);
		} catch (failure) {
			if (!wasRefused(failure)) throw failure;
			const remainingMilliseconds = giveUpAt - Date.now();
			if (remainingMilliseconds <= 0) throw failureAfterWaiting(failure, waitMilliseconds);
			await Bun.sleep(Math.min(pauseMilliseconds, remainingMilliseconds));
			pauseMilliseconds = Math.min(pauseMilliseconds * 2, longestPauseMilliseconds);
		}
	}
}

function wasRefused(failure: unknown): boolean {
	return typeof failure === 'object' && failure !== null && 'code' in failure && failure.code === refusedConnectionCode;
}

function failureAfterWaiting(failure: unknown, waitMilliseconds: number): Error {
	const reason = failure instanceof Error ? failure.message : String(failure);
	return new Error(`nothing listened for ${waitMilliseconds / 1000} s: ${reason}`);
}
