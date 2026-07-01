import type { LoadFlow } from './flow-load-tracker';
import { requestQuickTaskCreation } from './flow-quick-task-create';
import { defaultFlowTaskOwner } from './flow-task-draft';
import { flowText } from './text';
import type { FlowQuickTaskCreateResult, FlowSummary } from './flow-types';

type FlowPageText = typeof flowText.ko;

type FlowTaskQuickCreateControllerInput = {
	summary: FlowSummary | null;
	text: FlowPageText;
	loadFlow: LoadFlow;
};

export class FlowTaskQuickCreateController {
	quickTaskText = $state('');
	taskErrorMessage = $state('');
	quickTaskDuplicateMessage = $state('');
	isCreatingQuickTask = $state(false);

	private summary = $state<FlowSummary | null>(null);
	private text: FlowPageText = flowText.ko;
	private loadFlow: LoadFlow;
	private quickTaskDuplicatePrompt = $state('');

	constructor() {
		this.loadFlow = async () => false;
	}

	sync = (input: FlowTaskQuickCreateControllerInput): void => {
		this.summary = input.summary;
		this.text = input.text;
		this.loadFlow = input.loadFlow;
	};

	clearStaleDuplicatePrompt = (): void => {
		if (!this.quickTaskDuplicateMessage) return;
		if (this.quickTaskText.trim() === this.quickTaskDuplicatePrompt) return;
		this.quickTaskDuplicateMessage = '';
		this.quickTaskDuplicatePrompt = '';
	};

	createQuickTask = async (allowDuplicate = false): Promise<FlowQuickTaskCreateResult> => {
		const prompt = this.quickTaskText.trim();
		const owner = defaultFlowTaskOwner(this.summary?.members ?? [], this.summary?.currentUserEmail ?? '', '');
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
			await this.loadFlow(this.currentWeek());
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
