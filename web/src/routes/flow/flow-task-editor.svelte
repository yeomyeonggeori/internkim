<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import FlowTaskDetailView from './flow-task-detail-view.svelte';
	import FlowTaskEditorFields from './flow-task-editor-fields.svelte';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import FlowTaskEditorParticipants from './flow-task-editor-participants.svelte';
	import FlowTaskEditorSummary from './flow-task-editor-summary.svelte';
	import type { FlowTaskEditorOption, FlowTaskEditorText } from './flow-task-editor-types';
	import type { FlowMember, FlowTask } from './flow-types';

	type Props = {
		taskDraft: FlowTask | null;
		isEditingTask: boolean;
		members: FlowMember[];
		memberOptions: FlowTaskEditorOption[];
		categoryOptions: FlowTaskEditorOption[];
		typeOptions: FlowTaskEditorOption[];
		sizeOptions: FlowTaskEditorOption[];
		statusOptions: FlowTaskEditorOption[];
		taskErrorMessage: string;
		isSavingTask: boolean;
		isDeletingTask: boolean;
		pageTitle: string;
		text: FlowTaskEditorText;
		statusLabel: (status: string) => string;
		businessColor: (business: string) => string;
		taskTypeColor: (type: string) => string;
		memberEmail: (memberID: string) => string;
		setTaskOwnerID: (memberID: string) => void;
		setParticipantNames: (names: string[]) => void;
		removeParticipantID: (memberID: string) => void;
		saveTask: () => void;
		deleteTask: (task: FlowTask) => Promise<void>;
		canUpdateTask: (task: FlowTask) => boolean;
		canDeleteTask: (task: FlowTask) => boolean;
		canManageTaskAssignment: (task: FlowTask) => boolean;
		isOwnTask: (task: FlowTask) => boolean;
		startEditingTask: () => void;
		closeEditor: () => void;
	};

	let {
		taskDraft = $bindable<FlowTask | null>(null),
		isEditingTask,
		members,
		memberOptions,
		categoryOptions,
		typeOptions,
		sizeOptions,
		statusOptions,
		taskErrorMessage,
		isSavingTask,
		isDeletingTask,
		pageTitle,
		text,
		statusLabel,
		businessColor,
		taskTypeColor,
		memberEmail,
		setTaskOwnerID,
		setParticipantNames,
		removeParticipantID,
		saveTask,
		deleteTask,
		canUpdateTask,
		canDeleteTask,
		canManageTaskAssignment,
		isOwnTask,
		startEditingTask,
		closeEditor
	}: Props = $props();

	let canEditTask = $derived(taskDraft ? canUpdateTask(taskDraft) : false);
	let canRemoveTask = $derived(taskDraft ? Boolean(taskDraft.id) && canDeleteTask(taskDraft) : false);
	let canEditTaskAssignment = $derived(taskDraft ? canManageTaskAssignment(taskDraft) : false);

	function sheetTitle(): string {
		if (!taskDraft) return text.createTitle;
		if (taskDraft.id) return isEditingTask ? text.editTitle : text.detailTitle;
		return isOwnTask(taskDraft) ? text.createTitle : text.requestTitle;
	}

	function confirmTaskDelete(task: FlowTask): void {
		confirmDelete({
			title: text.deleteTitle,
			description: text.deleteDescription.replace('{task}', task.content),
			confirm: { text: text.deleteConfirm },
			cancel: { text: text.cancel },
			onConfirm: async () => deleteTask(task)
		});
	}
</script>

<Sheet.Root open={taskDraft !== null} onOpenChange={(open) => {
	if (!open) closeEditor();
}}>
	<Sheet.Content class="w-full overflow-y-auto sm:max-w-xl">
		<Sheet.Header>
			<Sheet.Title>{sheetTitle()}</Sheet.Title>
			<Sheet.Description>{taskDraft?.weekCode} · {pageTitle}</Sheet.Description>
		</Sheet.Header>
		{#if taskDraft}
			{#if !isEditingTask}
				<div class="space-y-4 px-4 pb-6">
					<FlowTaskDetailView task={taskDraft} {text} {statusLabel} {businessColor} {taskTypeColor} {memberEmail} />
					<Sheet.Footer>
						{#if canEditTask}
							<Button class="gap-2" onclick={startEditingTask}>
								<PencilIcon />
								{text.editTitle}
							</Button>
						{:else}
							<p class="text-sm text-muted-foreground">{text.readOnly}</p>
						{/if}
					</Sheet.Footer>
				</div>
			{:else}
				<div class="space-y-4 px-4 pb-6">
						<FlowTaskEditorSummary {taskDraft} {text} {statusLabel} {businessColor} {taskTypeColor} />
					<FlowTaskEditorFields
						bind:taskDraft
						{members}
						{memberOptions}
						{categoryOptions}
						{typeOptions}
						{sizeOptions}
						{statusOptions}
						{canEditTask}
						{canEditTaskAssignment}
						{text}
						{statusLabel}
						{setTaskOwnerID}
					/>
					<FlowTaskEditorParticipants
						{taskDraft}
						{members}
						{canEditTask}
						{canEditTaskAssignment}
						{text}
						{setParticipantNames}
						{removeParticipantID}
					/>
					<Separator />
					<div class="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">
						{text.dateRule}
					</div>
					{#if taskErrorMessage}
						<p class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{taskErrorMessage}</p>
					{/if}
					<Sheet.Footer>
						{#if canRemoveTask}
							<Button variant="destructive" onclick={() => {
								if (taskDraft) confirmTaskDelete(taskDraft);
							}} disabled={isDeletingTask || isSavingTask}>
								{isDeletingTask ? text.deleting : text.deleteAction}
							</Button>
						{/if}
						{#if canEditTask}
							<Button onclick={saveTask} disabled={isSavingTask || !taskDraft.content.trim()}>
								{isSavingTask ? text.saving : text.save}
							</Button>
						{/if}
					</Sheet.Footer>
				</div>
			{/if}
		{/if}
	</Sheet.Content>
</Sheet.Root>
