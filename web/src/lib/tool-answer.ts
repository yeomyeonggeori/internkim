import { z } from 'zod';

export class ToolRefused extends Error {
	constructor(
		message: string,
		readonly errorCode: string | undefined,
		readonly status: number
	) {
		super(message);
		this.name = 'ToolRefused';
	}
}

export const refusedToolOutcomes: readonly string[] = ['failed', 'denied'];

const toolAnswerSchema = z.object({
	result: z.unknown().optional(),
	outcome: z.string().optional(),
	message: z.string().optional(),
	error: z.string().optional(),
	errorCode: z.string().optional()
});

export function resultOrRefusal(name: string, status: number, document: unknown): unknown {
	const answered = toolAnswerSchema.safeParse(document);
	const answer = answered.success ? answered.data : {};
	const isRefused = status < 200 || status >= 300 || refusedToolOutcomes.includes(answer.outcome ?? '');
	if (isRefused) {
		throw new ToolRefused(answer.error ?? answer.message ?? `${name} answered ${status}`, answer.errorCode, status);
	}
	if (answer.result === undefined) throw new Error(`${name} answered nothing`);
	return answer.result;
}
