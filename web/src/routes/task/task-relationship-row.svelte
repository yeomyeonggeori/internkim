<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import CircleDotIcon from '@lucide/svelte/icons/circle-dot';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import Unlink2Icon from '@lucide/svelte/icons/unlink-2';
	import DefinitionBadge from '$lib/components/definition-badge.svelte';
	import {
		isTaskStatusCompleted
	} from './task-status';
	import { relationshipStatusIconClass } from './task-style';
	import { taskDefinitionLabel } from './task-workspace-model';
	import type { Task } from './task-types';

	type RelationshipResult = void | boolean;

	type Props = {
		task: Task;
		taskTypeColor: (type: string | null) => string;
		etcLabel: string;
		editable: boolean;
		pending: boolean;
		statusLabel: (status: string) => string;
		openTaskLabel: string;
		moreActionsLabel: string;
		removeRelationshipLabel: string;
		onOpenTask: (task: Task) => void;
		onRemoveRelationship: (task: Task) => RelationshipResult | Promise<RelationshipResult>;
		canOpenTask?: boolean;
	};

	let {
		task,
		taskTypeColor,
		etcLabel,
		editable,
		pending,
		statusLabel,
		openTaskLabel,
		moreActionsLabel,
		removeRelationshipLabel,
		onOpenTask,
		onRemoveRelationship,
		canOpenTask = true
	}: Props = $props();

	function stopPropagation(event: Event): void {
		event.stopPropagation();
	}

	let statusKind = $derived(
		isTaskStatusCompleted(task.status) ? 'completed' : 'incomplete'
	);
</script>

<div
	class="group/relationship relative flex min-h-12 items-center gap-3 rounded-lg px-2 py-2 transition-colors data-[openable=true]:hover:bg-muted/50 data-[pending=true]:opacity-55"
	data-openable={canOpenTask}
	data-pending={pending}
	data-task-relationship-task={task.id}
>
	{#if canOpenTask}
		<button
			type="button"
			class="absolute inset-0 z-0 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
			aria-label={`${openTaskLabel}: ${task.content}`}
			disabled={pending}
			onclick={() => onOpenTask(task)}
		>
			<span class="sr-only">{openTaskLabel}: {task.content}</span>
		</button>
	{/if}

	<span
		class={`pointer-events-none relative z-[1] inline-flex size-5 shrink-0 items-center justify-center ${relationshipStatusIconClass(task.status)}`}
		aria-label={statusLabel(task.status)}
		data-relationship-status-kind={statusKind}
	>
		{#if isTaskStatusCompleted(task.status)}
			<CircleCheckIcon class="size-5" />
		{:else}
			<CircleDotIcon class="size-5" />
		{/if}
	</span>

	<div class="pointer-events-none relative z-[1] min-w-0 flex-1 space-y-1.5">
		<div class="flex min-w-0 items-center gap-2">
			<DefinitionBadge label={taskDefinitionLabel(task.type, etcLabel)} color={taskTypeColor(task.type)} class="max-w-24 shrink-0" />
			<div class="truncate text-sm font-medium text-foreground">{task.content}</div>
		</div>
		<div class="flex min-w-0 items-center gap-1.5">
			<PersonAvatar
				name={task.ownerName}
				seed={task.ownerID || task.ownerName}
				memberID={task.ownerID}
				
				class="size-4 ring-1 ring-border/60"
			/>
			<span class="truncate text-xs text-muted-foreground">{task.ownerName}</span>
		</div>
	</div>

	{#if editable}
		<div class="relative z-10 shrink-0">
			<DropdownMenu.Root>
				<DropdownMenu.Trigger onclick={stopPropagation} onkeydown={stopPropagation}>
					{#snippet child({ props })}
						<Button
							{...props}
							variant="ghost"
							size="icon-xs"
							aria-label={moreActionsLabel}
							title={moreActionsLabel}
							disabled={pending}
						>
							<EllipsisIcon />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end" sideOffset={6} class="z-[52]">
					<DropdownMenu.Item
						variant="destructive"
						onclick={(event) => {
							event.stopPropagation();
							void onRemoveRelationship(task);
						}}
					>
						<Unlink2Icon />
						{removeRelationshipLabel}
					</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		</div>
	{/if}
</div>
