<script lang="ts">
	import PlusIcon from '@lucide/svelte/icons/plus';
	import {
		buildFlowTaskChildProgress,
		buildFlowTaskRelationships
	} from './flow-task-relationships';
	import FlowTaskRelationshipRow from './flow-task-relationship-row.svelte';
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

<section class="space-y-2 py-5" data-flow-task-relationships>
	<h3 class="text-sm font-semibold text-foreground">{text.title}</h3>

	<div class="space-y-1">
		{#if canManageRelationships && !relationships.parent}
			<button
				type="button"
				class="flex min-h-9 w-full items-center justify-between gap-3 rounded-lg px-2 text-left transition-colors hover:bg-muted/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
				disabled={isPending(task.id)}
				aria-label={`${text.parent} ${text.add}`}
				onclick={() => (parentSelectorOpen = true)}
			>
				<h4 class="text-xs font-semibold tracking-wide text-muted-foreground">{text.parent}</h4>
				<span class="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground">
					<PlusIcon class="size-3.5" />
					{text.add}
				</span>
			</button>
		{:else}
			<div class="flex min-h-9 items-center px-2">
				<h4 class="text-xs font-semibold tracking-wide text-muted-foreground">{text.parent}</h4>
			</div>
		{/if}

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

	<div class="space-y-1 border-t border-border/60 pt-4">
		{#if canManageRelationships}
			<button
				type="button"
				class="flex min-h-9 w-full items-center justify-between gap-3 rounded-lg px-2 text-left transition-colors hover:bg-muted/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
				disabled={isPending(task.id)}
				aria-label={`${text.children} ${text.add}`}
				onclick={() => (childSelectorOpen = true)}
			>
				<div class="flex items-center gap-2">
					<h4 class="text-xs font-semibold tracking-wide text-muted-foreground">{text.children}</h4>
					{#if childProgress}
						<span
							class="relationship-progress-ring inline-flex size-3.5 shrink-0 items-center justify-center rounded-full"
							style={`--relationship-progress-angle: ${childProgress.percent * 3.6}deg`}
							aria-hidden="true"
						>
							<span class="size-2 rounded-full bg-background"></span>
						</span>
						<span class="text-xs font-medium tabular-nums text-violet-700 dark:text-violet-300">
							{childProgress.completed} / {childProgress.total}
						</span>
					{/if}
				</div>
				<span class="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground">
					<PlusIcon class="size-3.5" />
					{text.add}
				</span>
			</button>
		{:else}
			<div class="flex min-h-9 items-center gap-2 px-2">
				<h4 class="text-xs font-semibold tracking-wide text-muted-foreground">{text.children}</h4>
				{#if childProgress}
					<span
						class="relationship-progress-ring inline-flex size-3.5 shrink-0 items-center justify-center rounded-full"
						style={`--relationship-progress-angle: ${childProgress.percent * 3.6}deg`}
						aria-hidden="true"
					>
						<span class="size-2 rounded-full bg-background"></span>
					</span>
					<span class="text-xs font-medium tabular-nums text-violet-700 dark:text-violet-300">
						{childProgress.completed} / {childProgress.total}
					</span>
				{/if}
			</div>
		{/if}

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

<style>
	.relationship-progress-ring {
		background: conic-gradient(
			#8b5cf6 var(--relationship-progress-angle),
			var(--muted) var(--relationship-progress-angle)
		);
	}
</style>
