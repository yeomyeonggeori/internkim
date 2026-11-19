import { z } from 'zod';
import { ToolRefused } from '$lib/tool-answer';

export type TaskToolInvoker = (name: string, input: Record<string, unknown>) => Promise<unknown>;

export type QuickAddOutcome =
	| { kind: 'added'; title: string; ownerName: string; endDate: string }
	| { kind: 'refused'; title: string; reason: string };

const draftedTaskSchema = z.looseObject({ title: z.string() });

type DraftedTask = z.infer<typeof draftedTaskSchema>;

const draftedTasksSchema = z.object({ tasks: z.array(draftedTaskSchema) });

const addedTaskSchema = z.looseObject({
	content: z.string().optional(),
	ownerName: z.string().optional(),
	endDate: z.string().optional()
});

export async function addTasksFromNote(note: string, invoke: TaskToolInvoker): Promise<QuickAddOutcome[]> {
	const { tasks } = draftedTasksSchema.parse(await invoke('task_draft_get', { note }));
	const outcomes: QuickAddOutcome[] = [];
	for (const draft of tasks) outcomes.push(await addDraftedTask(draft, invoke));
	return outcomes;
}

async function addDraftedTask(draft: DraftedTask, invoke: TaskToolInvoker): Promise<QuickAddOutcome> {
	try {
		const added = addedTaskSchema.parse(await invoke('task_add', draft));
		return { kind: 'added', title: added.content ?? draft.title, ownerName: added.ownerName ?? '', endDate: added.endDate ?? '' };
	} catch (refusal) {
		if (!(refusal instanceof ToolRefused)) throw refusal;
		return { kind: 'refused', title: draft.title, reason: refusal.message };
	}
}

export function isEveryTaskAdded(outcomes: QuickAddOutcome[]): boolean {
	return outcomes.length > 0 && outcomes.every((outcome) => outcome.kind === 'added');
}

export function isAnyTaskAdded(outcomes: QuickAddOutcome[]): boolean {
	return outcomes.some((outcome) => outcome.kind === 'added');
}
