<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { personProfileImagePath } from '$lib/person-profile-image';
	import BanIcon from '@lucide/svelte/icons/ban';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import CircleDashedIcon from '@lucide/svelte/icons/circle-dashed';
	import CircleDotIcon from '@lucide/svelte/icons/circle-dot';
	import CirclePauseIcon from '@lucide/svelte/icons/circle-pause';
	import CircleQuestionMarkIcon from '@lucide/svelte/icons/circle-question-mark';
	import CircleXIcon from '@lucide/svelte/icons/circle-x';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import Unlink2Icon from '@lucide/svelte/icons/unlink-2';
	import { flowDefinitionOutlineBadgeStyle } from './flow-definition-colors';
	import {
		isFlowStatusCompleted,
		isFlowStatusInProgress,
		isFlowStatusPaused,
		isFlowStatusPlanned,
		isFlowStatusRejected,
		isFlowStatusRequested,
		isFlowStatusStopped
	} from './flow-status';
	import { statusIconClass } from './flow-style';
	import type { FlowTask } from './flow-types';

	type RelationshipResult = void | boolean;

	type Props = {
		task: FlowTask;
		taskTypeColor: (type: string) => string;
		editable: boolean;
		pending: boolean;
		openTaskLabel: string;
		moreActionsLabel: string;
		removeRelationshipLabel: string;
		onOpenTask: (task: FlowTask) => void;
		onRemoveRelationship: (task: FlowTask) => RelationshipResult | Promise<RelationshipResult>;
		canOpenTask?: boolean;
	};

	let {
		task,
		taskTypeColor,
		editable,
		pending,
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
</script>

<div
	class="group/relationship relative flex min-h-12 items-center gap-3 rounded-lg px-2 py-2 transition-colors data-[openable=true]:hover:bg-muted/50 data-[pending=true]:opacity-55"
	data-openable={canOpenTask}
	data-pending={pending}
	data-flow-relationship-task={task.id}
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

	<span class={`pointer-events-none relative z-[1] inline-flex size-5 shrink-0 items-center justify-center ${statusIconClass(task.status)}`} aria-label={task.status}>
		{#if isFlowStatusCompleted(task.status)}
			<CircleCheckIcon class="size-5" />
		{:else if isFlowStatusInProgress(task.status)}
			<CircleDashedIcon class="size-5" />
		{:else if isFlowStatusPlanned(task.status)}
			<CircleDotIcon class="size-5" />
		{:else if isFlowStatusRequested(task.status)}
			<CircleQuestionMarkIcon class="size-5" />
		{:else if isFlowStatusPaused(task.status)}
			<CirclePauseIcon class="size-5" />
		{:else if isFlowStatusRejected(task.status)}
			<CircleXIcon class="size-5" />
		{:else if isFlowStatusStopped(task.status)}
			<BanIcon class="size-5" />
		{:else}
			<CircleDotIcon class="size-5" />
		{/if}
	</span>

	<div class="pointer-events-none relative z-[1] min-w-0 flex-1 space-y-1.5">
		<div class="flex min-w-0 items-center gap-2">
			<Badge
				variant="outline"
				class="h-5 max-w-24 shrink-0 rounded-md px-1.5 py-0 text-[11px] font-medium shadow-none"
				style={flowDefinitionOutlineBadgeStyle(taskTypeColor(task.type))}
			>
				<span class="truncate">{task.type}</span>
			</Badge>
			<div class="truncate text-sm font-medium text-foreground">{task.content}</div>
		</div>
		<div class="flex min-w-0 items-center gap-1.5">
			<PersonAvatar
				name={task.ownerName}
				seed={task.ownerID || task.ownerName}
				memberID={task.ownerID}
				image={personProfileImagePath(task.ownerID)}
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
