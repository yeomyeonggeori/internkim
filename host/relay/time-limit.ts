export const readTimeLimitMilliseconds = 3000;

export async function withinTimeLimit<Value>(work: Promise<Value>, milliseconds: number): Promise<Value> {
	let timer: ReturnType<typeof setTimeout> | undefined;
	const expired = new Promise<never>((_, reject) => {
		timer = setTimeout(() => reject(new Error(`no answer within ${milliseconds}ms`)), milliseconds);
	});
	try {
		return await Promise.race([work, expired]);
	} finally {
		clearTimeout(timer);
	}
}
