const quickRefusalMilliseconds = 1500;
const retryDelayMilliseconds = 1000;
const waitLimitMilliseconds = 20000;

export type PasskeyClock = {
	now: () => number;
	sleep: (milliseconds: number) => Promise<void>;
};

const realClock: PasskeyClock = {
	now: () => Date.now(),
	sleep: (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds))
};

export async function waitingForPasskeyApproval<Result>(
	ceremony: () => Promise<Result>,
	insideDesktopApp: boolean,
	isRefusal: (error: unknown) => boolean,
	clock: PasskeyClock = realClock
): Promise<Result> {
	const startedAt = clock.now();
	for (;;) {
		const attemptedAt = clock.now();
		try {
			return await ceremony();
		} catch (error) {
			const refusedAt = clock.now();
			const refusedAtOnce = refusedAt - attemptedAt < quickRefusalMilliseconds;
			const mayWaitLonger = refusedAt - startedAt < waitLimitMilliseconds;
			const awaitingApproval = insideDesktopApp && isRefusal(error) && refusedAtOnce;
			if (!awaitingApproval || !mayWaitLonger) throw error;
			await clock.sleep(retryDelayMilliseconds);
		}
	}
}
