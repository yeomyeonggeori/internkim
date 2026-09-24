import { z } from 'zod';
import { WorkspaceTaskSize } from '../catalog/workspace-task';

export type TaskLabelDraft = { title: string; note: string };

const decidedTaskLabelsSchema = z.object({
	business: z.string(),
	type: z.string(),
	size: z.union([z.enum(WorkspaceTaskSize), z.literal('')])
});

export type DecidedTaskLabels = z.infer<typeof decidedTaskLabelsSchema>;

export type TaskLabelDecider = (draft: TaskLabelDraft) => Promise<DecidedTaskLabels | null>;

export const leavesTaskLabelsUndecided: TaskLabelDecider = async () => null;

export function decidedTaskLabelsOf(answer: unknown): DecidedTaskLabels | null {
	const parsed = decidedTaskLabelsSchema.safeParse(answer);
	return parsed.success ? parsed.data : null;
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
