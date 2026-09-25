import { z } from 'zod';
import { taskLabelGetResultSchema } from '../catalog/labels';
import { ToolOutcome } from '../catalog/protocol';

export type TaskLabelDraft = { title: string; note: string };

export type DecidedTaskLabels = z.infer<typeof taskLabelGetResultSchema>;

export type TaskLabelDecider = (draft: TaskLabelDraft) => Promise<DecidedTaskLabels | null>;

export const leavesTaskLabelsUndecided: TaskLabelDecider = async () => null;

const answeredTaskLabelsSchema = z.object({
	outcome: z.literal(ToolOutcome.Succeeded),
	result: taskLabelGetResultSchema
});

export function decidedTaskLabelsOf(answer: unknown): DecidedTaskLabels | null {
	const parsed = answeredTaskLabelsSchema.safeParse(answer);
	return parsed.success ? parsed.data.result : null;
}

type LabelledWrite = { title?: string; note?: string; business?: string; type?: string; size?: string };

export function missesATaskLabel(written: LabelledWrite): boolean {
	return written.business === undefined || written.type === undefined || written.size === undefined;
}

export function withDecidedTaskLabels<Written extends LabelledWrite>(
	written: Written,
	decided: DecidedTaskLabels | null
): Written {
	if (!decided) return written;
	return {
		...written,
		business: written.business ?? (decided.business || undefined),
		type: written.type ?? decided.type,
		size: written.size ?? (decided.size || undefined)
	};
}
