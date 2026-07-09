<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { personProfileImagePath } from '$lib/person-profile-image';
	import { buildFlowTaskBoardCardDisplay } from './flow-task-board-card-model';
	import { sizeBadgeClass } from './flow-style';
	import type { FlowTask } from './flow-types';

	type Props = {
		task: FlowTask;
		businessFallback: string;
		openTask: (task: FlowTask) => void;
		isPending?: boolean;
		isReadOnly?: boolean;
		onTaskDragStart?: (event: DragEvent, task: FlowTask) => void;
		onTaskDragEnd?: (event: DragEvent, task: FlowTask) => void;
		onTaskDragOver?: (event: DragEvent, task: FlowTask) => void;
		onTaskDrop?: (event: DragEvent, task: FlowTask) => void;
	};

	let {
		task,
		businessFallback,
		openTask,
		isPending = false,
		isReadOnly = false,
		onTaskDragStart,
		onTaskDragEnd,
		onTaskDragOver,
		onTaskDrop
	}: Props = $props();

	let canDrag = $derived(!isPending && !isReadOnly);
	let cardClass = $derived([
		'gap-1.5 rounded-md border bg-card p-2.5 shadow-xs transition',
		isPending
			? 'cursor-progress opacity-60 ring-1 ring-primary/20'
			: canDrag
				? 'cursor-grab hover:border-primary/40 hover:shadow-sm active:cursor-grabbing'
				: 'cursor-pointer hover:border-primary/40 hover:shadow-sm',
		isReadOnly ? 'bg-muted/20' : ''
	].join(' '));

	let display = $derived(buildFlowTaskBoardCardDisplay(task, businessFallback));

	function openCurrentTask(): void {
		if (isPending) return;
		openTask(task);
	}
</script>

<Card.Root
		class={cardClass}
		role="button"
		tabindex={isPending ? -1 : 0}
		draggable={canDrag}
		aria-disabled={isPending}
		data-flow-board-card={task.id}
		data-flow-board-pending={isPending ? 'true' : 'false'}
	onclick={openCurrentTask}
	onkeydown={(event) => {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			openCurrentTask();
		}
	}}
	ondragstart={(event) => onTaskDragStart?.(event, task)}
	ondragend={(event) => onTaskDragEnd?.(event, task)}
	ondragover={(event) => onTaskDragOver?.(event, task)}
	ondrop={(event) => onTaskDrop?.(event, task)}
>
	<div class="flex min-w-0 flex-wrap items-center gap-1">
		<Badge variant="outline" class="max-w-24 gap-1 truncate pl-1 pr-1.5 py-0 text-xs font-medium">
			<PersonAvatar name={display.ownerName} seed={task.ownerID || display.ownerName} image={personProfileImagePath(task.ownerID)} class="size-4" />
			{display.ownerName}
		</Badge>
		{#each display.participantNames as name, index}
			<Badge variant="outline" class="max-w-24 gap-1 truncate pl-1 pr-1.5 py-0 text-xs">
				<PersonAvatar name={name} seed={display.participantIDs[index] ?? name} image={personProfileImagePath(display.participantIDs[index])} class="size-3.5" />
				{name}
			</Badge>
		{/each}
	</div>

	<div class="flex items-start justify-between gap-2">
		<div class="min-w-0">
			<div class="line-clamp-2 text-sm font-semibold leading-5 text-card-foreground">{task.content}</div>
		</div>
		<Badge class={`h-6 shrink-0 px-2 text-xs ${sizeBadgeClass(task.size)}`}>{task.size}</Badge>
	</div>

	{#if task.goal}
		<div class="line-clamp-1 text-xs leading-5 text-muted-foreground">{task.goal}</div>
	{/if}

	{#if display.metadataLabels.length > 0}
		<div class="flex flex-wrap items-center gap-1 pt-0.5">
			{#each display.metadataLabels as label}
				<Badge variant="outline" class="max-w-24 truncate px-1.5 py-0 text-xs">{label}</Badge>
			{/each}
		</div>
	{/if}

	{#if display.dateLabel}
		<div class="pt-0.5">
			<Badge variant="secondary" class="max-w-full truncate px-1.5 py-0 text-xs">{display.dateLabel}</Badge>
		</div>
	{/if}
</Card.Root>
