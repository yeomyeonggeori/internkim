<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import TaskDetailView from './task-detail-view.svelte';
	import TaskEditorFields from './task-editor-fields.svelte';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import TaskEditorParticipants from './task-editor-participants.svelte';
	import TaskEditorSummary from './task-editor-summary.svelte';
	import TaskRelationshipsSection from './task-relationships-section.svelte';
	import { formatTaskWeekCodeRange } from './task-week-label';
	import type { TaskEditorOption, TaskEditorText } from './task-editor-types';
	import type { TaskMember, Task } from './task-types';

	type Props = {
		taskDraft: Task | null;
		isEditingTask: boolean;
		members: TaskMember[];
		categoryOptions: TaskEditorOption[];
		typeOptions: TaskEditorOption[];
		sizeOptions: TaskEditorOption[];
		statusOptions: TaskEditorOption[];
		taskErrorMessage: string;
		isSavingTask: boolean;
		isDeletingTask: boolean;
		pageTitle: string;
		text: TaskEditorText;
		statusLabel: (status: string) => string;
		businessColor: (business: string | null) => string;
		taskTypeColor: (type: string | null) => string;
		tasks: Task[];
		currentMemberID: string;
		pendingRelationshipTaskIDs: string[];
		memberEmail: (memberID: string) => string;
		setParticipantIDs: (memberIDs: string[]) => void;
		removeParticipantID: (memberID: string) => void;
		canRemoveParticipant: (task: Task, memberID: string) => boolean;
		saveTask: () => void;
		deleteTask: (task: Task) => Promise<void>;
		canUpdateTask: (task: Task) => boolean;
		canDeleteTask: (task: Task) => boolean;
		canManageTaskAssignment: (task: Task) => boolean;
		isOwnTask: (task: Task) => boolean;
		startEditingTask: () => void;
		closeEditor: () => void;
		openRelatedTask: (task: Task) => void;
		setTaskParent: (
			taskID: string,
			parentTaskID?: string
		) => void | boolean | Promise<void | boolean>;
		setTaskParents: (taskIDs: string[], parentTaskID: string) => boolean | Promise<boolean>;
		createChildTask: (parentTaskID: string) => void;
		relationshipsReady: boolean;
		relationshipsLoadingLabel: string;
		relationshipsError: string;
		relationshipsRetryLabel: string;
		retryRelationships: () => Promise<boolean>;
	};

	let {
		taskDraft = $bindable<Task | null>(null),
		isEditingTask,
		members,
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
		tasks,
		currentMemberID,
		pendingRelationshipTaskIDs,
		memberEmail,
		setParticipantIDs,
		removeParticipantID,
		canRemoveParticipant,
		saveTask,
		deleteTask,
		canUpdateTask,
		canDeleteTask,
		canManageTaskAssignment,
		isOwnTask,
		startEditingTask,
		closeEditor,
		openRelatedTask,
		setTaskParent,
		setTaskParents,
		createChildTask,
		relationshipsReady,
		relationshipsLoadingLabel,
		relationshipsError,
		relationshipsRetryLabel,
		retryRelationships
	}: Props = $props();

	let canEditTask = $derived(taskDraft ? canUpdateTask(taskDraft) : false);
	let canRemoveTask = $derived(taskDraft ? Boolean(taskDraft.id) && canDeleteTask(taskDraft) : false);
	let canEditTaskAssignment = $derived(taskDraft ? canManageTaskAssignment(taskDraft) : false);

	function sheetTitle(): string {
		if (!taskDraft) return text.createTitle;
		if (taskDraft.id) return isEditingTask ? text.editTitle : text.detailTitle;
		return isOwnTask(taskDraft) ? text.createTitle : text.requestTitle;
	}

	function confirmTaskDelete(task: Task): void {
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
	<Sheet.Content class="w-full gap-0 sm:max-w-xl">
		<Sheet.Header>
			<Sheet.Title>{sheetTitle()}</Sheet.Title>
			<Sheet.Description>{formatTaskWeekCodeRange(taskDraft?.weekCode ?? '')} · {pageTitle}</Sheet.Description>
		</Sheet.Header>
		{#if taskDraft}
			{#if !isEditingTask}
				<div class="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 pb-4">
					<TaskDetailView task={taskDraft} {text} {statusLabel} {businessColor} {taskTypeColor} {memberEmail} />
					{#if taskDraft.id && relationshipsReady}
						<TaskRelationshipsSection
							task={taskDraft}
							{tasks}
							{currentMemberID}
							editable={false}
							canManageRelationships={false}
							pendingTaskIDs={pendingRelationshipTaskIDs}
							text={text.relationships}
							{taskTypeColor}
							{statusLabel}
							onOpenTask={openRelatedTask}
							onSetParent={setTaskParent}
							onSetParents={setTaskParents}
							onCreateChild={createChildTask}
							allowTaskSwitching={true}
						/>
					{:else if taskDraft.id}
						<p role="status" class="text-sm text-muted-foreground">{relationshipsError || relationshipsLoadingLabel}</p>
						{#if relationshipsError}<Button variant="outline" onclick={retryRelationships}>{relationshipsRetryLabel}</Button>{/if}
					{/if}
				</div>
				<Sheet.Footer class="border-t">
					{#if canEditTask}
						<Button class="gap-2" onclick={startEditingTask}>
							<PencilIcon />
							{text.editTitle}
						</Button>
					{:else}
						<p class="text-muted-foreground text-sm">{text.readOnly}</p>
					{/if}
				</Sheet.Footer>
			{:else}
				<div class="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 pb-4">
					<TaskEditorSummary {taskDraft} {text} {statusLabel} {businessColor} {taskTypeColor} />
					<TaskEditorFields
						bind:taskDraft
						{categoryOptions}
						{typeOptions}
						{sizeOptions}
						{statusOptions}
						{canEditTask}
						{text}
						{statusLabel}
						{memberEmail}
					/>
					<TaskEditorParticipants
						{taskDraft}
						{members}
						{canEditTask}
						{canEditTaskAssignment}
						{text}
						{setParticipantIDs}
						{removeParticipantID}
						{canRemoveParticipant}
					/>
					{#if taskDraft.id && relationshipsReady}
						<TaskRelationshipsSection
							task={taskDraft}
							{tasks}
							{currentMemberID}
							editable={canEditTask}
							canManageRelationships={canEditTask}
							pendingTaskIDs={pendingRelationshipTaskIDs}
							text={text.relationships}
							{taskTypeColor}
							{statusLabel}
							onOpenTask={openRelatedTask}
							onSetParent={setTaskParent}
							onSetParents={setTaskParents}
							onCreateChild={createChildTask}
							allowTaskSwitching={false}
						/>
					{:else if taskDraft.id}
						<p role="status" class="text-sm text-muted-foreground">{relationshipsError || relationshipsLoadingLabel}</p>
						{#if relationshipsError}<Button variant="outline" onclick={retryRelationships}>{relationshipsRetryLabel}</Button>{/if}
					{/if}
					<Separator />
					<div class="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">
						{text.dateRule}
					</div>
					{#if taskErrorMessage}
						<p class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{taskErrorMessage}</p>
					{/if}
				</div>
				<Sheet.Footer class="flex-row items-center justify-end gap-2 border-t">
					{#if canRemoveTask}
						<Button
							variant="destructive"
							class="mr-auto"
							onclick={() => {
								if (taskDraft) confirmTaskDelete(taskDraft);
							}}
							disabled={isDeletingTask || isSavingTask}
						>
							{isDeletingTask ? text.deleting : text.deleteAction}
						</Button>
					{/if}
					{#if canEditTask}
						<Button onclick={saveTask} disabled={isSavingTask || !taskDraft.content.trim()}>
							{isSavingTask ? text.saving : text.save}
						</Button>
					{/if}
				</Sheet.Footer>
			{/if}
		{/if}
	</Sheet.Content>
</Sheet.Root>
