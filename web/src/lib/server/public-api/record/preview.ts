import { eventOfHint } from './event-tools';
import { taskRowOfHint } from './task-tools';
import type { RecordContext } from './company';
import type { TaskRow } from './tasks';

export type PreviewedTarget = {
	inputField: string;
	id: string;
	title: string;
	startsAt?: string;
};

type Preview = (context: RecordContext, input: Record<string, unknown>) => Promise<PreviewedTarget>;

// inputField is what blueclaw's narrowedToolInput replaces with the identity
// before it runs an approved call; without it the call replays the hint and
// resolves a second time.
const previewsOverTheRecord: Record<string, Preview> = {
	task_delete: async (context, input) =>
		targetOf('taskHint', await taskRowOfHint(context, hintOf(input, 'taskHint'))),
	event_delete: async (context, input) =>
		targetOf('eventHint', await eventOfHint(context, hintOf(input, 'eventHint')))
};

export function previewOfTool(name: string): Preview | undefined {
	return previewsOverTheRecord[name];
}

function hintOf(input: Record<string, unknown>, field: string): string {
	const hint = input[field];
	return typeof hint === 'string' ? hint : '';
}

function targetOf(inputField: string, row: TaskRow): PreviewedTarget {
	return {
		inputField,
		id: row.id,
		title: row.title,
		...(row.starts_at ? { startsAt: row.starts_at } : {})
	};
}
