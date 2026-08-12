export type Answer = { callID: string; status: number; body: unknown };

const answerOverheadBytes = 512;

// https://supabase.com/docs/guides/realtime/limits
export const largestMessageTheChannelCarries = 1_000_000;
const envelopeShare = 0.1;
export const defaultAnswerByteCeiling = Math.floor(largestMessageTheChannelCarries * (1 - envelopeShare));

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
		body: { error: `the answer is ${bytes} bytes, over the ${ceiling} this channel carries` }
	};
}
