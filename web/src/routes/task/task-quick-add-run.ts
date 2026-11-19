import { z } from 'zod';
import { ToolRefused } from '$lib/tool-answer';

export type TaskToolInvoker = (name: string, input: Record<string, unknown>) => Promise<unknown>;

export type QuickAddOutcome =
	| { kind: 'added'; title: string; ownerName: string; endDate: string; leftOut: string[] }
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
	let attempt = draft;
	const leftOut: string[] = [];
	for (;;) {
		try {
			const added = addedTaskSchema.parse(await invoke('task_add', attempt));
			return { kind: 'added', title: added.content ?? draft.title, ownerName: added.ownerName ?? '', endDate: added.endDate ?? '', leftOut };
		} catch (refusal) {
			if (!(refusal instanceof ToolRefused)) throw refusal;
			const unresolved = refusal.hint;
			const narrower = unresolved === undefined ? undefined : withoutValue(attempt, unresolved);
			if (!narrower || unresolved === undefined) return { kind: 'refused', title: draft.title, reason: refusal.message };
			attempt = narrower;
			leftOut.push(unresolved);
		}
	}
}

function withoutValue(draft: DraftedTask, value: string): DraftedTask | undefined {
	let isRemoved = false;
	const narrower: DraftedTask = { title: draft.title };
	for (const [field, written] of Object.entries(draft)) {
		if (field === 'title') continue;
		if (written === value) {
			isRemoved = true;
			continue;
		}
		if (Array.isArray(written) && written.includes(value)) {
			isRemoved = true;
			narrower[field] = written.filter((item) => item !== value);
			continue;
		}
		narrower[field] = written;
	}
	return isRemoved ? narrower : undefined;
}

export function isEveryTaskAdded(outcomes: QuickAddOutcome[]): boolean {
	return outcomes.length > 0 && outcomes.every((outcome) => outcome.kind === 'added');
}

export function isAnyTaskAdded(outcomes: QuickAddOutcome[]): boolean {
	return outcomes.some((outcome) => outcome.kind === 'added');
}
