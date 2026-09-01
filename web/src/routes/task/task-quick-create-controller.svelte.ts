import type { LoadTask } from './task-load-tracker';
import { requestQuickTaskCreation } from './task-quick-task-create';
import { defaultTaskOwner } from './task-draft';
import { taskText } from './text';
import type { TaskQuickTaskCreateResult, TaskSummary } from './task-types';
import type { PageText } from '$lib/i18n/page-text.svelte';

type TaskPageText = PageText<typeof taskText>;

type TaskQuickCreateControllerInput = {
	summary: TaskSummary | null;
	text: TaskPageText;
	loadTask: LoadTask;
};

export class TaskQuickCreateController {
	quickTaskText = $state('');
	taskErrorMessage = $state('');
	quickTaskDuplicateMessage = $state('');
	isCreatingQuickTask = $state(false);

	private summary = $state<TaskSummary | null>(null);
	private text: TaskPageText = taskText.ko;
	private loadTask: LoadTask;
	private quickTaskDuplicatePrompt = $state('');

	constructor() {
		this.loadTask = async () => false;
	}

	sync = (input: TaskQuickCreateControllerInput): void => {
		this.summary = input.summary;
		this.text = input.text;
		this.loadTask = input.loadTask;
	};

	clearStaleDuplicatePrompt = (): void => {
		if (!this.quickTaskDuplicateMessage) return;
		if (this.quickTaskText.trim() === this.quickTaskDuplicatePrompt) return;
		this.quickTaskDuplicateMessage = '';
		this.quickTaskDuplicatePrompt = '';
	};

	createQuickTask = async (allowDuplicate = false): Promise<TaskQuickTaskCreateResult> => {
		const prompt = this.quickTaskText.trim();
		const owner = defaultTaskOwner(this.summary?.members ?? [], this.summary?.currentUserEmail ?? '', '');
		if (!prompt || !owner || !this.summary) return 'ignored';
		this.isCreatingQuickTask = true;
		this.taskErrorMessage = '';
		if (!allowDuplicate) this.clearDuplicatePrompt();
		try {
			const result = await requestQuickTaskCreation(
				{
					prompt,
					ownerID: owner.id,
					participantIDs: [owner.id],
					weekCode: this.taskWeek(),
					allowDuplicate
				},
				this.text.task.quickAddError
			);
			if (result === 'duplicate') {
				this.quickTaskDuplicateMessage = this.text.task.quickAddDuplicate;
				this.quickTaskDuplicatePrompt = prompt;
				return 'duplicate';
			}
			this.quickTaskText = '';
			this.clearDuplicatePrompt();
			await this.loadTask(this.currentWeek());
			return 'created';
		} catch (error) {
			this.taskErrorMessage = error instanceof Error ? error.message : this.text.task.quickAddError;
			return 'failed';
		} finally {
			this.isCreatingQuickTask = false;
		}
	};

	private currentWeek(): string {
		return this.summary?.week.code ?? '';
	}

	private taskWeek(): string {
		return this.summary?.currentWeek?.code ?? this.summary?.week.code ?? '';
	}

	private clearDuplicatePrompt(): void {
		this.quickTaskDuplicateMessage = '';
		this.quickTaskDuplicatePrompt = '';
	}
}
