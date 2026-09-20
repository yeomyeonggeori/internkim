<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Command from '$lib/components/ui/command';
	import Link2Icon from '@lucide/svelte/icons/link-2';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import DefinitionBadge from '$lib/components/definition-badge.svelte';
	import {
		taskChildCandidates,
		taskParentCandidates
	} from './task-relationships';
	import type { TaskRelationshipsText } from './task-editor-types';
	import { taskDefinitionLabel } from './task-workspace-model';
	import type { Task } from './task-types';

	type RelationshipMode = 'parent' | 'children';
	type RelationshipResult = void | boolean;

	type Props = {
		open?: boolean;
		mode: RelationshipMode;
		task: Task;
		tasks: Task[];
		currentMemberID: string;
		pendingTaskIDs: string[];
		text: TaskRelationshipsText;
		taskTypeColor: (type: string | null) => string;
		onSetParent: (taskID: string, parentID?: string) => RelationshipResult | Promise<RelationshipResult>;
		onSetParents: (taskIDs: string[], parentID: string) => boolean | Promise<boolean>;
		onCreateChild: () => boolean;
	};

	let {
		open = $bindable(false),
		mode,
		task,
		tasks,
		currentMemberID,
		pendingTaskIDs,
		text,
		taskTypeColor,
		onSetParent,
		onSetParents,
		onCreateChild
	}: Props = $props();

	let selectedTaskIDs = $state<string[]>([]);
	let submitting = $state(false);
	let candidates = $derived(
		mode === 'parent'
			? taskParentCandidates(task, tasks, currentMemberID)
			: taskChildCandidates(task, tasks, currentMemberID)
	);
	let title = $derived(mode === 'parent' ? text.parent : text.children);
	let searchPlaceholder = $derived(mode === 'parent' ? text.searchParent : text.searchChildren);

	function handleOpenChange(nextOpen: boolean): void {
		if (!nextOpen) selectedTaskIDs = [];
	}

	function toggleChild(taskID: string): void {
		selectedTaskIDs = selectedTaskIDs.includes(taskID)
			? selectedTaskIDs.filter((selectedTaskID) => selectedTaskID !== taskID)
			: [...selectedTaskIDs, taskID];
	}

	async function selectParent(parentID: string): Promise<void> {
		if (submitting || pendingTaskIDs.includes(task.id)) return;
		submitting = true;
		try {
			const result = await onSetParent(task.id, parentID);
			if (result !== false) open = false;
		} finally {
			submitting = false;
		}
	}

	async function connectSelectedChildren(): Promise<void> {
		if (submitting || selectedTaskIDs.length === 0) return;
		submitting = true;
		try {
			if (await onSetParents(selectedTaskIDs, task.id)) open = false;
		} finally {
			submitting = false;
		}
	}

	function createChild(): void {
		if (onCreateChild()) open = false;
	}
</script>

<Command.Dialog
	bind:open
	{title}
	description={searchPlaceholder}
	showCloseButton
	class="sm:max-w-lg"
	onOpenChange={handleOpenChange}
>
	<Command.Input placeholder={searchPlaceholder} />
	<Command.List>
		<Command.Empty>{text.noCandidates}</Command.Empty>
		<Command.Group heading={title}>
			{#each candidates as candidate (candidate.id)}
				<Command.Item
					value={`relationship-${mode}-${candidate.id}`}
					keywords={[candidate.content, candidate.ownerName, candidate.type ?? '', candidate.business ?? '']}
					data-checked={mode === 'parent' ? task.parentTaskID === candidate.id : selectedTaskIDs.includes(candidate.id)}
					disabled={submitting || pendingTaskIDs.includes(candidate.id)}
					onSelect={() => mode === 'parent' ? void selectParent(candidate.id) : toggleChild(candidate.id)}
				>
					<div class="min-w-0 flex-1">
						<div class="truncate font-medium">{candidate.content}</div>
						<div class="mt-1 flex min-w-0 items-center gap-2">
							<DefinitionBadge
								label={taskDefinitionLabel(candidate.type, text.etcLabel)}
								color={taskTypeColor(candidate.type)}
								class="max-w-28"
							/>
							<span class="truncate text-xs text-muted-foreground">{candidate.ownerName}</span>
						</div>
					</div>
				</Command.Item>
			{/each}
		</Command.Group>

		{#if mode === 'children'}
			<Command.Separator />
			<Command.Group>
				<Command.Item value="relationship-create-child" forceMount onSelect={createChild} disabled={submitting}>
					<PlusIcon />
					{text.createChild}
				</Command.Item>
			</Command.Group>
		{/if}
	</Command.List>

	{#if mode === 'children'}
		<div class="flex items-center justify-end border-t border-border/60 p-2">
			<Button
				size="sm"
				disabled={submitting || selectedTaskIDs.length === 0}
				onclick={() => void connectSelectedChildren()}
			>
				<Link2Icon />
				{text.connectSelected}
			</Button>
		</div>
	{/if}
</Command.Dialog>
