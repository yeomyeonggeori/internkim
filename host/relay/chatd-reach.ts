import { reasonOf } from './failure';
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
	const asAService = await withChatdServiceToken(init, process.env.CHATD_SERVICE_TOKEN_PATH);
	for (;;) {
		try {
			return await fetch(url, asAService);
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
	return new Error(`nothing listened for ${waitMilliseconds / 1000} s: ${reasonOf(failure)}`);
}

export async function withChatdServiceToken(init: RequestInit, tokenPath: string | undefined): Promise<RequestInit> {
	const keptAt = tokenPath?.trim();
	if (!keptAt) return init;
	const kept = await Bun.file(keptAt)
		.text()
		.catch((failure: unknown) => {
			throw new Error(`the token chatd asks of the host services could not be read at ${keptAt}: ${reasonOf(failure)}`);
		});
	const headers = new Headers(init.headers);
	headers.set('Authorization', `Bearer ${kept.trim()}`);
	return { ...init, headers };
}
