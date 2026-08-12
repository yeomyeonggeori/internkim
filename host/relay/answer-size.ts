export type Answer = { callID: string; status: number; body: unknown };

const answerOverheadBytes = 512;

// measure-broadcast-ceiling.ts took 3,000,491 bytes and got 422 "Payload size exceeds
// tenant limit" on the next, so the 3 MB Supabase publishes for Pro and Team is decimal.
export const largestMessageTheProPlanCarries = 3_000_000;
export const defaultAnswerByteCeiling = largestMessageTheProPlanCarries;

export function answerByteLength(answer: Answer): number {
	return new TextEncoder().encode(JSON.stringify(answer)).length;
}

export function largestRawBytesThatFit(ceiling: number): number {
	return Math.floor(((ceiling - answerOverheadBytes) * 3) / 4);
}

export function oversizeNotice(answer: Answer, ceiling: number): Answer | null {
	const bytes = answerByteLength(answer);
	if (bytes <= ceiling) return null;
	return {
		callID: answer.callID,
		status: 413,
		body: { error: `the answer is ${bytes} bytes, over the ${ceiling} this relay sends` }
	};
}
