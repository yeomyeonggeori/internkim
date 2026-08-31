import type { SupabaseClient } from '@supabase/supabase-js';
import { recordContextOf, type RecordContext } from './company';
import { eventAdd, eventDelete, eventList, eventUpdate } from './event-tools';
import { HintUnresolved } from './people';
import { LabelUnresolved } from './labels';
import { NothingMatchesTheHint, RecordRefusedTheWrite } from './tasks';
import { personList, taskAdd, taskDelete, taskList, taskUpdate } from './task-tools';

type ToolInput = Record<string, unknown>;
type ToolRun = (context: RecordContext, input: ToolInput) => Promise<unknown> | unknown;

// A tool runs where its records live. These nine write nothing but rows in the
// record, so nothing about them needs the company's own computer.
const toolsOverTheRecord: Record<string, ToolRun> = {
	task_add: (context, input) => taskAdd(context, input),
	task_update: (context, input) => taskUpdate(context, input),
	task_list: (context, input) => taskList(context, input),
	task_delete: (context, input) => taskDelete(context, input),
	event_add: (context, input) => eventAdd(context, input),
	event_update: (context, input) => eventUpdate(context, input),
	event_list: (context, input) => eventList(context, input),
	event_delete: (context, input) => eventDelete(context, input),
	person_list: (context) => personList(context)
};

export function recordRunsTheTool(name: string): boolean {
	return Object.hasOwn(toolsOverTheRecord, name);
}

export type ToolAnswer = { status: number; body: unknown };

export async function runToolOverTheRecord(
	caller: SupabaseClient,
	requesterID: string,
	name: string,
	input: ToolInput,
	now: Date
): Promise<ToolAnswer> {
	const run = toolsOverTheRecord[name];
	if (!run) return { status: 404, body: { error: `no tool here goes by ${name}` } };

	try {
		const context = await recordContextOf(caller, requesterID, now);
		return { status: 200, body: { tool: name, result: await run(context, input) } };
	} catch (refusal) {
		return refusalAnswer(name, refusal);
	}
}

// A caller who named something the record could not place gets the candidates
// back, so the next call can name one exactly rather than guess again.
function refusalAnswer(name: string, refusal: unknown): ToolAnswer {
	if (refusal instanceof HintUnresolved) {
		return { status: 409, body: { error: refusal.message, hint: refusal.hint, candidates: refusal.candidates } };
	}
	if (refusal instanceof NothingMatchesTheHint) {
		return { status: 409, body: { error: refusal.message, hint: refusal.hint, candidates: refusal.candidates } };
	}
	if (refusal instanceof LabelUnresolved) {
		return { status: 409, body: { error: refusal.message, registered: refusal.registered } };
	}
	if (refusal instanceof RecordRefusedTheWrite) {
		return { status: refusal.status, body: { error: refusal.message } };
	}
	if (refusal instanceof Error) return { status: 400, body: { error: refusal.message } };
	return { status: 500, body: { error: `${name} failed for a reason it did not name` } };
}
