<script lang="ts">
	import { Separator } from '$lib/components/ui/separator';
	import {
		buildFlowTaskChildProgress,
		buildFlowTaskRelationships
	} from './flow-task-relationships';
	import FlowTaskRelationshipRow from './flow-task-relationship-row.svelte';
	import FlowTaskRelationshipSectionHeader from './flow-task-relationship-section-header.svelte';
	import FlowTaskRelationshipSelector from './flow-task-relationship-selector.svelte';
	import type { FlowTaskRelationshipsText } from './flow-task-editor-types';
	import type { FlowTask } from './flow-types';

	type RelationshipResult = void | boolean;

	type Props = {
		task: FlowTask;
		tasks: FlowTask[];
		currentMemberID: string;
		editable: boolean;
		canManageRelationships?: boolean;
		pendingTaskIDs?: string[];
		text: FlowTaskRelationshipsText;
		taskTypeColor: (type: string) => string;
		onOpenTask: (task: FlowTask) => void;
		onSetParent: (taskID: string, parentID?: string) => RelationshipResult | Promise<RelationshipResult>;
		onSetParents: (taskIDs: string[], parentID: string) => boolean | Promise<boolean>;
		onCreateChild: (parentID: string) => void;
		allowTaskSwitching?: boolean;
	};

	let {
		task,
		tasks,
		currentMemberID,
		editable,
		canManageRelationships = editable,
		pendingTaskIDs = [],
		text,
		taskTypeColor,
		onOpenTask,
		onSetParent,
		onSetParents,
		onCreateChild,
		allowTaskSwitching = true
	}: Props = $props();

	function openTask(relatedTask: FlowTask): boolean {
		if (!allowTaskSwitching && !window.confirm(text.discardChanges)) return false;
		onOpenTask(relatedTask);
		return true;
	}

	function createChild(): boolean {
		if (!allowTaskSwitching && !window.confirm(text.discardChanges)) return false;
		onCreateChild(task.id);
		return true;
	}

	let parentSelectorOpen = $state(false);
	let childSelectorOpen = $state(false);
	let relationships = $derived(buildFlowTaskRelationships(task, tasks));
	let childProgress = $derived(buildFlowTaskChildProgress(task.id, tasks));

	function isPending(taskID: string): boolean {
		return pendingTaskIDs.includes(taskID);
	}
</script>

<section class="flex flex-col gap-2 py-5" data-flow-task-relationships>
	<h3 class="text-sm font-semibold text-foreground">{text.title}</h3>

	<div class="flex flex-col gap-1">
		<FlowTaskRelationshipSectionHeader
			title={text.parent}
			addLabel={text.add}
			actionable={canManageRelationships && !relationships.parent}
			disabled={isPending(task.id)}
			onAdd={() => (parentSelectorOpen = true)}
		/>

		{#if relationships.parent}
			<FlowTaskRelationshipRow
				task={relationships.parent}
				{taskTypeColor}
				{editable}
				pending={isPending(task.id)}
				openTaskLabel={text.parent}
				moreActionsLabel={text.moreActions}
				removeRelationshipLabel={text.removeRelationship}
				onOpenTask={openTask}
				canOpenTask={allowTaskSwitching}
				onRemoveRelationship={() => onSetParent(task.id, undefined)}
			/>
		{/if}
	</div>

	<Separator class="bg-border/60" />

	<div class="flex flex-col gap-1 pt-2">
		<FlowTaskRelationshipSectionHeader
			title={text.children}
			addLabel={text.add}
			progressLabel={text.progressLabel
				.replace('{completed}', String(childProgress?.completed ?? 0))
				.replace('{total}', String(childProgress?.total ?? 0))}
			progress={childProgress}
			actionable={canManageRelationships}
			disabled={isPending(task.id)}
			onAdd={() => (childSelectorOpen = true)}
		/>

		{#if relationships.children.length > 0}
			<div class="divide-y divide-border/60" data-flow-relationship-list>
				{#each relationships.children as child (child.id)}
					<FlowTaskRelationshipRow
						task={child}
						{taskTypeColor}
						{editable}
						pending={isPending(child.id)}
						openTaskLabel={text.children}
						moreActionsLabel={text.moreActions}
						removeRelationshipLabel={text.removeRelationship}
						onOpenTask={openTask}
						canOpenTask={allowTaskSwitching}
						onRemoveRelationship={(childTask) => onSetParent(childTask.id, undefined)}
					/>
				{/each}
			</div>
		{/if}
	</div>
</section>

{#if canManageRelationships}
	<FlowTaskRelationshipSelector
		bind:open={parentSelectorOpen}
		mode="parent"
		{task}
		{tasks}
		{currentMemberID}
		{pendingTaskIDs}
		{text}
		{taskTypeColor}
		{onSetParent}
		{onSetParents}
		onCreateChild={createChild}
	/>
	<FlowTaskRelationshipSelector
		bind:open={childSelectorOpen}
		mode="children"
		{task}
		{tasks}
		{currentMemberID}
		{pendingTaskIDs}
		{text}
		{taskTypeColor}
		{onSetParent}
		{onSetParents}
		onCreateChild={createChild}
	/>
{/if}
