import { invokeTool } from '$lib/public-api-call';
import type { LoadTask } from './task-load-tracker';
import { addTasksFromNote, isAnyTaskAdded, isEveryTaskAdded, type QuickAddOutcome } from './task-quick-add-run';
import { markTaskSnapshotStale } from './task-snapshot-storage';
import type { TaskSummary } from './task-types';

type TaskQuickCreateControllerInput = {
	summary: TaskSummary | null;
	loadTask: LoadTask;
};

export class TaskQuickCreateController {
	note = $state('');
	outcomes = $state<QuickAddOutcome[] | null>(null);
	failure = $state('');
	isAdding = $state(false);

	private summary = $state<TaskSummary | null>(null);
	private loadTask: LoadTask = async () => false;

	sync = (input: TaskQuickCreateControllerInput): void => {
		this.summary = input.summary;
		this.loadTask = input.loadTask;
	};

	addFromNote = async (): Promise<void> => {
		const note = this.note.trim();
		if (!note || this.isAdding) return;
		const outcomes = await this.addingTasksFrom(note);
		if (!outcomes || !isAnyTaskAdded(outcomes)) return;
		markTaskSnapshotStale();
		await this.loadTask(this.summary?.week.code ?? '');
	};

	private async addingTasksFrom(note: string): Promise<QuickAddOutcome[] | null> {
		this.isAdding = true;
		this.failure = '';
		this.outcomes = null;
		try {
			const outcomes = await addTasksFromNote(note, invokeTool);
			this.outcomes = outcomes;
			if (isEveryTaskAdded(outcomes)) this.note = '';
			return outcomes;
		} catch (error) {
			this.failure = error instanceof Error ? error.message : String(error);
			return null;
		} finally {
			this.isAdding = false;
		}
	}
}
